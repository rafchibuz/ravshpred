CREATE TABLE IF NOT EXISTS casting_winners (
    participant_id SMALLINT PRIMARY KEY CHECK (participant_id > 0),
    selected_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
