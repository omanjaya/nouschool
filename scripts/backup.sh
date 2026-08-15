#!/usr/bin/env bash
# scripts/backup.sh — backup harian Postgres via pg_dump di dalam container db
# (docker-compose.yml, service "db") ke backups/nouschool-YYYY-MM-DD.sql.gz.
# Retensi: hanya 14 file terakhir disimpan, sisanya dihapus otomatis.
#
# Pakai: ./scripts/backup.sh   (dari root repo, atau path apa pun — script
# menentukan root repo dari lokasinya sendiri).
#
# Env opsional (override default docker-compose.yml):
#   DB_SERVICE   nama service docker compose (default: db)
#   DB_USER      user Postgres (default: nouschool)
#   DB_NAME      nama database (default: nouschool)
#   RETENTION    jumlah file terakhir yang disimpan (default: 14)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BACKUP_DIR="$REPO_ROOT/backups"

DB_SERVICE="${DB_SERVICE:-db}"
DB_USER="${DB_USER:-nouschool}"
DB_NAME="${DB_NAME:-nouschool}"
RETENTION="${RETENTION:-14}"

mkdir -p "$BACKUP_DIR"

STAMP="$(date +%F)"
OUT_FILE="$BACKUP_DIR/nouschool-$STAMP.sql.gz"

echo "==> Backup database '$DB_NAME' (service compose '$DB_SERVICE') -> $OUT_FILE"

docker compose exec -T "$DB_SERVICE" pg_dump -U "$DB_USER" "$DB_NAME" | gzip > "$OUT_FILE"

SIZE=$(du -h "$OUT_FILE" | cut -f1)
echo "==> Selesai: $OUT_FILE ($SIZE)"

echo "==> Retensi: menyimpan $RETENTION file terakhir, menghapus sisanya."
# Urut berdasarkan nama (format tanggal YYYY-MM-DD sudah urut leksikografis =
# urut kronologis), yang tersisa setelah N terbaru dihapus.
ls -1t "$BACKUP_DIR"/nouschool-*.sql.gz 2>/dev/null | tail -n +"$((RETENTION + 1))" | while read -r old; do
    echo "    hapus: $old"
    rm -f -- "$old"
done

echo "==> Backup selesai."
