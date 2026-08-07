#!/bin/sh
set -eu

backup_root="${BACKUP_DIR:-/backups}"
daily_keep="${BACKUP_DAILY_RETENTION:-7}"
weekly_keep="${BACKUP_WEEKLY_RETENTION:-4}"
interval="${BACKUP_INTERVAL_SECONDS:-86400}"

mkdir -p "$backup_root/daily" "$backup_root/weekly"

rotate_backups() {
  directory="$1"
  keep="$2"
  ls -1t "$directory"/*.dump 2>/dev/null | awk "NR > $keep" | while IFS= read -r old_backup; do
    rm -f -- "$old_backup" "$old_backup.sha256"
  done
}

create_backup() {
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  temporary="$backup_root/.ravshann-$timestamp.dump.tmp"
  daily="$backup_root/daily/ravshann-$timestamp.dump"

  trap 'rm -f -- "$temporary"' EXIT INT TERM
  pg_dump \
    --dbname="$DATABASE_URL" \
    --format=custom \
    --compress=6 \
    --no-owner \
    --no-privileges \
    --file="$temporary"
  pg_restore --list "$temporary" >/dev/null
  mv -- "$temporary" "$daily"
  sha256sum "$daily" > "$daily.sha256"
  trap - EXIT INT TERM

  if [ "$(date -u +%u)" = "7" ]; then
    weekly="$backup_root/weekly/ravshann-$timestamp.dump"
    cp -- "$daily" "$weekly"
    sha256sum "$weekly" > "$weekly.sha256"
  fi

  rotate_backups "$backup_root/daily" "$daily_keep"
  rotate_backups "$backup_root/weekly" "$weekly_keep"
  printf 'backup_completed file=%s\n' "$daily"
}

while true; do
  create_backup
  if [ "${BACKUP_RUN_ONCE:-false}" = "true" ]; then
    exit 0
  fi
  sleep "$interval"
done
