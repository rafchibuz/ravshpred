CREATE TABLE IF NOT EXISTS casting_reviewers (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS casting_scores (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    participant_id SMALLINT NOT NULL CHECK (participant_id > 0),
    score NUMERIC(3,1) NOT NULL CHECK (score BETWEEN 1 AND 10 AND score * 2 = trunc(score * 2)),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, participant_id)
);

CREATE INDEX IF NOT EXISTS casting_scores_user_updated_idx
    ON casting_scores(user_id, updated_at DESC);

ALTER TABLE casting_scores
    DROP CONSTRAINT IF EXISTS casting_scores_participant_id_check;

ALTER TABLE casting_scores
    ADD CONSTRAINT casting_scores_participant_id_check CHECK (participant_id > 0);

ALTER TABLE casting_scores
    ALTER COLUMN score TYPE NUMERIC(3,1) USING score::NUMERIC(3,1);

ALTER TABLE casting_scores
    DROP CONSTRAINT IF EXISTS casting_scores_score_check;

ALTER TABLE casting_scores
    ADD CONSTRAINT casting_scores_score_check CHECK (score BETWEEN 1 AND 10 AND score * 2 = trunc(score * 2));

CREATE TABLE IF NOT EXISTS casting_dataset (
    singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    participants JSONB NOT NULL,
    uploaded_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
