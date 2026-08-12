package postgres

import (
	"context"
	"time"

	"github.com/ravshann/predlozhka/backend/internal/domain"
)

func (s *Store) ViewerRating(ctx context.Context, channels []string, since *time.Time, lastStream bool, meTwitchID string, limit int) (domain.ViewerRating, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	result := domain.ViewerRating{GeneratedAt: time.Now().UTC(), Items: []domain.ViewerRatingEntry{}, Collectors: []domain.ViewerRatingStatus{}}
	if len(channels) == 1 {
		result.Channel = channels[0]
	} else {
		result.Channel = "all"
	}

	const statsSQL = `
		WITH latest_completed AS (
			SELECT channel_login, id FROM twitch_rating_streams
			WHERE channel_login = ANY($1) AND observed_live AND ended_at IS NOT NULL
			ORDER BY ended_at DESC, started_at DESC LIMIT 1
		)
		SELECT min(started_at), count(*)
		FROM twitch_rating_streams s
		WHERE channel_login = ANY($1) AND observed_live
		  AND started_at >= COALESCE($2::timestamptz, '-infinity'::timestamptz)
		  AND (NOT $3 OR (s.channel_login, s.id) IN (SELECT channel_login, id FROM latest_completed))
	`
	if err := s.pool.QueryRow(ctx, statsSQL, channels, since, lastStream).Scan(&result.CollectionStartedAt, &result.StreamCount); err != nil {
		return domain.ViewerRating{}, err
	}

	const ratingSQL = `
		WITH latest_completed AS (
			SELECT channel_login, id FROM twitch_rating_streams
			WHERE channel_login = ANY($1) AND observed_live AND ended_at IS NOT NULL
			ORDER BY ended_at DESC, started_at DESC LIMIT 1
		), selected_streams AS (
			SELECT s.channel_login, s.id, s.started_at
			FROM twitch_rating_streams s
			WHERE s.channel_login = ANY($1)
			  AND observed_live
			  AND s.started_at >= COALESCE($2::timestamptz, '-infinity'::timestamptz)
			  AND (NOT $3 OR (s.channel_login, s.id) IN (SELECT channel_login, id FROM latest_completed))
		), qualifying AS (
			SELECT m.*, m.channel_login || ':' || m.stream_id AS stream_key,
			       floor(extract(epoch FROM m.sent_at) / 600)::bigint AS bucket
			FROM twitch_rating_messages m
			JOIN selected_streams s ON s.channel_login = m.channel_login AND s.id = m.stream_id
			WHERE m.channel_login = ANY($1)
			  AND m.sent_at >= COALESCE($2::timestamptz, '-infinity'::timestamptz)
			  AND NOT m.is_command AND NOT m.is_duplicate
			  AND (m.text_length >= 2 OR m.is_emote_only)
		), latest_users AS (
			SELECT DISTINCT ON (user_id) user_id, login, display_name, avatar_url, role
			FROM qualifying
			ORDER BY user_id, sent_at DESC
		), per_stream AS (
			SELECT user_id, stream_key, count(*) AS messages, count(DISTINCT bucket) AS buckets,
			       count(*) FILTER (WHERE is_reply) AS replies, max(sent_at) AS last_activity
			FROM qualifying GROUP BY user_id, stream_key
		), active AS (
			SELECT * FROM per_stream WHERE messages >= 3 AND buckets >= 2
		), user_active AS (
			SELECT user_id, count(*) AS active_streams, sum(buckets) AS active_buckets, sum(replies) AS replies
			FROM active GROUP BY user_id
		), active_calendar AS (
			SELECT q.user_id, count(DISTINCT q.sent_at::date) AS active_days,
			       count(DISTINCT date_trunc('week', q.sent_at)) AS active_weeks
			FROM qualifying q JOIN active a ON a.user_id=q.user_id AND a.stream_key=q.stream_key
			GROUP BY q.user_id
		), per_user AS (
			SELECT q.user_id,
			       count(*) AS messages,
			       COALESCE(max(cal.active_days), 0) AS active_days,
			       COALESCE(max(cal.active_weeks), 0) AS active_weeks,
			       max(q.sent_at) AS last_activity,
			       COALESCE(max(a.active_streams), 0) AS active_streams,
			       COALESCE(max(a.active_buckets), 0) AS active_buckets,
			       COALESCE(max(a.replies), 0) AS replies
			FROM qualifying q
			LEFT JOIN user_active a ON a.user_id = q.user_id
			LEFT JOIN active_calendar cal ON cal.user_id = q.user_id
			GROUP BY q.user_id
		), constants AS (
			SELECT count(*)::numeric AS streams,
			       greatest(count(DISTINCT date_trunc('week', started_at)), 1)::numeric AS stream_weeks
			FROM selected_streams
		), scored AS (
			SELECT u.user_id, lu.login, COALESCE(NULLIF(lu.display_name, ''), lu.login) AS display_name,
			       lu.avatar_url, lu.role, u.messages, u.active_streams, u.active_days, u.active_weeks,
			       u.last_activity,
			       CASE WHEN c.streams = 0 THEN 0 ELSE round(u.active_streams::numeric / c.streams * 100) END::int AS coverage,
			       round(least(100, greatest(0,
				 (CASE WHEN c.streams = 0 THEN 0 ELSE u.active_streams::numeric / c.streams * 100 END) * .40 +
				 (u.active_weeks::numeric / c.stream_weeks * 100) * .25 +
				 (exp(-extract(epoch FROM (now() - u.last_activity)) / 86400 / 30) * 100) * .20 +
				 (CASE WHEN u.active_streams = 0 THEN 0 ELSE least(1, u.active_buckets::numeric / u.active_streams / 4) * 100 END) * .10 +
				 (CASE WHEN u.messages = 0 THEN 0 ELSE least(1, u.replies::numeric / u.messages / .2) * 100 END) * .05
			 )))::int AS score,
			 c.streams::int AS stream_count
			FROM per_user u JOIN latest_users lu ON lu.user_id = u.user_id CROSS JOIN constants c
		), ranked AS (
			SELECT *, row_number() OVER (ORDER BY score DESC, active_streams DESC, messages DESC, login)::int AS rank
			FROM scored
		)
		SELECT rank, user_id, login, display_name, avatar_url, role, score, messages, active_streams,
		       coverage, active_days, active_weeks, last_activity, stream_count
		FROM ranked ORDER BY rank
	`
	rows, err := s.pool.Query(ctx, ratingSQL, channels, since, lastStream)
	if err != nil {
		return domain.ViewerRating{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item domain.ViewerRatingEntry
		var streamCount int
		if err := rows.Scan(&item.Rank, &item.TwitchID, &item.Login, &item.DisplayName, &item.AvatarURL, &item.Role,
			&item.Score, &item.Messages, &item.ActiveStreams, &item.StreamCoverage, &item.ActiveDays,
			&item.ActiveWeeks, &item.LastActivity, &streamCount); err != nil {
			return domain.ViewerRating{}, err
		}
		if streamCount < 8 || result.CollectionStartedAt == nil || time.Since(*result.CollectionStartedAt) < 30*24*time.Hour {
			item.Confidence = "low"
		} else if streamCount < 15 || time.Since(*result.CollectionStartedAt) < 60*24*time.Hour {
			item.Confidence = "medium"
		} else {
			item.Confidence = "high"
		}
		result.ParticipantCount++
		if item.Rank <= limit {
			result.Items = append(result.Items, item)
		}
		if meTwitchID != "" && item.TwitchID == meTwitchID {
			copy := item
			result.Me = &copy
		}
	}
	if err := rows.Err(); err != nil {
		return domain.ViewerRating{}, err
	}

	statusRows, err := s.pool.Query(ctx, `
		SELECT channel_login, collector_status, collector_user, last_event_at, last_error, updated_at
		FROM twitch_rating_state WHERE channel_login = ANY($1) ORDER BY channel_login`, channels)
	if err != nil {
		return domain.ViewerRating{}, err
	}
	defer statusRows.Close()
	for statusRows.Next() {
		var item domain.ViewerRatingStatus
		if err := statusRows.Scan(&item.Channel, &item.Status, &item.CollectorUser, &item.LastEventAt, &item.LastError, &item.UpdatedAt); err != nil {
			return domain.ViewerRating{}, err
		}
		result.Collectors = append(result.Collectors, item)
	}
	return result, statusRows.Err()
}
