#!/usr/bin/env bash
# scripts/restore.sh — restore database dari file backup .sql.gz (hasil
# scripts/backup.sh) ke container db (docker-compose.yml, service "db").
#
# !!! PERINGATAN: operasi ini MENIMPA seluruh isi database tujuan (DROP data
# lama secara implisit lewat psql yang menjalankan ulang dump). Semua data
# yang ada SEKARANG di database akan HILANG dan digantikan isi file backup.
# TIDAK ADA konfirmasi interaktif tambahan di script ini — pastikan file &
# target sudah benar SEBELUM menjalankan.
#
# Pakai: ./scripts/restore.sh backups/nouschool-2026-08-16.sql.gz
#
# Env opsional (override default docker-compose.yml):
#   DB_SERVICE   nama service docker compose (default: db)
#   DB_USER      user Postgres (default: nouschool)
#   DB_NAME      nama database (default: nouschool)

set -euo pipefail

DB_SERVICE="${DB_SERVICE:-db}"
DB_USER="${DB_USER:-nouschool}"
DB_NAME="${DB_NAME:-nouschool}"

if [ "$#" -ne 1 ]; then
    echo "Pakai: $0 <path-ke-file-backup.sql.gz>" >&2
    exit 1
fi

BACKUP_FILE="$1"
if [ ! -f "$BACKUP_FILE" ]; then
    echo "File tidak ditemukan: $BACKUP_FILE" >&2
    exit 1
fi

echo "!!! PERINGATAN: restore akan MENIMPA seluruh data di database '$DB_NAME' (service '$DB_SERVICE')."
echo "!!! File sumber: $BACKUP_FILE"
echo "Lanjutkan? Ketik 'ya' untuk melanjutkan, apa pun selain itu untuk batal:"
read -r CONFIRM
if [ "$CONFIRM" != "ya" ]; then
    echo "Dibatalkan."
    exit 1
fi

echo "==> Restore dimulai..."
gunzip -c "$BACKUP_FILE" | docker compose exec -T "$DB_SERVICE" psql -U "$DB_USER" "$DB_NAME"
echo "==> Restore selesai."
