package postgres

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ravshann/predlozhka/backend/internal/domain"
	"github.com/ravshann/predlozhka/backend/internal/store"
)

//go:embed migrations/000002_news.sql
var newsSchema string

//go:embed migrations/000003_portal.sql
var portalSchema string

//go:embed migrations/000004_movies.sql
var moviesSchema string

type Store struct {
	pool *pgxpool.Pool
}

type YouTubeRefreshTarget struct {
	SubmissionID string
	YouTubeID    string
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	result := &Store{pool: pool}
	if err := result.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if _, err := pool.Exec(ctx, newsSchema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply news schema: %w", err)
	}
	if _, err := pool.Exec(ctx, portalSchema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply portal schema: %w", err)
	}
	if _, err := pool.Exec(ctx, moviesSchema); err != nil {
		pool.Close()
		return nil, fmt.Errorf("apply movies schema: %w", err)
	}
	return result, nil
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }
func (s *Store) Close()                         { s.pool.Close() }

func (s *Store) ListCategories(ctx context.Context) ([]domain.Category, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, slug, name, is_system, sort_order, created_at
		FROM categories ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Category
	for rows.Next() {
		var item domain.Category
		if err := rows.Scan(&item.ID, &item.Slug, &item.Name, &item.IsSystem, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

const videoSelect = `
	SELECT
		s.id::text, s.youtube_id, s.youtube_url, s.title, s.channel_title, s.thumbnail_url,
		s.duration_seconds, s.view_count, s.youtube_like_count, s.status::text,
		s.submitter_comment, s.moderator_comment, s.version, s.created_at, s.updated_at,
		s.kinopoisk_url, s.movie_title, s.movie_year, s.movie_studio, s.movie_rating,
		u.id::text, COALESCE(u.twitch_id,''), COALESCE(u.twitch_login,''), u.display_name,
		u.avatar_url, u.role::text, u.created_at,
		c.id::text, c.slug, c.name, c.is_system, c.sort_order, c.created_at,
		EXISTS(SELECT 1 FROM streamer_marks sm WHERE sm.submission_id = s.id),
		COALESCE((SELECT SUM(v.value) FROM votes v WHERE v.submission_id = s.id), 0),
		COALESCE((SELECT v.value FROM votes v WHERE v.submission_id = s.id AND v.user_id::text = $1), 0)
	FROM submissions s
	JOIN users u ON u.id = s.author_id
	JOIN categories c ON c.id = s.category_id
`

func (s *Store) ListFeed(ctx context.Context, params store.FeedParams) ([]domain.Video, string, error) {
	limit := clampLimit(params.Limit)
	cursorTime, cursorID := decodeCursor(params.Cursor)
	query := videoSelect + `
		WHERE s.status IN ('approved', 'pending', 'rejected') AND s.deleted_at IS NULL
		  AND ($2 = '' OR c.slug = $2)
		  AND ($3::boolean IS NULL OR EXISTS(SELECT 1 FROM streamer_marks sm WHERE sm.submission_id = s.id) = $3)
		  AND ($4::timestamptz IS NULL OR (s.created_at, s.id::text) < ($4, $5))
		ORDER BY s.created_at DESC, s.id DESC
		LIMIT $6`
	rows, err := s.pool.Query(ctx, query, params.UserID, params.Category, params.Watched, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	items, err := scanVideos(rows)
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeCursor(last.CreatedAt, last.ID)
		items = items[:limit]
	}
	return items, next, nil
}

func (s *Store) ListYouTubeRefreshTargets(ctx context.Context, limit int) ([]YouTubeRefreshTarget, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, youtube_id
		FROM submissions
		WHERE deleted_at IS NULL
		ORDER BY metadata_fetched_at ASC NULLS FIRST, created_at DESC
		LIMIT $1`, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]YouTubeRefreshTarget, 0)
	for rows.Next() {
		var item YouTubeRefreshTarget
		if err := rows.Scan(&item.SubmissionID, &item.YouTubeID); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) UpdateYouTubeMetadata(
	ctx context.Context,
	submissionID, title, channelTitle, thumbnailURL string,
	durationSeconds int,
	viewCount, likeCount int64,
) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE submissions
		SET title=$2,
			channel_title=$3,
			thumbnail_url=$4,
			duration_seconds=$5,
			view_count=$6,
			youtube_like_count=$7,
			metadata_fetched_at=now(),
			updated_at=now()
		WHERE id::text=$1 AND deleted_at IS NULL`,
		submissionID, title, channelTitle, thumbnailURL, durationSeconds, viewCount, likeCount)
	return err
}

func (s *Store) ListByStatus(ctx context.Context, status domain.SubmissionStatus, limit int) ([]domain.Video, error) {
	rows, err := s.pool.Query(ctx, videoSelect+`
		WHERE s.status = $2 AND s.deleted_at IS NULL
		ORDER BY s.created_at ASC LIMIT $3`, "", status, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVideos(rows)
}

func (s *Store) ListMine(ctx context.Context, userID string, limit int) ([]domain.Video, error) {
	rows, err := s.pool.Query(ctx, videoSelect+`
		WHERE s.author_id::text = $2 AND s.deleted_at IS NULL
		ORDER BY s.created_at DESC LIMIT $3`, userID, userID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVideos(rows)
}

func (s *Store) CreateSubmission(ctx context.Context, input store.CreateSubmissionInput) (domain.Video, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return domain.Video{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, input.AuthorID); err != nil {
		return domain.Video{}, err
	}
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*) FROM submissions
		WHERE author_id::text = $1 AND created_at >= now() - interval '24 hours' AND deleted_at IS NULL`,
		input.AuthorID).Scan(&count); err != nil {
		return domain.Video{}, err
	}
	if count >= input.DailyLimit {
		return domain.Video{}, store.ErrDailyLimit
	}
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO submissions (
			youtube_id, youtube_url, title, channel_title, thumbnail_url,
			duration_seconds, view_count, youtube_like_count, metadata_fetched_at,
			author_id, category_id, submitter_comment,
			kinopoisk_url, movie_title, movie_year, movie_studio, movie_rating
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id::text`,
		input.YouTubeID, input.YouTubeURL, input.Title, input.ChannelTitle, input.ThumbnailURL,
		input.DurationSeconds, input.ViewCount, input.YouTubeLikeCount, input.AuthorID, input.CategoryID, input.Comment,
		input.KinopoiskURL, input.MovieTitle, input.MovieYear, input.MovieStudio, input.MovieRating,
	).Scan(&id)
	if isUniqueViolation(err) {
		return domain.Video{}, store.ErrDuplicate
	}
	if err != nil {
		return domain.Video{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'submit','submission',$2)`, input.AuthorID, id); err != nil {
		return domain.Video{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Video{}, err
	}
	return s.getVideo(ctx, id, input.AuthorID)
}

func (s *Store) Vote(ctx context.Context, submissionID, userID string, value int) (int64, int, error) {
	if value != -1 && value != 1 {
		return 0, 0, errors.New("vote must be -1 or 1")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback(ctx)
	var authorID string
	var approved bool
	if err := tx.QueryRow(ctx, `
		SELECT author_id::text,status='approved'
		FROM submissions WHERE id::text=$1 AND deleted_at IS NULL
		FOR UPDATE`, submissionID).Scan(&authorID, &approved); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, store.ErrNotFound
		}
		return 0, 0, err
	}
	if !approved {
		return 0, 0, store.ErrConflict
	}
	if authorID == userID {
		var allowSelf bool
		err := tx.QueryRow(ctx, `
			SELECT COALESCE((SELECT value::text::boolean FROM system_settings WHERE key='allow_self_vote'),false)`).
			Scan(&allowSelf)
		if err != nil {
			return 0, 0, err
		}
		if !allowSelf {
			return 0, 0, store.ErrSelfVote
		}
	}
	var current int
	err = tx.QueryRow(ctx, `SELECT value FROM votes WHERE submission_id::text=$1 AND user_id::text=$2 FOR UPDATE`, submissionID, userID).Scan(&current)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = tx.Exec(ctx, `INSERT INTO votes(submission_id,user_id,value) VALUES($1,$2,$3)`, submissionID, userID, value)
		current = value
	case err != nil:
		return 0, 0, err
	case current == value:
		_, err = tx.Exec(ctx, `DELETE FROM votes WHERE submission_id::text=$1 AND user_id::text=$2`, submissionID, userID)
		current = 0
	default:
		_, err = tx.Exec(ctx, `UPDATE votes SET value=$3,updated_at=now() WHERE submission_id::text=$1 AND user_id::text=$2`, submissionID, userID, value)
		current = value
	}
	if err != nil {
		return 0, 0, err
	}
	var rating int64
	if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(value),0) FROM votes WHERE submission_id::text=$1`, submissionID).Scan(&rating); err != nil {
		return 0, 0, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id,metadata) VALUES($1,'vote','submission',$2,jsonb_build_object('value',$3))`, userID, submissionID, current); err != nil {
		return 0, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, 0, err
	}
	return rating, current, nil
}

func (s *Store) Decide(ctx context.Context, input store.DecideInput) (domain.Video, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Video{}, err
	}
	defer tx.Rollback(ctx)
	var from domain.SubmissionStatus
	var authorID string
	var videoTitle, categoryName string
	err = tx.QueryRow(ctx, `
		SELECT s.status::text,s.author_id::text,s.title,c.name
		FROM submissions s JOIN categories c ON c.id=s.category_id
		WHERE s.id::text=$1 AND s.deleted_at IS NULL FOR UPDATE`,
		input.SubmissionID).Scan(&from, &authorID, &videoTitle, &categoryName)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Video{}, store.ErrNotFound
	}
	if err != nil {
		return domain.Video{}, err
	}
	if !domain.CanTransition(from, input.Status) {
		return domain.Video{}, store.ErrConflict
	}
	command, err := tx.Exec(ctx, `
		UPDATE submissions SET status=$2,moderator_comment=$3,version=version+1,updated_at=now()
		WHERE id::text=$1 AND version=$4`, input.SubmissionID, input.Status, input.Comment, input.Version)
	if err != nil {
		return domain.Video{}, err
	}
	if command.RowsAffected() != 1 {
		return domain.Video{}, store.ErrVersionConflict
	}
	if input.Status != domain.StatusApproved {
		if _, err := tx.Exec(ctx, `DELETE FROM streamer_marks WHERE submission_id::text=$1`, input.SubmissionID); err != nil {
			return domain.Video{}, err
		}
	}
	action := "reject"
	if input.Status == domain.StatusApproved {
		action = "approve"
	} else if input.Status == domain.StatusChangesRequested {
		action = "request_changes"
	} else if input.Status == domain.StatusPending {
		action = "restore"
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO moderation_actions(submission_id,moderator_id,action,from_status,to_status,reason_code,comment,submission_version)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		input.SubmissionID, input.ModeratorID, action, from, input.Status, input.ReasonCode, input.Comment, input.Version+1)
	if err != nil {
		return domain.Video{}, err
	}
	if input.Status == domain.StatusApproved || input.Status == domain.StatusRejected || input.Status == domain.StatusChangesRequested {
		title := map[domain.SubmissionStatus]string{
			domain.StatusApproved:         "Видео одобрено",
			domain.StatusRejected:         "Видео отклонено",
			domain.StatusChangesRequested: "Требуется исправление",
		}[input.Status]
		body := fmt.Sprintf("«%s» · %s", videoTitle, categoryName)
		if input.Comment != "" {
			body += ": " + input.Comment
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO notifications(user_id,type,title,body,submission_id)
			VALUES($1,'moderation',$2,$3,$4)`, authorID, title, body, input.SubmissionID)
		if err != nil {
			return domain.Video{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Video{}, err
	}
	return s.getVideo(ctx, input.SubmissionID, input.ModeratorID)
}

func (s *Store) SetWatched(ctx context.Context, submissionID, actorID string, watched bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var approved bool
	if err := tx.QueryRow(ctx, `SELECT status='approved' FROM submissions WHERE id::text=$1 AND deleted_at IS NULL`, submissionID).Scan(&approved); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		return err
	}
	if !approved {
		return store.ErrConflict
	}
	if watched {
		_, err = tx.Exec(ctx, `
			INSERT INTO streamer_marks(submission_id,marked_by) VALUES($1,$2)
			ON CONFLICT(submission_id) DO UPDATE SET marked_by=excluded.marked_by,marked_at=now()`, submissionID, actorID)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM streamer_marks WHERE submission_id::text=$1`, submissionID)
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_log(actor_id,action,target_type,target_id)
		VALUES($1,$2,'submission',$3)`, actorID, map[bool]string{true: "watched_on", false: "watched_off"}[watched], submissionID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpdateVideoCategory(ctx context.Context, submissionID, categoryID, actorID string) error {
	command, err := s.pool.Exec(ctx, `
		UPDATE submissions SET category_id=$2,updated_at=now(),version=version+1
		WHERE id::text=$1 AND deleted_at IS NULL`, submissionID, categoryID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return store.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id,metadata) VALUES($1,'category_change','submission',$2,jsonb_build_object('category_id',$3))`, actorID, submissionID, categoryID)
	return err
}

func (s *Store) UpdateMovieMetadata(ctx context.Context, input store.UpdateMovieInput) (domain.Video, error) {
	command, err := s.pool.Exec(ctx, `
		UPDATE submissions SET kinopoisk_url=$3,movie_title=$4,movie_year=$5,
			movie_studio=$6,movie_rating=$7,updated_at=now(),version=version+1
		WHERE id::text=$1 AND version=$2 AND deleted_at IS NULL`,
		input.SubmissionID, input.Version, input.KinopoiskURL, input.MovieTitle,
		input.MovieYear, input.MovieStudio, input.MovieRating)
	if err != nil {
		return domain.Video{}, err
	}
	if command.RowsAffected() != 1 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM submissions WHERE id::text=$1 AND deleted_at IS NULL)`, input.SubmissionID).Scan(&exists); err != nil {
			return domain.Video{}, err
		}
		if exists {
			return domain.Video{}, store.ErrVersionConflict
		}
		return domain.Video{}, store.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'movie_metadata_update','submission',$2)`, input.ModeratorID, input.SubmissionID)
	if err != nil {
		return domain.Video{}, err
	}
	return s.getVideo(ctx, input.SubmissionID, input.ModeratorID)
}

func (s *Store) DeleteVideo(ctx context.Context, submissionID, actorID string) error {
	command, err := s.pool.Exec(ctx, `UPDATE submissions SET deleted_at=now(),updated_at=now() WHERE id::text=$1 AND deleted_at IS NULL`, submissionID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return store.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'delete','submission',$2)`, actorID, submissionID)
	return err
}

