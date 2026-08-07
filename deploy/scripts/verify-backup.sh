#!/bin/sh
set -eu

backup_file="${1:-}"
if [ -z "$backup_file" ] || [ ! -f "$backup_file" ]; then
  echo "usage: verify-backup.sh /backups/daily/ravshann-....dump" >&2
  exit 2
fi

checksum_file="$backup_file.sha256"
if [ ! -f "$checksum_file" ]; then
  echo "checksum file not found: $checksum_file" >&2
  exit 3
fi

(cd "$(dirname "$backup_file")" && sha256sum -c "$(basename "$checksum_file")")
pg_restore --list "$backup_file" >/dev/null
printf 'backup_valid file=%s\n' "$backup_file"
