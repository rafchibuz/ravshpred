package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ravshann/predlozhka/backend/internal/domain"
)

// ReadRatingSnapshot never calculates scores or reads the message history.
// Both top-100 and the viewer's own place come from one atomic publication.
func (s *Store) ReadRatingSnapshot(ctx context.Context, channel, period, userID string) (domain.ViewerRating, error) {
	result := domain.ViewerRating{Channel: channel, Period: period, Items: []domain.ViewerRatingEntry{}, Collectors: []domain.ViewerRatingStatus{}}
	var payload, personal, collectors []byte
	err := s.pool.QueryRow(ctx, `SELECT s.payload,e.payload,
		(SELECT COALESCE(jsonb_agg(jsonb_build_object('channel',channel_login,'status',collector_status,
		'collector_user',collector_user,'last_event_at',last_event_at,'last_error',last_error,'updated_at',updated_at)), '[]'::jsonb)
		FROM twitch_rating_state WHERE $3='all' OR channel_login=$3)
		FROM twitch_rating_snapshots s
		LEFT JOIN twitch_rating_snapshot_entries e ON e.snapshot_key=s.key AND e.user_id=$2
		WHERE s.key=$1`, channel+":"+period, userID, channel).Scan(&payload, &personal, &collectors)
	if errors.Is(err, pgx.ErrNoRows) {
		result.Preparing = true
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(payload, &result); err != nil {
		return result, err
	}
	if err = json.Unmarshal(collectors, &result.Collectors); err != nil {
		return result, err
	}
	if len(personal) > 0 {
		var me domain.ViewerRatingEntry
		if err = json.Unmarshal(personal, &me); err != nil {
			return result, err
		}
		result.Me = &me
	}
	// Snapshot payloads remain compact and immutable. Merge profile images from
	// the small, separately hydrated top-100 profile cache at read time.
	ids := make([]string, 0, len(result.Items)+1)
	for _, item := range result.Items {
		ids = append(ids, item.TwitchID)
	}
	if result.Me != nil {
		ids = append(ids, result.Me.TwitchID)
	}
	avatars, err := s.ratingAvatarURLs(ctx, ids)
	if err != nil {
		return result, err
	}
	for index := range result.Items {
		if result.Items[index].AvatarURL == "" {
			result.Items[index].AvatarURL = avatars[result.Items[index].TwitchID]
		}
	}
	if result.Me != nil && result.Me.AvatarURL == "" {
		result.Me.AvatarURL = avatars[result.Me.TwitchID]
	}
	result.Stale = time.Since(result.GeneratedAt) > 3*time.Minute
	return result, nil
}

// RatingAvatarTargets returns only users that are currently missing a cached
// profile image (or whose image is older than a week). The list is derived
// from published snapshots, so unregistered viewers are included as well.
func (s *Store) RatingAvatarTargets(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT item->>'twitch_id'
		FROM twitch_rating_snapshots s
		CROSS JOIN LATERAL jsonb_array_elements(s.payload->'items') AS item
		LEFT JOIN twitch_rating_profiles p ON p.user_id=item->>'twitch_id'
		WHERE (p.user_id IS NULL OR p.avatar_url='' OR p.updated_at < now()-interval '7 days')
		ORDER BY 1
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != "" {
			result = append(result, id)
		}
	}
	return result, rows.Err()
}

// SaveRatingAvatars publishes the latest Twitch profile images. It is safe to
// call repeatedly and stores no data for users outside the selected top-100.
func (s *Store) SaveRatingAvatars(ctx context.Context, avatars map[string]string) error {
	if len(avatars) == 0 {
		return nil
	}
	ids := make([]string, 0, len(avatars))
	for id := range avatars {
		if id != "" && avatars[id] != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return nil
	}
	urls := make([]string, len(ids))
	for index, id := range ids {
		urls[index] = avatars[id]
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO twitch_rating_profiles(user_id, avatar_url, updated_at)
		SELECT user_id, avatar_url, now() FROM unnest($1::text[], $2::text[]) AS v(user_id, avatar_url)
		ON CONFLICT(user_id) DO UPDATE SET avatar_url=excluded.avatar_url, updated_at=now()`, ids, urls)
	return err
}

func (s *Store) ratingAvatarURLs(ctx context.Context, userIDs []string) (map[string]string, error) {
	result := make(map[string]string, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT user_id, avatar_url FROM twitch_rating_profiles WHERE user_id=ANY($1)`, userIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, avatar string
		if err := rows.Scan(&id, &avatar); err != nil {
			return nil, err
		}
		if avatar != "" {
			result[id] = avatar
		}
	}
	return result, rows.Err()
}

