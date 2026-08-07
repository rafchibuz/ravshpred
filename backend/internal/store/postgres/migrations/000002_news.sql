CREATE TABLE IF NOT EXISTS news_posts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id uuid NOT NULL REFERENCES users(id),
    title varchar(160) NOT NULL CHECK (char_length(title) BETWEEN 1 AND 160),
    body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS news_posts_created_idx
    ON news_posts (created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS news_comments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id uuid NOT NULL REFERENCES news_posts(id) ON DELETE CASCADE,
    author_id uuid NOT NULL REFERENCES users(id),
    body varchar(1000) NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS news_comments_post_idx
    ON news_comments (post_id, created_at ASC, id ASC);
