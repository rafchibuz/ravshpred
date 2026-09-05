-- Optional link from an in-app notification to the news item that caused it.
ALTER TABLE notifications
    ADD COLUMN IF NOT EXISTS news_post_id uuid REFERENCES news_posts(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS notifications_news_post_idx
    ON notifications (news_post_id);

-- Retrying a publish request must not create duplicate news notifications.
CREATE UNIQUE INDEX IF NOT EXISTS notifications_news_once_idx
    ON notifications (user_id, news_post_id)
    WHERE type = 'news' AND news_post_id IS NOT NULL;
