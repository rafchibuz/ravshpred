import json
import logging
import math
import os
import shutil
import signal
import subprocess
import tempfile
import time
import threading
from pathlib import Path
from urllib.parse import urlparse

import psycopg
from psycopg.rows import dict_row


DATABASE_URL = os.environ["DATABASE_URL"]
MEDIA_ROOT = Path(os.getenv("RAVSHTOK_MEDIA_ROOT", "/media"))
OUTPUT_DIR = MEDIA_ROOT / "ravshtok"
POLL_SECONDS = max(2, int(os.getenv("RAVSHTOK_POLL_SECONDS", "5")))
MAX_DURATION = min(90, max(15, int(os.getenv("RAVSHTOK_MAX_DURATION_SECONDS", "90"))))
MAX_SIZE_MB = max(20, int(os.getenv("RAVSHTOK_MAX_SIZE_MB", "150")))
DOWNLOAD_TIMEOUT = max(30, int(os.getenv("RAVSHTOK_DOWNLOAD_TIMEOUT_SECONDS", "240")))
TRANSCODE_TIMEOUT = max(120, int(os.getenv("RAVSHTOK_TRANSCODE_TIMEOUT_SECONDS", "900")))
FFMPEG_PRESET = os.getenv("RAVSHTOK_FFMPEG_PRESET", "veryfast").strip().lower()
if FFMPEG_PRESET not in {"ultrafast", "superfast", "veryfast", "faster", "fast", "medium"}:
    FFMPEG_PRESET = "veryfast"
MAX_ATTEMPTS = max(1, int(os.getenv("RAVSHTOK_MAX_ATTEMPTS", "5")))
INITIAL_RETENTION_DAYS = max(30, int(os.getenv("RAVSHTOK_INITIAL_RETENTION_DAYS", "90")))
DIAGNOSTICS_SECONDS = max(60, int(os.getenv("RAVSHTOK_DIAGNOSTICS_SECONDS", "300")))

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger("ravshtok-worker")
running = True
stop_event = threading.Event()


def stop_worker(_signum, _frame):
    global running
    running = False
    stop_event.set()


signal.signal(signal.SIGTERM, stop_worker)
signal.signal(signal.SIGINT, stop_worker)


def allowed_source(source_url: str) -> bool:
    try:
        parsed = urlparse(source_url)
    except ValueError:
        return False
    host = (parsed.hostname or "").lower()
    if parsed.scheme not in {"http", "https"}:
        return False
    if host in {"tiktok.com", "www.tiktok.com", "m.tiktok.com"}:
        return parsed.path.lower().startswith("/@") and "/video/" in parsed.path.lower()
    if host in {"vm.tiktok.com", "vt.tiktok.com"}:
        return parsed.path not in {"", "/"}
    if host in {"instagram.com", "www.instagram.com"}:
        path = parsed.path.lower()
        return path.startswith("/reel/") or path.startswith("/reels/")
    return False


def run(command: list[str], timeout: int = DOWNLOAD_TIMEOUT) -> subprocess.CompletedProcess:
    return subprocess.run(command, check=True, capture_output=True, text=True, timeout=timeout)


def claim_job(connection):
    with connection.transaction(), connection.cursor(row_factory=dict_row) as cursor:
        cursor.execute(
            """
            WITH candidate AS (
                SELECT rm.submission_id
                FROM ravshtok_media rm
                JOIN submissions s ON s.id=rm.submission_id
                WHERE s.status='approved' AND s.deleted_at IS NULL
                  AND rm.attempts < %s
                  AND (
                    (rm.status IN ('pending','failed') AND rm.next_attempt_at <= now())
                    OR (rm.status='processing' AND rm.lease_until < now())
                  )
                ORDER BY rm.next_attempt_at,rm.created_at
                FOR UPDATE SKIP LOCKED
                LIMIT 1
            )
            UPDATE ravshtok_media rm
            SET status='processing',attempts=attempts+1,lease_until=now()+interval '30 minutes',
                last_error='',updated_at=now()
            FROM candidate c,submissions s
            WHERE rm.submission_id=c.submission_id AND s.id=rm.submission_id
            RETURNING rm.submission_id::text,s.source_url,rm.platform,rm.attempts
            """,
            (MAX_ATTEMPTS,),
        )
        return cursor.fetchone()


