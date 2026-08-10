CREATE TABLE IF NOT EXISTS twitch_rating_streams (
    id text NOT NULL,
    channel_login text NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    title text NOT NULL DEFAULT '',
    game_name text NOT NULL DEFAULT '',
    observed_live boolean NOT NULL DEFAULT true,
    PRIMARY KEY (channel_login, id)
);

CREATE TABLE IF NOT EXISTS twitch_rating_messages (
    id text PRIMARY KEY,
    channel_login text NOT NULL,
    user_id text NOT NULL,
    login text NOT NULL,
    display_name text NOT NULL DEFAULT '',
    avatar_url text NOT NULL DEFAULT '',
    sent_at timestamptz NOT NULL,
    stream_id text,
    role text NOT NULL DEFAULT 'viewer' CHECK (role IN ('viewer','vip','moderator','broadcaster')),
    text_hash text NOT NULL DEFAULT '',
    text_length integer NOT NULL DEFAULT 0,
    is_command boolean NOT NULL DEFAULT false,
    is_duplicate boolean NOT NULL DEFAULT false,
    is_emote_only boolean NOT NULL DEFAULT false,
    is_reply boolean NOT NULL DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_twitch_rating_streams_channel_time
    ON twitch_rating_streams(channel_login, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_twitch_rating_messages_channel_time
    ON twitch_rating_messages(channel_login, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_twitch_rating_messages_user_time
    ON twitch_rating_messages(user_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_twitch_rating_messages_stream
    ON twitch_rating_messages(channel_login, stream_id);

CREATE TABLE IF NOT EXISTS twitch_rating_state (
    channel_login text PRIMARY KEY,
    collector_status text NOT NULL DEFAULT 'not_configured',
    collector_user text NOT NULL DEFAULT '',
    last_event_at timestamptz,
    last_error text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS twitch_rating_credentials (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    access_token text NOT NULL,
    refresh_token text NOT NULL,
    collector_user_id text NOT NULL,
    collector_login text NOT NULL,
    expires_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO twitch_rating_state(channel_login)
VALUES ('ravshann'), ('ravshanbtw')
ON CONFLICT (channel_login) DO NOTHING;
