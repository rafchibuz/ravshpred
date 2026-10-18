CREATE INDEX IF NOT EXISTS twitch_rating_snapshot_entries_login_idx
    ON twitch_rating_snapshot_entries (snapshot_key, lower(payload->>'login'));