func (s *Store) CreateCategory(ctx context.Context, slug, name, actorID string) (domain.Category, error) {
	var result domain.Category
	err := s.pool.QueryRow(ctx, `
		INSERT INTO categories(slug,name,sort_order)
		VALUES($1,$2,(SELECT COALESCE(max(sort_order),0)+10 FROM categories))
		RETURNING id::text,slug,name,is_system,sort_order,created_at`, slug, name).
		Scan(&result.ID, &result.Slug, &result.Name, &result.IsSystem, &result.SortOrder, &result.CreatedAt)
	if isUniqueViolation(err) {
		return domain.Category{}, store.ErrConflict
	}
	if err == nil {
		_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'create','category',$2)`, actorID, result.ID)
	}
	return result, err
}

func (s *Store) DeleteCategory(ctx context.Context, categoryID, actorID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var system bool
	if err := tx.QueryRow(ctx, `SELECT is_system FROM categories WHERE id::text=$1 FOR UPDATE`, categoryID).Scan(&system); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return store.ErrNotFound
		}
		return err
	}
	if system {
		return store.ErrConflict
	}
	var fallbackID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM categories WHERE slug='uncategorized'`).Scan(&fallbackID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE submissions SET category_id=$2,updated_at=now() WHERE category_id::text=$1`, categoryID, fallbackID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM categories WHERE id::text=$1`, categoryID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'delete','category',$2)`, actorID, categoryID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) AssignModerator(ctx context.Context, login, actorID string) (domain.User, error) {
	var result domain.User
	err := s.pool.QueryRow(ctx, `
		UPDATE users SET role='moderator',updated_at=now()
		WHERE lower(twitch_login)=lower($1) AND deleted_at IS NULL AND role <> 'owner'
		RETURNING id::text,COALESCE(twitch_id,''),COALESCE(twitch_login,''),display_name,avatar_url,role::text,created_at`, login).
		Scan(&result.ID, &result.TwitchID, &result.Login, &result.Display, &result.AvatarURL, &result.Role, &result.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, store.ErrNotFound
	}
	if err == nil {
		_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'assign_moderator','user',$2)`, actorID, result.ID)
	}
	return result, err
}

