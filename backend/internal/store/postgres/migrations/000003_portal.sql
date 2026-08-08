ALTER TABLE users ADD COLUMN IF NOT EXISTS last_login_at timestamptz;

INSERT INTO system_settings (key, value) VALUES
    ('social_twitch', '"https://www.twitch.tv/ravshann"'),
    ('social_youtube', '""'),
    ('social_telegram', '""'),
    ('social_vk', '""')
ON CONFLICT (key) DO NOTHING;
