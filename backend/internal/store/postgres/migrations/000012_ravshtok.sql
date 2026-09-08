CREATE TABLE IF NOT EXISTS ravshtok_media (
    submission_id uuid PRIMARY KEY REFERENCES submissions(id) ON DELETE CASCADE,
    platform text NOT NULL CHECK (platform IN ('tiktok', 'instagram')),
    external_id text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'ready', 'failed', 'expired')),
    media_path text NOT NULL DEFAULT '',
    poster_path text NOT NULL DEFAULT '',
    duration_seconds integer NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    width integer NOT NULL DEFAULT 0 CHECK (width >= 0),
    height integer NOT NULL DEFAULT 0 CHECK (height >= 0),
    size_bytes bigint NOT NULL DEFAULT 0 CHECK (size_bytes >= 0),
    attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error text NOT NULL DEFAULT '',
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    lease_until timestamptz,
    ready_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ravshtok_media_queue_idx
    ON ravshtok_media (status, next_attempt_at, created_at)
    WHERE status IN ('pending', 'failed');

CREATE INDEX IF NOT EXISTS ravshtok_media_expiry_idx
    ON ravshtok_media (expires_at)
    WHERE status = 'ready' AND expires_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS ravshtok_views (
    submission_id uuid NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    first_viewed_at timestamptz NOT NULL DEFAULT now(),
    last_viewed_at timestamptz NOT NULL DEFAULT now(),
    view_count integer NOT NULL DEFAULT 1 CHECK (view_count >= 1),
    PRIMARY KEY (submission_id, user_id)
);

CREATE INDEX IF NOT EXISTS ravshtok_views_user_idx
    ON ravshtok_views (user_id, last_viewed_at DESC);

INSERT INTO system_settings(key, value)
VALUES ('ravshtok_daily_limit', '10'::jsonb)
ON CONFLICT (key) DO NOTHING;

INSERT INTO ravshtok_media (submission_id, platform)
SELECT s.id,
       CASE
           WHEN lower(s.source_url) LIKE '%instagram.com/%' THEN 'instagram'
           ELSE 'tiktok'
       END
FROM submissions s
WHERE s.source_type = 'short_video' AND s.deleted_at IS NULL
ON CONFLICT (submission_id) DO NOTHING;