func (s *Store) RemoveModerator(ctx context.Context, userID, actorID string) error {
	command, err := s.pool.Exec(ctx, `UPDATE users SET role='user',updated_at=now() WHERE id::text=$1 AND role='moderator'`, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return store.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'remove_moderator','user',$2)`, actorID, userID)
	return err
}

func (s *Store) ListModerators(ctx context.Context) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text,COALESCE(twitch_id,''),COALESCE(twitch_login,''),
			display_name,avatar_url,role::text,created_at
		FROM users WHERE role='moderator' AND deleted_at IS NULL
		ORDER BY lower(display_name)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.User
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.TwitchID, &user.Login, &user.Display, &user.AvatarURL, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	return result, rows.Err()
}

func (s *Store) ListUsersStats(ctx context.Context, limit int) ([]domain.UserStats, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),u.display_name,
			u.avatar_url,u.role::text,u.created_at,u.last_login_at,
			count(s.id) FILTER (WHERE s.deleted_at IS NULL),
			count(s.id) FILTER (WHERE s.status='pending' AND s.deleted_at IS NULL),
			count(s.id) FILTER (WHERE s.status='approved' AND s.deleted_at IS NULL),
			count(s.id) FILTER (WHERE s.status='rejected' AND s.deleted_at IS NULL),
			count(sm.submission_id) FILTER (WHERE s.deleted_at IS NULL),
			count(s.id) FILTER (WHERE s.deleted_at IS NOT NULL),
			(SELECT count(*) FROM news_comments nc WHERE nc.author_id=u.id)
		FROM users u
		LEFT JOIN submissions s ON s.author_id=u.id
		LEFT JOIN streamer_marks sm ON sm.submission_id=s.id
		WHERE u.deleted_at IS NULL
		GROUP BY u.id
		ORDER BY u.created_at DESC LIMIT $1`, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.UserStats, 0)
	for rows.Next() {
		var item domain.UserStats
		if err := rows.Scan(&item.User.ID, &item.User.TwitchID, &item.User.Login, &item.User.Display,
			&item.User.AvatarURL, &item.User.Role, &item.User.CreatedAt, &item.LastLogin,
			&item.Total, &item.Pending, &item.Approved, &item.Rejected, &item.Watched, &item.Deleted, &item.Comments); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListNotifications(ctx context.Context, userID string, limit int) ([]domain.Notification, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text,type,title,body,read_at,created_at
		FROM notifications
		WHERE user_id::text=$1
		ORDER BY created_at DESC LIMIT $2`, userID, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Notification
	for rows.Next() {
		var item domain.Notification
		if err := rows.Scan(&item.ID, &item.Type, &item.Title, &item.Body, &item.ReadAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ListNews(ctx context.Context, limit int) ([]domain.NewsPost, error) {
	limit = clampLimit(limit)
	rows, err := s.pool.Query(ctx, `
		SELECT p.id::text,p.title,p.body,p.created_at,p.updated_at,
			u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),
			u.display_name,u.avatar_url,u.role::text,u.created_at
		FROM news_posts p
		JOIN users u ON u.id=p.author_id
		ORDER BY p.created_at DESC,p.id DESC
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	posts := make([]domain.NewsPost, 0)
	index := make(map[string]int)
	for rows.Next() {
		var post domain.NewsPost
		if err := rows.Scan(
			&post.ID, &post.Title, &post.Body, &post.CreatedAt, &post.UpdatedAt,
			&post.Author.ID, &post.Author.TwitchID, &post.Author.Login,
			&post.Author.Display, &post.Author.AvatarURL, &post.Author.Role, &post.Author.CreatedAt,
		); err != nil {
			return nil, err
		}
		post.Comments = []domain.NewsComment{}
		index[post.ID] = len(posts)
		posts = append(posts, post)
	}
	if err := rows.Err(); err != nil || len(posts) == 0 {
		return posts, err
	}

	commentRows, err := s.pool.Query(ctx, `
		SELECT c.id::text,c.post_id::text,c.body,c.created_at,
			u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),
			u.display_name,u.avatar_url,u.role::text,u.created_at
		FROM news_comments c
		JOIN users u ON u.id=c.author_id
		WHERE c.post_id IN (
			SELECT id FROM news_posts ORDER BY created_at DESC,id DESC LIMIT $1
		)
		ORDER BY c.created_at ASC,c.id ASC`, limit)
	if err != nil {
		return nil, err
	}
	defer commentRows.Close()
	for commentRows.Next() {
		var comment domain.NewsComment
		if err := commentRows.Scan(
			&comment.ID, &comment.PostID, &comment.Body, &comment.CreatedAt,
			&comment.Author.ID, &comment.Author.TwitchID, &comment.Author.Login,
			&comment.Author.Display, &comment.Author.AvatarURL, &comment.Author.Role, &comment.Author.CreatedAt,
		); err != nil {
			return nil, err
		}
		if position, ok := index[comment.PostID]; ok {
			posts[position].Comments = append(posts[position].Comments, comment)
		}
	}
	return posts, commentRows.Err()
}

