-- Derived data only. Original messages and existing scores are not modified.
CREATE TABLE IF NOT EXISTS twitch_rating_snapshots (
    key text PRIMARY KEY,
    generated_at timestamptz NOT NULL,
    payload jsonb NOT NULL
);
CREATE TABLE IF NOT EXISTS twitch_rating_snapshot_entries (
    snapshot_key text NOT NULL REFERENCES twitch_rating_snapshots(key) ON DELETE CASCADE,
    user_id text NOT NULL,
    payload jsonb NOT NULL,
    PRIMARY KEY(snapshot_key, user_id)
);
