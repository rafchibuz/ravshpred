UPDATE ravshtok_media
SET status = 'pending',
    attempts = 0,
    last_error = '',
    next_attempt_at = now(),
    lease_until = NULL,
    updated_at = now()
WHERE platform = 'instagram'
  AND status = 'failed'
  AND last_error ILIKE '%downloader did not produce a media file%';