func (s *Store) CreateNewsPost(ctx context.Context, authorID, title, body string) (domain.NewsPost, error) {
	var postID string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO news_posts(author_id,title,body)
		VALUES($1,$2,$3)
		RETURNING id::text`, authorID, title, body).Scan(&postID)
	if err != nil {
		return domain.NewsPost{}, err
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO audit_log(actor_id,action,target_type,target_id)
		VALUES($1,'create','news_post',$2)`, authorID, postID); err != nil {
		return domain.NewsPost{}, err
	}
	return s.newsPostByID(ctx, postID)
}

func (s *Store) newsPostByID(ctx context.Context, postID string) (domain.NewsPost, error) {
	var post domain.NewsPost
	err := s.pool.QueryRow(ctx, `
		SELECT p.id::text,p.title,p.body,p.created_at,p.updated_at,
			u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),
			u.display_name,u.avatar_url,u.role::text,u.created_at
		FROM news_posts p
		JOIN users u ON u.id=p.author_id
		WHERE p.id::text=$1`, postID).Scan(
		&post.ID, &post.Title, &post.Body, &post.CreatedAt, &post.UpdatedAt,
		&post.Author.ID, &post.Author.TwitchID, &post.Author.Login,
		&post.Author.Display, &post.Author.AvatarURL, &post.Author.Role, &post.Author.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NewsPost{}, store.ErrNotFound
	}
	post.Comments = []domain.NewsComment{}
	return post, err
}

