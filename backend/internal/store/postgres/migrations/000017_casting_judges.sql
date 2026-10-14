CREATE TABLE IF NOT EXISTS casting_judges (
    position SMALLINT PRIMARY KEY CHECK (position BETWEEN 1 AND 20),
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO casting_judges(position, name)
VALUES (1, 'Судья 1')
ON CONFLICT(position) DO NOTHING;

ALTER TABLE casting_scores
    ADD COLUMN IF NOT EXISTS judge_id SMALLINT NOT NULL DEFAULT 1;

DROP INDEX IF EXISTS casting_scores_participant_unique_idx;

DO $$
DECLARE
    primary_key_name TEXT;
BEGIN
    SELECT conname INTO primary_key_name
    FROM pg_constraint
    WHERE conrelid = 'casting_scores'::regclass AND contype = 'p';

    IF primary_key_name IS NOT NULL AND pg_get_constraintdef(
        (SELECT oid FROM pg_constraint WHERE conrelid = 'casting_scores'::regclass AND conname = primary_key_name)
    ) <> 'PRIMARY KEY (participant_id, judge_id)' THEN
        EXECUTE format('ALTER TABLE casting_scores DROP CONSTRAINT %I', primary_key_name);
        primary_key_name := NULL;
    END IF;

    IF primary_key_name IS NULL THEN
        ALTER TABLE casting_scores
            ADD CONSTRAINT casting_scores_pkey PRIMARY KEY (participant_id, judge_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'casting_scores'::regclass AND conname = 'casting_scores_judge_id_fkey'
    ) THEN
        ALTER TABLE casting_scores
            ADD CONSTRAINT casting_scores_judge_id_fkey
            FOREIGN KEY (judge_id) REFERENCES casting_judges(position) ON DELETE CASCADE;
    END IF;
END $$;
