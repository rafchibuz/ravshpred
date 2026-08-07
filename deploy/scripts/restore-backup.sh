#!/bin/sh
set -eu

backup_file="${1:-}"
target_database_url="${TARGET_DATABASE_URL:-}"

if [ -z "$backup_file" ] || [ ! -f "$backup_file" ]; then
  echo "usage: TARGET_DATABASE_URL=... RESTORE_CONFIRM=RESTORE restore-backup.sh /backups/...dump" >&2
  exit 2
fi
if [ -z "$target_database_url" ]; then
  echo "TARGET_DATABASE_URL is required" >&2
  exit 3
fi
if [ "${RESTORE_CONFIRM:-}" != "RESTORE" ]; then
  echo "restore cancelled: set RESTORE_CONFIRM=RESTORE explicitly" >&2
  exit 4
fi

"/usr/local/bin/verify-backup.sh" "$backup_file"
pg_restore \
  --dbname="$target_database_url" \
  --clean \
  --if-exists \
  --no-owner \
  --no-privileges \
  "$backup_file"
printf 'restore_completed file=%s\n' "$backup_file"
