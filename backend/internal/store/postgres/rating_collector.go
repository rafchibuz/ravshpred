package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/ravshann/predlozhka/backend/internal/store"
	"github.com/ravshann/predlozhka/backend/internal/twitch"
)

func (s *Store) RatingCredentials(ctx context.Context) (twitch.RatingCredentials, error) {
	var result twitch.RatingCredentials
	err := s.pool.QueryRow(ctx, `SELECT access_token, refresh_token, collector_user_id, collector_login, expires_at FROM twitch_rating_credentials WHERE id`).Scan(
		&result.AccessToken, &result.RefreshToken, &result.UserID, &result.Login, &result.ExpiresAt,
	)
	if err == pgx.ErrNoRows {
		return twitch.RatingCredentials{}, store.ErrNotFound
	}
	return result, err
}

func (s *Store) SaveRatingCredentials(ctx context.Context, value twitch.RatingCredentials) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO twitch_rating_credentials(id, access_token, refresh_token, collector_user_id, collector_login, expires_at, updated_at)
		VALUES(true, $1, $2, $3, $4, $5, now())
		ON CONFLICT(id) DO UPDATE SET access_token=excluded.access_token, refresh_token=excluded.refresh_token,
		collector_user_id=excluded.collector_user_id, collector_login=excluded.collector_login,
		expires_at=excluded.expires_at, updated_at=now()`, value.AccessToken, value.RefreshToken, value.UserID, value.Login, value.ExpiresAt)
	return err
}

func (s *Store) UpsertRatingStream(ctx context.Context, value twitch.RatingStream) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO twitch_rating_streams(id, channel_login, started_at, ended_at, title, game_name, observed_live)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(channel_login,id) DO UPDATE SET started_at=excluded.started_at,
		ended_at=COALESCE(excluded.ended_at,twitch_rating_streams.ended_at), title=excluded.title,
		game_name=excluded.game_name, observed_live=twitch_rating_streams.observed_live OR excluded.observed_live`,
		value.ID, value.Channel, value.StartedAt, value.EndedAt, value.Title, value.GameName, value.ObservedLive)
	return err
}

func (s *Store) CloseRatingStreams(ctx context.Context, channel, streamID string, endedAt time.Time) error {
	query := `UPDATE twitch_rating_streams SET ended_at=$1 WHERE channel_login=$2 AND ended_at IS NULL`
	args := []any{endedAt, channel}
	if streamID != "" {
		query += ` AND id=$3`
		args = append(args, streamID)
	}
	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

func (s *Store) InsertRatingMessage(ctx context.Context, value twitch.RatingMessage) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO twitch_rating_messages(id, channel_login, user_id, login, display_name, avatar_url, sent_at,
		stream_id, role, text_hash, text_length, is_command, is_duplicate, is_emote_only, is_reply)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT(id) DO NOTHING`, value.ID, value.Channel, value.UserID, value.Login, value.DisplayName,
		value.AvatarURL, value.SentAt, value.StreamID, value.Role, value.TextHash, value.TextLength,
		value.IsCommand, value.IsDuplicate, value.IsEmoteOnly, value.IsReply)
	return err
}

func (s *Store) SetRatingCollectorStatus(ctx context.Context, channels []string, status, user, lastError string, lastEvent *time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE twitch_rating_state SET collector_status=$1, collector_user=$2, last_error=$3,
		last_event_at=COALESCE($4,last_event_at), updated_at=now() WHERE channel_login=ANY($5)`,
		status, user, lastError, lastEvent, channels)
	return err
}
