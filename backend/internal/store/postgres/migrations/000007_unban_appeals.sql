CREATE TABLE IF NOT EXISTS unban_appeals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id uuid NOT NULL REFERENCES users(id),
    platform text NOT NULL CHECK (platform IN ('twitch','telegram')),
    community text NOT NULL CHECK (community IN ('ravshann','ravshanbtw','ravshann_telegram')),
    banned_username text NOT NULL,
    ban_reason text NOT NULL,
    statement text NOT NULL,
    position text NOT NULL CHECK (position IN ('admit','mistake','unsure')),
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','in_review','needs_info','approved','rejected','withdrawn','duplicate')),
    moderator_comment text NOT NULL DEFAULT '',
    internal_note text NOT NULL DEFAULT '',
    moderator_id uuid REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    resolved_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_unban_appeals_author_created ON unban_appeals(author_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_unban_appeals_queue ON unban_appeals(status, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_unban_appeals_one_open
    ON unban_appeals(author_id, platform, community)
    WHERE status IN ('pending','in_review','needs_info');
