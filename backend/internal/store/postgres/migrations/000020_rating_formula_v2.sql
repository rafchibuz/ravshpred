-- Invalidate only derived rating snapshots once. Source streams and messages stay intact.
CREATE TABLE IF NOT EXISTS twitch_rating_formula_versions (
    version integer PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
    PERFORM pg_advisory_xact_lock(913573);
    IF NOT EXISTS (SELECT 1 FROM twitch_rating_formula_versions WHERE version = 2) THEN
        DELETE FROM twitch_rating_snapshots;
        INSERT INTO twitch_rating_formula_versions(version) VALUES (2);
    END IF;
END $$;