def probe_video(path: Path) -> dict:
    result = run([
        "ffprobe", "-v", "error",
        "-show_entries", "stream=codec_type,codec_name,width,height,pix_fmt:format=duration",
        "-of", "json", str(path),
    ], timeout=30)
    payload = json.loads(result.stdout)
    streams = payload.get("streams") or []
    stream = next((item for item in streams if item.get("codec_type") == "video"), {})
    audio = next((item for item in streams if item.get("codec_type") == "audio"), None)
    duration = int(math.ceil(float((payload.get("format") or {}).get("duration") or 0)))
    return {
        "duration": duration,
        "width": int(stream.get("width") or 0),
        "height": int(stream.get("height") or 0),
        "video_codec": str(stream.get("codec_name") or "").lower(),
        "audio_codec": str((audio or {}).get("codec_name") or "").lower(),
        "pixel_format": str(stream.get("pix_fmt") or "").lower(),
    }


def duration_error() -> ValueError:
    return ValueError("Ролик длиннее 1:30. Максимальная длительность RavshTOK — 1 минута 30 секунд")


def can_remux(metadata: dict) -> bool:
    return (
        metadata.get("video_codec") == "h264"
        and metadata.get("audio_codec") in {"", "aac"}
        and metadata.get("pixel_format") in {"yuv420p", "yuvj420p"}
        and 0 < int(metadata.get("width") or 0) <= 1080
        and 0 < int(metadata.get("height") or 0) <= 1920
    )


def prepare_media(job: dict) -> tuple[Path, Path, dict]:
    source_url = job["source_url"]
    if not allowed_source(source_url):
        raise ValueError("source URL is not an allowed TikTok or Instagram Reel URL")

    work_dir = Path(tempfile.mkdtemp(prefix="ravshtok-"))
    try:
        output_template = str(work_dir / "source.%(ext)s")
        try:
            download = run([
                "yt-dlp", "--no-playlist", "--no-progress",
                "--restrict-filenames", "--max-filesize", f"{MAX_SIZE_MB}M",
                # Instagram often omits the duration until the file has been downloaded.
                # The optional comparison keeps such Reels; ffprobe validates the real
                # duration below before anything is published.
                "--match-filter", f"duration <=? {MAX_DURATION}",
                "-f", "bv*+ba/b", "-o", output_template, source_url,
            ])
        except subprocess.CalledProcessError as error:
            details = (error.stderr or error.stdout or "").strip().splitlines()
            detail = details[-1][:900] if details else "yt-dlp завершился с ошибкой"
            if "does not pass filter" in detail.lower() and "duration" in detail.lower():
                raise duration_error() from error
            raise RuntimeError(detail) from error
        media_extensions = {".mp4", ".webm", ".mkv", ".mov", ".m4v"}
        sources = [
            path for path in work_dir.rglob("source.*")
            if path.is_file() and path.suffix.lower() in media_extensions
        ]
        if not sources:
            details = (download.stderr or download.stdout or "").strip().splitlines()
            detail = details[-1][:700] if details else "Instagram или TikTok не вернул медиафайл"
            raise RuntimeError(f"Не удалось получить видео: {detail}")
        source = max(sources, key=lambda path: path.stat().st_size)

        source_meta = probe_video(source)
        if source_meta["duration"] <= 0:
            raise ValueError("Не удалось определить длительность ролика")
        if source_meta["duration"] > MAX_DURATION:
            raise duration_error()
        if source.stat().st_size > MAX_SIZE_MB * 1024 * 1024:
            raise ValueError(f"source video is larger than {MAX_SIZE_MB} MB")

        normalized = work_dir / "video.mp4"
        poster = work_dir / "poster.webp"
        try:
            if can_remux(source_meta):
                logger.info("compatible source, remuxing id=%s", job["submission_id"])
                run([
                    "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(source),
                    "-map", "0:v:0", "-map", "0:a:0?", "-c", "copy",
                    "-movflags", "+faststart", str(normalized),
                ], timeout=TRANSCODE_TIMEOUT)
            else:
                logger.info(
                    "transcoding source id=%s codec=%s dimensions=%sx%s preset=%s",
                    job["submission_id"], source_meta["video_codec"], source_meta["width"],
                    source_meta["height"], FFMPEG_PRESET,
                )
                run([
                    "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", str(source),
                    "-map", "0:v:0", "-map", "0:a:0?",
                    "-vf", "scale='min(1080,iw)':'min(1920,ih)':force_original_aspect_ratio=decrease:force_divisible_by=2",
                    "-c:v", "libx264", "-preset", FFMPEG_PRESET, "-crf", "22", "-pix_fmt", "yuv420p",
                    "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", str(normalized),
                ], timeout=TRANSCODE_TIMEOUT)
        except subprocess.TimeoutExpired as error:
            raise RuntimeError(
                f"FFmpeg не успел обработать ролик за {TRANSCODE_TIMEOUT} секунд"
            ) from error
        run([
            "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-ss", "0.2",
            "-i", str(normalized), "-frames:v", "1", "-vf", "scale=540:-2", str(poster),
        ], timeout=45)
        metadata = probe_video(normalized)
        metadata["size"] = normalized.stat().st_size
        return normalized, poster, metadata
    except Exception:
        shutil.rmtree(work_dir, ignore_errors=True)
        raise


