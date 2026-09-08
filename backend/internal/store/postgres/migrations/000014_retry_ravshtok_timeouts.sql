UPDATE ravshtok_media
SET status = 'pending',
    attempts = 0,
    last_error = '',
    next_attempt_at = now(),
    lease_until = NULL,
    updated_at = now()
WHERE status = 'failed'
  AND last_error ILIKE '%ffmpeg%'
  AND last_error ILIKE '%timed out after 240 seconds%';
