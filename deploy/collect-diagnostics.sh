#!/bin/sh
set -eu

SINCE="${1:-24h}"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUTPUT_DIR="diagnostics-${STAMP}"
ARCHIVE="${OUTPUT_DIR}.tar.gz"
COMPOSE="docker compose --env-file .env -f deploy/compose.yaml"

mkdir -p "$OUTPUT_DIR"
$COMPOSE ps > "$OUTPUT_DIR/containers.txt" 2>&1 || true
$COMPOSE logs --timestamps --since "$SINCE" > "$OUTPUT_DIR/compose.log" 2>&1 || true
docker stats --no-stream > "$OUTPUT_DIR/resources.txt" 2>&1 || true
docker system df > "$OUTPUT_DIR/docker-disk.txt" 2>&1 || true
df -h > "$OUTPUT_DIR/server-disk.txt" 2>&1 || true
$COMPOSE exec -T frontend du -sh /media > "$OUTPUT_DIR/ravshtok-media.txt" 2>&1 || true
$COMPOSE exec -T postgres psql -U ravshann -d ravshann -c \
  "SELECT status,count(*) AS items,pg_size_pretty(COALESCE(sum(size_bytes),0)::bigint) AS media_size FROM ravshtok_media GROUP BY status ORDER BY status;" \
  > "$OUTPUT_DIR/ravshtok-queue.txt" 2>&1 || true

tar -czf "$ARCHIVE" "$OUTPUT_DIR"
printf 'Created %s\n' "$ARCHIVE"
