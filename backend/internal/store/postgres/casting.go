package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/ravshann/predlozhka/backend/internal/domain"
	"github.com/ravshann/predlozhka/backend/internal/store"
)

func (s *Store) HasCastingAccess(ctx context.Context, userID string) (bool, error) {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT u.role='owner' OR EXISTS(
			SELECT 1 FROM casting_reviewers cr WHERE cr.user_id=u.id
		)
		FROM users u WHERE u.id::text=$1 AND u.deleted_at IS NULL`, userID).Scan(&allowed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return allowed, err
}

func (s *Store) ListCastingReviewers(ctx context.Context) ([]domain.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id::text,COALESCE(u.twitch_id,''),COALESCE(u.twitch_login,''),
			u.display_name,u.avatar_url,u.role::text,u.created_at
		FROM casting_reviewers cr
		JOIN users u ON u.id=cr.user_id
		WHERE u.deleted_at IS NULL
		ORDER BY lower(u.display_name),lower(COALESCE(u.twitch_login,''))`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]domain.User, 0)
	for rows.Next() {
		var user domain.User
		if err := rows.Scan(&user.ID, &user.TwitchID, &user.Login, &user.Display, &user.AvatarURL, &user.Role, &user.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	return result, rows.Err()
}

func (s *Store) AssignCastingReviewer(ctx context.Context, login, actorID string) (domain.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, err
	}
	defer tx.Rollback(ctx)
	var user domain.User
	err = tx.QueryRow(ctx, `
		SELECT id::text,COALESCE(twitch_id,''),COALESCE(twitch_login,''),display_name,avatar_url,role::text,created_at
		FROM users WHERE lower(twitch_login)=lower($1) AND deleted_at IS NULL`, login).
		Scan(&user.ID, &user.TwitchID, &user.Login, &user.Display, &user.AvatarURL, &user.Role, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, store.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO casting_reviewers(user_id,granted_by) VALUES($1,$2)
		ON CONFLICT(user_id) DO UPDATE SET granted_by=EXCLUDED.granted_by`, user.ID, actorID); err != nil {
		return domain.User{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'grant_casting_access','user',$2)`, actorID, user.ID); err != nil {
		return domain.User{}, err
	}
	return user, tx.Commit(ctx)
}

func (s *Store) RemoveCastingReviewer(ctx context.Context, userID, actorID string) error {
	command, err := s.pool.Exec(ctx, `DELETE FROM casting_reviewers WHERE user_id::text=$1`, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return store.ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id) VALUES($1,'revoke_casting_access','user',$2)`, actorID, userID)
	return err
}

func (s *Store) CastingParticipants(ctx context.Context) (json.RawMessage, error) {
	var payload []byte
	err := s.pool.QueryRow(ctx, `SELECT participants FROM casting_dataset WHERE singleton=TRUE`).Scan(&payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return json.RawMessage("[]"), nil
	}
	return json.RawMessage(payload), err
}

func (s *Store) ReplaceCastingParticipants(ctx context.Context, participants json.RawMessage, actorID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM casting_scores`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM casting_winners`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO casting_dataset(singleton,participants,uploaded_by,updated_at)
		VALUES(TRUE,$1,$2,now())
		ON CONFLICT(singleton) DO UPDATE SET participants=EXCLUDED.participants,uploaded_by=EXCLUDED.uploaded_by,updated_at=now()`, participants, actorID); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(actor_id,action,target_type,target_id,metadata) VALUES($1,'replace_casting_participants','casting','dataset',jsonb_build_object('count',jsonb_array_length($2::jsonb)))`, actorID, participants); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CastingJudgeNames(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT name FROM casting_judges ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		result = append(result, name)
	}
	return result, rows.Err()
}

func (s *Store) ReplaceCastingJudgeNames(ctx context.Context, names []string, actorID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `DELETE FROM casting_scores WHERE judge_id > $1`, len(names)); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM casting_judges WHERE position > $1`, len(names)); err != nil {
		return err
	}
	for index, name := range names {
		name = strings.TrimSpace(name)
		if _, err = tx.Exec(ctx, `
			INSERT INTO casting_judges(position,name) VALUES($1,$2)
			ON CONFLICT(position) DO UPDATE SET name=EXCLUDED.name,updated_at=now()`, index+1, name); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO audit_log(actor_id,action,target_type,target_id,metadata)
		VALUES($1,'replace_casting_judges','casting','judges',jsonb_build_object('count',$2::int))`, actorID, len(names)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) CastingScores(ctx context.Context, _ string) (map[int]map[int]float64, error) {
	rows, err := s.pool.Query(ctx, `SELECT participant_id,judge_id,score FROM casting_scores`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int]map[int]float64)
	for rows.Next() {
		var participantID, judgeID int
		var score float64
		if err := rows.Scan(&participantID, &judgeID, &score); err != nil {
			return nil, err
		}
		if result[participantID] == nil {
			result[participantID] = make(map[int]float64)
		}
		result[participantID][judgeID] = score
	}
	return result, rows.Err()
}

func (s *Store) SetCastingScore(ctx context.Context, userID string, participantID, judgeID int, score float64) error {
	command, err := s.pool.Exec(ctx, `
		INSERT INTO casting_scores(user_id,participant_id,judge_id,score) VALUES($1,$2,$3,$4)
		ON CONFLICT(participant_id,judge_id) DO UPDATE SET user_id=EXCLUDED.user_id,score=EXCLUDED.score,updated_at=now()`, userID, participantID, judgeID, score)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return store.ErrConflict
	}
	return nil
}

func (s *Store) CastingWinners(ctx context.Context) ([]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT participant_id FROM casting_winners ORDER BY participant_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]int, 0)
	for rows.Next() {
		var participantID int
		if err := rows.Scan(&participantID); err != nil {
			return nil, err
		}
		result = append(result, participantID)
	}
	return result, rows.Err()
}

func (s *Store) SetCastingWinner(ctx context.Context, participantID int, selected bool, actorID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if selected {
		_, err = tx.Exec(ctx, `
			INSERT INTO casting_winners(participant_id,selected_by) VALUES($1,$2)
			ON CONFLICT(participant_id) DO UPDATE SET selected_by=EXCLUDED.selected_by,updated_at=now()`, participantID, actorID)
	} else {
		_, err = tx.Exec(ctx, `DELETE FROM casting_winners WHERE participant_id=$1`, participantID)
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO audit_log(actor_id,action,target_type,target_id,metadata)
		VALUES($1,'set_casting_winner','casting_participant',$2::text,jsonb_build_object('selected',$3::boolean))`, actorID, participantID, selected); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