func (s *Store) DeleteNewsPost(ctx context.Context, postID, actorID string) error {
	command, err := s.pool.Exec(ctx, `DELETE FROM news_posts WHERE id::text=$1`, postID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO audit_log(actor_id,action,target_type,target_id)
		VALUES($1,'delete','news_post',$2)`, actorID, postID)
	return err
}

func (s *Store) CreateNewsComment(ctx context.Context, postID, authorID, body string) (domain.NewsComment, error) {
	var comment domain.NewsComment
	err := s.pool.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO news_comments(post_id,author_id,body)
			SELECT p.id,$2,$3 FROM news_posts p WHERE p.id::text=$1
			RETURNING id,post_id,author_id,body,created_at
		)
		SELECT i.id::text,i.post_id::text,i.body,i.created_at,
			u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),
			u.display_name,u.avatar_url,u.role::text,u.created_at
		FROM inserted i
		JOIN users u ON u.id=i.author_id`, postID, authorID, body).Scan(
		&comment.ID, &comment.PostID, &comment.Body, &comment.CreatedAt,
		&comment.Author.ID, &comment.Author.TwitchID, &comment.Author.Login,
		&comment.Author.Display, &comment.Author.AvatarURL, &comment.Author.Role, &comment.Author.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NewsComment{}, store.ErrNotFound
	}
	if err == nil {
		_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'comment','news_post',$2)`, authorID, postID)
	}
	return comment, err
}

func (s *Store) DeleteNewsComment(ctx context.Context, commentID, actorID string, canManage bool) error {
	command, err := s.pool.Exec(ctx, `
		DELETE FROM news_comments
		WHERE id::text=$1 AND (author_id::text=$2 OR $3)`, commentID, actorID, canManage)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return store.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'delete_comment','news_comment',$2)`, actorID, commentID)
	return err
}

