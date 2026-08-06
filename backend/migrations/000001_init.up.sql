CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('user', 'moderator', 'owner');
CREATE TYPE submission_status AS ENUM ('pending', 'approved', 'rejected', 'changes_requested', 'hidden');
CREATE TYPE moderation_action AS ENUM ('approve', 'reject', 'request_changes', 'hide', 'restore', 'delete', 'category_change', 'watched_on', 'watched_off');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    twitch_id text UNIQUE,
    twitch_login text,
    display_name text NOT NULL,
    avatar_url text NOT NULL DEFAULT '',
    role user_role NOT NULL DEFAULT 'user',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((role = 'owner') OR twitch_id IS NOT NULL)
);

CREATE UNIQUE INDEX one_active_owner_idx ON users ((role)) WHERE role = 'owner' AND deleted_at IS NULL;
CREATE INDEX users_twitch_login_idx ON users (lower(twitch_login)) WHERE deleted_at IS NULL;

CREATE TABLE owner_credentials (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    webauthn_user_handle bytea NOT NULL UNIQUE,
    credential_id bytea NOT NULL UNIQUE,
    public_key bytea NOT NULL,
    sign_count bigint NOT NULL DEFAULT 0,
    transports text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);

CREATE TABLE owner_recovery_codes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash bytea NOT NULL UNIQUE,
    used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token_hash bytea NOT NULL,
    ip_prefix inet,
    user_agent_hash bytea,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_active_idx ON sessions (token_hash, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE oauth_states (
    state_hash bytea PRIMARY KEY,
    code_verifier_hash bytea NOT NULL,
    return_to text NOT NULL DEFAULT '/',
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE,
    name text NOT NULL UNIQUE,
    is_system boolean NOT NULL DEFAULT false,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO categories (slug, name, is_system, sort_order) VALUES
    ('uncategorized', 'Без категории', true, 0),
    ('funny', 'Смешное', false, 10),
    ('trailers', 'Трейлеры', false, 20),
    ('movies-series', 'Фильмы и сериалы', false, 30),
    ('exposes', 'Разоблачения', false, 40);

CREATE TABLE submissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    youtube_id varchar(11) NOT NULL,
    youtube_url text NOT NULL,
    title text NOT NULL,
    channel_title text NOT NULL,
    thumbnail_url text NOT NULL,
    duration_seconds integer NOT NULL DEFAULT 0 CHECK (duration_seconds >= 0),
    view_count bigint NOT NULL DEFAULT 0 CHECK (view_count >= 0),
    youtube_like_count bigint NOT NULL DEFAULT 0 CHECK (youtube_like_count >= 0),
    metadata_fetched_at timestamptz,
    author_id uuid NOT NULL REFERENCES users(id),
    category_id uuid NOT NULL REFERENCES categories(id),
    status submission_status NOT NULL DEFAULT 'pending',
    submitter_comment varchar(500) NOT NULL DEFAULT '',
    moderator_comment varchar(500) NOT NULL DEFAULT '',
    version integer NOT NULL DEFAULT 1,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX submissions_youtube_active_idx ON submissions (youtube_id) WHERE deleted_at IS NULL;
CREATE INDEX submissions_feed_idx ON submissions (created_at DESC, id DESC) WHERE status = 'approved' AND deleted_at IS NULL;
CREATE INDEX submissions_moderation_idx ON submissions (status, created_at ASC, id ASC) WHERE deleted_at IS NULL;
CREATE INDEX submissions_author_idx ON submissions (author_id, created_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE votes (
    submission_id uuid NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    value smallint NOT NULL CHECK (value IN (-1, 1)),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (submission_id, user_id)
);
CREATE INDEX votes_rating_idx ON votes (submission_id, value);

CREATE TABLE streamer_marks (
    submission_id uuid PRIMARY KEY REFERENCES submissions(id) ON DELETE CASCADE,
    watched boolean NOT NULL DEFAULT true,
    marked_by uuid NOT NULL REFERENCES users(id),
    marked_at timestamptz NOT NULL DEFAULT now(),
    CHECK (watched)
);

CREATE TABLE moderation_actions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    submission_id uuid NOT NULL REFERENCES submissions(id) ON DELETE CASCADE,
    moderator_id uuid NOT NULL REFERENCES users(id),
    action moderation_action NOT NULL,
    from_status submission_status,
    to_status submission_status,
    reason_code text NOT NULL DEFAULT '',
    comment varchar(500) NOT NULL DEFAULT '',
    submission_version integer NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX moderation_actions_submission_idx ON moderation_actions (submission_id, created_at DESC);

CREATE TABLE notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type text NOT NULL,
    title text NOT NULL,
    body text NOT NULL,
    submission_id uuid REFERENCES submissions(id) ON DELETE SET NULL,
    read_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);

CREATE TABLE audit_log (
    id bigserial PRIMARY KEY,
    actor_id uuid REFERENCES users(id) ON DELETE SET NULL,
    action text NOT NULL,
    target_type text NOT NULL,
    target_id text NOT NULL,
    request_id uuid,
    metadata jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_created_idx ON audit_log (created_at DESC);

CREATE TABLE system_settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    updated_by uuid REFERENCES users(id),
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO system_settings (key, value) VALUES
    ('submission_daily_limit', '3'),
    ('submission_comment_limit', '500'),
    ('public_feed_enabled', 'true');

