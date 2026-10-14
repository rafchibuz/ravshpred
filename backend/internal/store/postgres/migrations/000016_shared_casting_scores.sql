WITH latest AS (
    SELECT user_id, participant_id, score, updated_at,
           row_number() OVER (PARTITION BY participant_id ORDER BY updated_at DESC, user_id DESC) AS position
    FROM casting_scores
)
DELETE FROM casting_scores scores
USING latest
WHERE scores.user_id = latest.user_id
  AND scores.participant_id = latest.participant_id
  AND latest.position > 1;

CREATE UNIQUE INDEX IF NOT EXISTS casting_scores_participant_unique_idx
    ON casting_scores(participant_id);
