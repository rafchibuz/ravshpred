CREATE TABLE IF NOT EXISTS twitch_clips (
    id text PRIMARY KEY,
    url text NOT NULL DEFAULT '',
    embed_url text NOT NULL DEFAULT '',
    broadcaster_name text NOT NULL DEFAULT '',
    creator_name text NOT NULL DEFAULT '',
    video_id text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    view_count integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL,
    thumbnail_url text NOT NULL DEFAULT '',
    duration double precision NOT NULL DEFAULT 0,
    vod_offset integer,
    fetched_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS twitch_clips_channel_created_idx
    ON twitch_clips (lower(broadcaster_name), created_at DESC);
CREATE INDEX IF NOT EXISTS twitch_clips_video_idx
    ON twitch_clips (video_id, created_at);

CREATE TABLE IF NOT EXISTS twitch_videos (
    id text PRIMARY KEY,
    stream_id text NOT NULL DEFAULT '',
    user_id text NOT NULL DEFAULT '',
    user_login text NOT NULL DEFAULT '',
    user_name text NOT NULL DEFAULT '',
    title text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL,
    published_at timestamptz NOT NULL,
    url text NOT NULL DEFAULT '',
    thumbnail_url text NOT NULL DEFAULT '',
    view_count integer NOT NULL DEFAULT 0,
    language text NOT NULL DEFAULT '',
    type text NOT NULL DEFAULT '',
    duration text NOT NULL DEFAULT '',
    fetched_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS twitch_videos_channel_created_idx
    ON twitch_videos (lower(user_login), created_at DESC);

CREATE TABLE IF NOT EXISTS twitch_cache_state (
    cache_key text PRIMARY KEY,
    item_count integer NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now()
);