def place_media(source: Path, destination: Path):
    """Copy from the tmpfs into the Docker volume, then publish atomically."""
    destination.parent.mkdir(parents=True, exist_ok=True)
    partial = destination.with_name(destination.name + ".part")
    partial.unlink(missing_ok=True)
    try:
        shutil.copy2(source, partial)
        os.replace(partial, destination)
        source.unlink(missing_ok=True)
    finally:
        partial.unlink(missing_ok=True)


def finish_job(connection, job: dict, normalized: Path, poster: Path, metadata: dict):
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    video_name = f"{job['submission_id']}.mp4"
    poster_name = f"{job['submission_id']}.webp"
    final_video = OUTPUT_DIR / video_name
    final_poster = OUTPUT_DIR / poster_name
    with connection.transaction(), connection.cursor() as cursor:
        # Serialize publication with deletion/moderation. A finished download must
        # never bring a deleted submission back to life.
        cursor.execute("SELECT status,deleted_at FROM submissions WHERE id::text=%s FOR UPDATE", (job["submission_id"],))
        submission = cursor.fetchone()
        if not submission or submission[0] != "approved" or submission[1] is not None:
            cursor.execute("UPDATE ravshtok_media SET status=CASE WHEN %s THEN 'expired' ELSE 'pending' END,lease_until=NULL,updated_at=now() WHERE submission_id::text=%s",
                           (not submission or submission[1] is not None, job["submission_id"]))
            shutil.rmtree(normalized.parent, ignore_errors=True)
            logger.info("media publication cancelled id=%s", job["submission_id"])
            return False
        place_media(normalized, final_video)
        place_media(poster, final_poster)
        shutil.rmtree(normalized.parent, ignore_errors=True)
        cursor.execute(
            """
            UPDATE ravshtok_media
            SET status='ready',media_path=%s,poster_path=%s,duration_seconds=%s,
                width=%s,height=%s,size_bytes=%s,ready_at=now(),
                expires_at=now()+(%s * interval '1 day'),lease_until=NULL,last_error='',updated_at=now()
            WHERE submission_id::text=%s
            """,
            (
                f"ravshtok/{video_name}", f"ravshtok/{poster_name}", metadata["duration"],
                metadata["width"], metadata["height"], metadata["size"],
                INITIAL_RETENTION_DAYS, job["submission_id"],
            ),
        )
    return True


def fail_job(connection, job: dict, error: Exception):
    delay_seconds = min(3600, 30 * (2 ** max(0, int(job["attempts"]) - 1)))
    message = str(error).strip()[:1000] or error.__class__.__name__
    with connection.transaction(), connection.cursor() as cursor:
        cursor.execute(
            """
            UPDATE ravshtok_media
            SET status='failed',last_error=%s,next_attempt_at=now()+(%s * interval '1 second'),
                lease_until=NULL,updated_at=now()
            WHERE submission_id::text=%s
            """,
            (message, delay_seconds, job["submission_id"]),
        )
    logger.warning("media preparation failed id=%s attempt=%s error=%s", job["submission_id"], job["attempts"], message)


def remove_media_file(relative):
    if not relative:
        return
    path = (MEDIA_ROOT / relative).resolve()
    if path.parent != OUTPUT_DIR.resolve() or path.suffix not in {".mp4", ".webp"}:
        raise ValueError("invalid media cleanup path")
    path.unlink(missing_ok=True)


