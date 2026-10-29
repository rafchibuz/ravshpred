package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ravshann/predlozhka/backend/internal/domain"
	"github.com/ravshann/predlozhka/backend/internal/store"
)

func (s *Store) ListRavshTOK(ctx context.Context, params store.RavshTOKParams) (domain.RavshTOKFeed, error) {
	if params.ExcludeIDs == nil {
		params.ExcludeIDs = []string{}
	}
	limit := params.Limit
	if limit < 1 {
		limit = 12
	}
	if limit > 30 {
		limit = 30
	}
	if params.Offset < 0 {
		params.Offset = 0
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			s.id::text,s.title,s.submitter_comment,s.source_url,rm.platform,rm.status,
			rm.media_path,rm.poster_path,rm.duration_seconds,rm.width,rm.height,
			COALESCE(v.likes,0),COALESCE(v.dislikes,0),COALESCE(uv.value,0),
			COALESCE(views.total,0),COALESCE(views.unique_viewers,0),
			rv.user_id IS NOT NULL,sm.submission_id IS NOT NULL,
			u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),
			u.display_name,u.avatar_url,u.role::text,u.created_at,s.created_at
		FROM submissions s
		JOIN ravshtok_media rm ON rm.submission_id=s.id
		JOIN users u ON u.id=s.author_id
		LEFT JOIN streamer_marks sm ON sm.submission_id=s.id
		LEFT JOIN ravshtok_views rv ON rv.submission_id=s.id AND rv.user_id::text=$1
		LEFT JOIN votes uv ON uv.submission_id=s.id AND uv.user_id::text=$1
		LEFT JOIN LATERAL (
			SELECT count(*) FILTER (WHERE value=1) AS likes,
			       count(*) FILTER (WHERE value=-1) AS dislikes
			FROM votes WHERE submission_id=s.id
		) v ON true
		LEFT JOIN LATERAL (
			SELECT sum(view_count) AS total,count(*) AS unique_viewers
			FROM ravshtok_views WHERE submission_id=s.id
		) views ON true
		WHERE s.status='approved' AND s.deleted_at IS NULL AND s.source_type='short_video'
		  AND rm.status='ready'
		  AND NOT (s.id::text = ANY($8::text[]))
		  AND ($2='all' OR rm.platform=$2)
		  AND (
			$3='all'
			OR ($3='new' AND CASE WHEN $4='owner' THEN sm.submission_id IS NULL ELSE rv.user_id IS NULL END)
			OR ($3='watched' AND CASE WHEN $4='owner' THEN sm.submission_id IS NOT NULL ELSE rv.user_id IS NOT NULL END)
		  )
		ORDER BY
			CASE WHEN $5='random' THEN random() END,
			CASE WHEN $5='popular' THEN COALESCE(v.likes,0) END DESC,
			CASE WHEN $5='popular' THEN COALESCE(v.dislikes,0) END ASC,
			s.created_at DESC,s.id DESC
		LIMIT $6 OFFSET $7`, params.UserID, params.Platform, params.Mode, string(params.Role), params.Sort, limit+1, params.Offset, params.ExcludeIDs)
	if err != nil {
		return domain.RavshTOKFeed{}, err
	}
	defer rows.Close()

	items := make([]domain.RavshTOKItem, 0, limit)
	for rows.Next() {
		var item domain.RavshTOKItem
		var mediaPath, posterPath string
		var role string
		if err := rows.Scan(
			&item.ID, &item.Title, &item.Description, &item.SourceURL, &item.Platform, &item.MediaStatus,
			&mediaPath, &posterPath, &item.DurationSeconds, &item.Width, &item.Height,
			&item.Likes, &item.Dislikes, &item.UserVote, &item.Views, &item.UniqueViewers, &item.UserViewed, &item.StreamerWatched,
			&item.Author.ID, &item.Author.TwitchID, &item.Author.Login, &item.Author.Display,
			&item.Author.AvatarURL, &role, &item.Author.CreatedAt, &item.CreatedAt,
		); err != nil {
			return domain.RavshTOKFeed{}, err
		}
		item.Author.Role = domain.Role(role)
		if mediaPath != "" {
			item.PlaybackURL = "/media/" + mediaPath
		}
		if posterPath != "" {
			item.PosterURL = "/media/" + posterPath
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.RavshTOKFeed{}, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	unavailable := []string{}
	if len(params.ExcludeIDs) > 0 {
		missing, err := s.pool.Query(ctx, `
			SELECT id FROM unnest($1::text[]) AS known(id)
			WHERE NOT EXISTS (
				SELECT 1 FROM submissions s JOIN ravshtok_media rm ON rm.submission_id=s.id
				WHERE s.id::text=known.id AND s.deleted_at IS NULL AND s.status='approved' AND rm.status='ready'
			)`, params.ExcludeIDs)
		if err != nil {
			return domain.RavshTOKFeed{}, err
		}
		defer missing.Close()
		for missing.Next() {
			var id string
			if err := missing.Scan(&id); err != nil {
				return domain.RavshTOKFeed{}, err
			}
			unavailable = append(unavailable, id)
		}
		if err := missing.Err(); err != nil {
			return domain.RavshTOKFeed{}, err
		}
	}
	return domain.RavshTOKFeed{Items: items, HasMore: hasMore, UnavailableIDs: unavailable}, nil
}

func (s *Store) MarkRavshTOKViewed(ctx context.Context, submissionID, userID string, role domain.Role) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var ready bool
	if err := tx.QueryRow(ctx, `
		SELECT rm.status='ready'
		FROM submissions s JOIN ravshtok_media rm ON rm.submission_id=s.id
		WHERE s.id::text=$1 AND s.status='approved' AND s.deleted_at IS NULL
		  AND s.source_type='short_video'`, submissionID).Scan(&ready); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		return err
	}
	if !ready {
		return store.ErrConflict
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO ravshtok_views(submission_id,user_id)
		VALUES($1,$2)
		ON CONFLICT(submission_id,user_id) DO UPDATE
		SET last_viewed_at=now(),view_count=ravshtok_views.view_count+1`, submissionID, userID); err != nil {
		return err
	}
	if role == domain.RoleOwner {
		command, err := tx.Exec(ctx, `
			INSERT INTO streamer_marks(submission_id,marked_by)
			VALUES($1,$2)
			ON CONFLICT(submission_id) DO NOTHING`, submissionID, userID)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE ravshtok_media
			SET expires_at=LEAST(COALESCE(expires_at,now()+interval '30 days'),now()+interval '30 days'),updated_at=now()
			WHERE submission_id::text=$1`, submissionID); err != nil {
			return err
		}
		if command.RowsAffected() > 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO audit_log(actor_id,action,target_type,target_id)
				VALUES($1,'ravshtok_watched','submission',$2)`, userID, submissionID); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) RavshTOKVoteCounts(ctx context.Context, submissionID string) (int64, int64, error) {
	var likes, dislikes int64
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE v.value=1),count(*) FILTER (WHERE v.value=-1)
		FROM submissions s
		LEFT JOIN votes v ON v.submission_id=s.id
		WHERE s.id::text=$1 AND s.source_type='short_video' AND s.status='approved' AND s.deleted_at IS NULL
		GROUP BY s.id`, submissionID).Scan(&likes, &dislikes)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, store.ErrNotFound
	}
	return likes, dislikes, err
}
