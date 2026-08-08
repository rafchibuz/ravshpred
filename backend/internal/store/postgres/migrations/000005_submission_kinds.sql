ALTER TABLE submissions ADD COLUMN IF NOT EXISTS content_kind text NOT NULL DEFAULT 'video';
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS source_type text NOT NULL DEFAULT 'youtube';
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS source_url text NOT NULL DEFAULT '';

UPDATE submissions SET source_url = youtube_url WHERE source_url = '';

ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_content_kind_check;
ALTER TABLE submissions ADD CONSTRAINT submissions_content_kind_check CHECK (content_kind IN ('video', 'stream_idea'));
ALTER TABLE submissions DROP CONSTRAINT IF EXISTS submissions_source_type_check;
ALTER TABLE submissions ADD CONSTRAINT submissions_source_type_check CHECK (source_type IN ('youtube', 'short_video', 'external', 'idea'));

DROP INDEX IF EXISTS submissions_youtube_active_idx;
CREATE UNIQUE INDEX submissions_youtube_active_idx
    ON submissions (youtube_id)
    WHERE deleted_at IS NULL AND youtube_id <> '';

CREATE UNIQUE INDEX IF NOT EXISTS submissions_source_active_idx
    ON submissions (source_url)
    WHERE deleted_at IS NULL AND source_url <> '' AND source_type <> 'youtube';

INSERT INTO categories (slug, name, is_system, sort_order)
VALUES ('stream-ideas', 'Идеи для стрима', true, 90)
ON CONFLICT (slug) DO NOTHING;