def cleanup_expired(connection):
    removed = 0
    with connection.transaction(), connection.cursor(row_factory=dict_row) as cursor:
        cursor.execute(
            """
            SELECT rm.submission_id::text,rm.media_path,rm.poster_path
            FROM ravshtok_media rm JOIN submissions s ON s.id=rm.submission_id
            WHERE (s.deleted_at IS NOT NULL AND (rm.status <> 'expired' OR rm.media_path <> '' OR rm.poster_path <> ''))
               OR (rm.status='ready' AND rm.expires_at IS NOT NULL AND rm.expires_at <= now())
            ORDER BY s.deleted_at NULLS LAST,rm.expires_at
            FOR UPDATE OF rm SKIP LOCKED
            LIMIT 50
            """
        )
        rows = cursor.fetchall()
        for row in rows:
            try:
                for relative in (row["media_path"], row["poster_path"]):
                    remove_media_file(relative)
            except (ValueError, OSError):
                logger.exception("media cleanup failed id=%s; will retry", row["submission_id"])
                continue
            cursor.execute(
                """
                UPDATE ravshtok_media
                SET status='expired',media_path='',poster_path='',size_bytes=0,lease_until=NULL,updated_at=now()
                WHERE submission_id::text=%s
                """,
                (row["submission_id"],),
            )
            removed += 1
            logger.info("media files removed id=%s", row["submission_id"])
    with connection.transaction(), connection.cursor() as cursor:
        cursor.execute("SELECT submission_id::text FROM ravshtok_media WHERE media_path <> '' OR poster_path <> ''")
        known_ids = {row[0] for row in cursor.fetchall()}
    for video_path in OUTPUT_DIR.iterdir():
        if video_path.suffix not in {".mp4", ".webp", ".part"} or video_path.stem in known_ids:
            continue
        try:
            # A publisher may have copied a file but not committed its DB row yet.
            if time.time() - video_path.stat().st_mtime < 3600:
                continue
            video_path.unlink(missing_ok=True)
            removed += 1
        except OSError:
            logger.warning("could not remove orphaned media id=%s", video_path.stem)
    return removed


def cleanup_loop():
    # Separate connection/thread: slow downloads must not delay deleted-file cleanup.
    while not stop_event.is_set():
        try:
            with psycopg.connect(DATABASE_URL, connect_timeout=10) as connection:
                removed = cleanup_expired(connection)
                if removed:
                    logger.info("media cleanup completed count=%s", removed)
        except Exception:
            logger.exception("media cleanup failed; will retry")
        stop_event.wait(30)


def log_diagnostics(connection):
    totals = {status: 0 for status in ("pending", "processing", "ready", "failed", "expired")}
    storage_bytes = 0
    with connection.transaction(), connection.cursor() as cursor:
        cursor.execute("SELECT status,count(*),COALESCE(sum(size_bytes),0) FROM ravshtok_media GROUP BY status")
        for status, count, size_bytes in cursor.fetchall():
            totals[status] = count
            storage_bytes += int(size_bytes or 0)
    logger.info(
        "worker status pending=%s processing=%s ready=%s failed=%s expired=%s storage_bytes=%s",
        totals["pending"], totals["processing"], totals["ready"], totals["failed"], totals["expired"], storage_bytes,
    )


def connect():
    while running:
        try:
            return psycopg.connect(DATABASE_URL, autocommit=False)
        except Exception as error:
            logger.warning("database unavailable: %s", error)
            time.sleep(POLL_SECONDS)
    return None


def main():
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    last_diagnostics = 0.0
    logger.info(
        "worker started poll_seconds=%s max_duration=%s max_size_mb=%s download_timeout=%s "
        "transcode_timeout=%s ffmpeg_preset=%s max_attempts=%s retention_days=%s",
        POLL_SECONDS, MAX_DURATION, MAX_SIZE_MB, DOWNLOAD_TIMEOUT, TRANSCODE_TIMEOUT,
        FFMPEG_PRESET, MAX_ATTEMPTS, INITIAL_RETENTION_DAYS,
    )
    cleaner = threading.Thread(target=cleanup_loop, name="media-cleanup", daemon=True)
    cleaner.start()
    connection = connect()
    while running and connection is not None:
        try:
            if connection.closed:
                connection = connect()
                continue
            now = time.monotonic()
            if now - last_diagnostics > DIAGNOSTICS_SECONDS:
                log_diagnostics(connection)
                last_diagnostics = now
            job = claim_job(connection)
            if not job:
                time.sleep(POLL_SECONDS)
                continue
            logger.info("preparing media id=%s platform=%s", job["submission_id"], job["platform"])
            job_started = time.monotonic()
            try:
                normalized, poster, metadata = prepare_media(job)
                if not finish_job(connection, job, normalized, poster, metadata):
                    continue
                logger.info(
                    "media ready id=%s duration=%s size=%s processing_ms=%s",
                    job["submission_id"], metadata["duration"], metadata["size"], int((time.monotonic() - job_started) * 1000),
                )
            except Exception as error:
                fail_job(connection, job, error)
        except (psycopg.Error, OSError) as error:
            logger.warning("worker loop error: %s", error)
            try:
                connection.close()
            except Exception:
                pass
            time.sleep(POLL_SECONDS)
            connection = connect()
    if connection is not None:
        connection.close()
    logger.info("worker stopped")


if __name__ == "__main__":
    main()