func (s *Store) MarkNotificationRead(ctx context.Context, userID, notificationID string) error {
	command, err := s.pool.Exec(ctx, `
		UPDATE notifications SET read_at=COALESCE(read_at,now())
		WHERE id::text=$1 AND user_id::text=$2`, notificationID, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return store.ErrNotFound
	}
	return nil
}

func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notifications SET read_at=now()
		WHERE user_id::text=$1 AND read_at IS NULL`, userID)
	return err
}

func (s *Store) ListAudit(ctx context.Context, limit int) ([]domain.AuditEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id,a.action,a.target_type,a.target_id,a.metadata,a.created_at,
			COALESCE(u.id::text,''),COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),
			COALESCE(u.display_name,''),COALESCE(u.avatar_url,''),COALESCE(u.role::text,'user'),
			COALESCE(u.created_at,a.created_at)
		FROM audit_log a
		LEFT JOIN users u ON u.id=a.actor_id
		ORDER BY a.created_at DESC LIMIT $1`, clampLimit(limit))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.AuditEntry
	for rows.Next() {
		var item domain.AuditEntry
		var actor domain.User
		var actorID string
		if err := rows.Scan(&item.ID, &item.Action, &item.TargetType, &item.TargetID, &item.Metadata, &item.CreatedAt,
			&actorID, &actor.TwitchID, &actor.Login, &actor.Display, &actor.AvatarURL, &actor.Role, &actor.CreatedAt); err != nil {
			return nil, err
		}
		if actorID != "" {
			actor.ID = actorID
			item.Actor = &actor
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) GetSettings(ctx context.Context) (domain.GlobalSettings, error) {
	result := domain.GlobalSettings{
		SubmissionDailyLimit: 3,
		CommentLimit:         500,
		PublicFeedEnabled:    true,
		AllowSelfVote:        false,
		Socials:              domain.SocialLinks{Twitch: "https://www.twitch.tv/ravshann"},
		SocialItems:          []domain.SocialItem{{Name: "Twitch", URL: "https://www.twitch.tv/ravshann"}},
	}
	var hasSocialItems bool
	rows, err := s.pool.Query(ctx, `SELECT key,value::text FROM system_settings`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return result, err
		}
		switch key {
		case "submission_daily_limit":
			result.SubmissionDailyLimit, _ = strconv.Atoi(value)
		case "submission_comment_limit":
			result.CommentLimit, _ = strconv.Atoi(value)
		case "public_feed_enabled":
			result.PublicFeedEnabled, _ = strconv.ParseBool(value)
		case "allow_self_vote":
			result.AllowSelfVote, _ = strconv.ParseBool(value)
		case "social_twitch":
			_ = json.Unmarshal([]byte(value), &result.Socials.Twitch)
		case "social_youtube":
			_ = json.Unmarshal([]byte(value), &result.Socials.YouTube)
		case "social_telegram":
			_ = json.Unmarshal([]byte(value), &result.Socials.Telegram)
		case "social_vk":
			_ = json.Unmarshal([]byte(value), &result.Socials.VK)
		case "social_links":
			if json.Unmarshal([]byte(value), &result.SocialItems) == nil {
				hasSocialItems = true
			}
		case "support_links":
			_ = json.Unmarshal([]byte(value), &result.SupportItems)
		}
	}
	if !hasSocialItems {
		result.SocialItems = nil
		for _, item := range []domain.SocialItem{{Name: "Twitch", URL: result.Socials.Twitch}, {Name: "YouTube", URL: result.Socials.YouTube}, {Name: "Telegram", URL: result.Socials.Telegram}, {Name: "VK", URL: result.Socials.VK}} {
			if item.URL != "" {
				result.SocialItems = append(result.SocialItems, item)
			}
		}
	}
	for index := range result.SocialItems {
		if result.SocialItems[index].Section == "" {
			result.SocialItems[index].Section = "primary"
		}
	}
	for index := range result.SupportItems {
		result.SupportItems[index].Section = "support"
	}
	return result, rows.Err()
}