func (s *Store) publishRatingSnapshot(ctx context.Context, key string, result domain.ViewerRating) error {
	entries := result.Items
	if len(result.Items) > 100 {
		result.Items = result.Items[:100]
	}
	result.Me = nil
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `INSERT INTO twitch_rating_snapshots(key,generated_at,payload) VALUES($1,$2,$3::jsonb)
		ON CONFLICT(key) DO UPDATE SET generated_at=excluded.generated_at,payload=excluded.payload`, key, result.GeneratedAt, mustJSON(result)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM twitch_rating_snapshot_entries WHERE snapshot_key=$1", key); err != nil {
		return err
	}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"twitch_rating_snapshot_entries"}, []string{"snapshot_key", "user_id", "payload"}, pgx.CopyFromSlice(len(entries), func(i int) ([]any, error) {
		return []any{key, entries[i].TwitchID, json.RawMessage(mustJSON(entries[i]))}, nil
	}))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// One dedicated connection bounds background work and cannot exhaust the HTTP pool.
// The advisory transaction lock prevents duplicate workers across API instances.
func (s *Store) RunRatingSnapshots(ctx context.Context, logger *slog.Logger) {
	cfg := s.pool.Config()
	cfg.MaxConns = 1
	cfg.MinConns = 0
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "60000"
	cfg.ConnConfig.RuntimeParams["work_mem"] = "16MB"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		logger.Error("rating snapshot worker failed", "error", err)
		return
	}
	defer pool.Close()
	worker := &Store{pool: pool}
	for ctx.Err() == nil {
		for _, period := range []string{"30d", "last_stream", "1d", "7d", "1y", "90d", "all"} {
			for _, channel := range []string{"all", "ravshann", "ravshanbtw"} {
				if ctx.Err() != nil {
					return
				}
				started := time.Now()
				jobCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
				err := worker.refreshRatingSnapshot(jobCtx, channel, period)
				cancel()
				if err != nil {
					logger.Warn("rating snapshot refresh failed", "channel", channel, "period", period, "duration_ms", time.Since(started).Milliseconds(), "error", err)
				} else {
					logger.Info("rating snapshot refresh completed", "channel", channel, "period", period, "duration_ms", time.Since(started).Milliseconds())
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Minute):
		}
	}
}

func snapshotSince(period string, now time.Time) *time.Time {
	if period == "1y" {
		value := now.AddDate(-1, 0, 0)
		return &value
	}
	days := map[string]int{"1d": 1, "7d": 7, "30d": 30, "90d": 90, "1y": 365}[period]
	if days == 0 {
		return nil
	}
	value := now.Add(-time.Duration(days) * 24 * time.Hour)
	return &value
}

func (s *Store) refreshRatingSnapshot(ctx context.Context, channel, period string) error {
	// Use a session lock on the worker's sole connection; always release it before reuse.
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	var locked bool
	err = conn.QueryRow(ctx, "SELECT pg_try_advisory_lock(913572)").Scan(&locked)
	if err != nil || !locked {
		conn.Release()
		return err
	}
	conn.Release()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.pool.Exec(cleanup, "SELECT pg_advisory_unlock(913572)")
	}()
	key := channel + ":" + period
	var fresh bool
	if err = s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM twitch_rating_snapshots WHERE key=$1 AND generated_at > now()-interval '60 seconds')", key).Scan(&fresh); err != nil || fresh {
		return err
	}
	channels := []string{"ravshann", "ravshanbtw"}
	if channel != "all" {
		channels = []string{channel}
	}
	result, err := s.ViewerRating(ctx, channels, snapshotSince(period, time.Now().UTC()), period == "last_stream", "", 0)
	if err != nil {
		return fmt.Errorf("calculate snapshot: %w", err)
	}
	result.Channel, result.Period = channel, period
	return s.publishRatingSnapshot(ctx, key, result)
}
