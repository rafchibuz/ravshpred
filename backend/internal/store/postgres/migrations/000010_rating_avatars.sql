-- Profile images for viewers shown in the rating. This table contains only
-- users that have appeared in a published top-100 snapshot.
CREATE TABLE IF NOT EXISTS twitch_rating_profiles (
    user_id text PRIMARY KEY,
    avatar_url text NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL DEFAULT now()
);