func (s *Store) UpdateSettings(ctx context.Context, settings domain.GlobalSettings, actorID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	values := map[string]any{
		"submission_daily_limit":   settings.SubmissionDailyLimit,
		"submission_comment_limit": settings.CommentLimit,
		"public_feed_enabled":      settings.PublicFeedEnabled,
		"allow_self_vote":          settings.AllowSelfVote,
		"social_twitch":            settings.Socials.Twitch,
		"social_youtube":           settings.Socials.YouTube,
		"social_telegram":          settings.Socials.Telegram,
		"social_vk":                settings.Socials.VK,
		"social_links":             settings.SocialItems,
		"support_links":            settings.SupportItems,
	}
	for key, value := range values {
		encoded, _ := json.Marshal(value)
		if _, err := tx.Exec(ctx, `
			INSERT INTO system_settings(key,value,updated_by,updated_at)
			VALUES($1,$2::jsonb,$3,now())
			ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_by=excluded.updated_by,updated_at=now()`,
			key, string(encoded), actorID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_log(actor_id,action,target_type,target_id,metadata)
		VALUES($1,'settings_update','system','global',$2::jsonb)`,
		actorID, mustJSON(values)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func mustJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func (s *Store) UpsertTwitchUser(ctx context.Context, twitchID, login, display, avatar string) (domain.User, error) {
	var result domain.User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users(twitch_id,twitch_login,display_name,avatar_url,last_login_at)
		VALUES($1,$2,$3,$4,now())
		ON CONFLICT(twitch_id) DO UPDATE SET
			twitch_login=excluded.twitch_login,
			display_name=excluded.display_name,
			avatar_url=excluded.avatar_url,
			last_login_at=now(),
			updated_at=now()
		RETURNING id::text,COALESCE(twitch_id,''),COALESCE(twitch_login,''),
			display_name,avatar_url,role::text,created_at`,
		twitchID, login, display, avatar).
		Scan(&result.ID, &result.TwitchID, &result.Login, &result.Display, &result.AvatarURL, &result.Role, &result.CreatedAt)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'login','user',$1::text)`, result.ID)
	}
	return result, err
}

func (s *Store) PromoteTwitchOwner(ctx context.Context, userID string) (domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		UPDATE users SET role='user',updated_at=now()
		WHERE role='owner' AND id::text<>$1 AND twitch_id IS NOT NULL`, userID); err != nil {
		return domain.User{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE users SET deleted_at=now(),updated_at=now()
		WHERE role='owner' AND id::text<>$1 AND twitch_id IS NULL`, userID); err != nil {
		return domain.User{}, err
	}
	var result domain.User
	err = tx.QueryRow(ctx, `
		UPDATE users SET role='owner',updated_at=now()
		WHERE id::text=$1 AND twitch_id IS NOT NULL AND deleted_at IS NULL
		RETURNING id::text,COALESCE(twitch_id,''),COALESCE(twitch_login,''),
			display_name,avatar_url,role::text,created_at`, userID).
		Scan(&result.ID, &result.TwitchID, &result.Login, &result.Display, &result.AvatarURL, &result.Role, &result.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, store.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, err
	}
	return result, nil
}

func (s *Store) CreateOAuthState(ctx context.Context, stateHash, verifierHash []byte, returnTo string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth_states(state_hash,code_verifier_hash,return_to,expires_at)
		VALUES($1,$2,$3,$4)`, stateHash, verifierHash, returnTo, expires)
	return err
}

