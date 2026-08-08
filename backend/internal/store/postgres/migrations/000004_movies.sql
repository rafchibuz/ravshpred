ALTER TABLE submissions ADD COLUMN IF NOT EXISTS kinopoisk_url text NOT NULL DEFAULT '';
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS movie_title text NOT NULL DEFAULT '';
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS movie_year integer;
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS movie_studio text NOT NULL DEFAULT '';
ALTER TABLE submissions ADD COLUMN IF NOT EXISTS movie_rating numeric(3,1);