func (s *Store) ConsumeOAuthState(ctx context.Context, stateHash []byte) (string, error) {
	var returnTo string
	err := s.pool.QueryRow(ctx, `
		DELETE FROM oauth_states
		WHERE state_hash=$1 AND expires_at>now()
		RETURNING return_to`, stateHash).Scan(&returnTo)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", store.ErrNotFound
	}
	return returnTo, err
}

func (s *Store) SessionByTokenHash(ctx context.Context, tokenHash []byte) (store.Session, error) {
	var result store.Session
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),u.display_name,u.avatar_url,u.role::text,u.created_at,s.csrf_token_hash,s.expires_at
		FROM sessions s JOIN users u ON u.id=s.user_id
		WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() AND u.deleted_at IS NULL`, tokenHash).
		Scan(&result.User.ID, &result.User.TwitchID, &result.User.Login, &result.User.Display, &result.User.AvatarURL, &result.User.Role, &result.User.CreatedAt, &result.CSRFHash, &result.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.Session{}, store.ErrNotFound
	}
	return result, err
}

func (s *Store) CreateSession(ctx context.Context, userID string, tokenHash, csrfHash []byte, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,csrf_token_hash,expires_at) VALUES($1,$2,$3,$4)`, tokenHash, userID, csrfHash, expires)
	return err
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (s *Store) getVideo(ctx context.Context, id, userID string) (domain.Video, error) {
	row := s.pool.QueryRow(ctx, videoSelect+` WHERE s.id::text=$2 AND s.deleted_at IS NULL`, userID, id)
	var video domain.Video
	if err := scanVideo(row, &video); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Video{}, store.ErrNotFound
		}
		return domain.Video{}, err
	}
	return video, nil
}

type scanner interface {
	Scan(...any) error
}

func scanVideo(row scanner, video *domain.Video) error {
	var role, status string
	err := row.Scan(
		&video.ID, &video.YouTubeID, &video.YouTubeURL, &video.Title, &video.ChannelTitle, &video.ThumbnailURL,
		&video.DurationSeconds, &video.ViewCount, &video.YouTubeLikeCount, &status,
		&video.SubmitterComment, &video.ModeratorComment, &video.Version, &video.CreatedAt, &video.UpdatedAt,
		&video.KinopoiskURL, &video.MovieTitle, &video.MovieYear, &video.MovieStudio, &video.MovieRating,
		&video.Author.ID, &video.Author.TwitchID, &video.Author.Login, &video.Author.Display,
		&video.Author.AvatarURL, &role, &video.Author.CreatedAt,
		&video.Category.ID, &video.Category.Slug, &video.Category.Name, &video.Category.IsSystem,
		&video.Category.SortOrder, &video.Category.CreatedAt,
		&video.Watched, &video.Rating, &video.UserVote,
	)
	video.Status = domain.SubmissionStatus(status)
	video.Author.Role = domain.Role(role)
	return err
}

func scanVideos(rows pgx.Rows) ([]domain.Video, error) {
	var items []domain.Video
	for rows.Next() {
		var video domain.Video
		if err := scanVideo(rows, &video); err != nil {
			return nil, err
		}
		items = append(items, video)
	}
	return items, rows.Err()
}

func clampLimit(value int) int {
	if value < 1 {
		return 20
	}
	if value > 100 {
		return 100
	}
	return value
}

func encodeCursor(created time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(created.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(value string) (*time.Time, string) {
	if value == "" {
		return nil, ""
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, ""
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return nil, ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, ""
	}
	return &parsed, parts[1]
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func TokenHash(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

var _ store.Store = (*Store)(nil)
