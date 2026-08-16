# NouSchool

SaaS multi-tenant untuk sekolah: absensi siswa, monitoring guru mengajar, izin guru, dashboard. Backend Go + PostgreSQL, frontend React PWA. Dokumentasi desain lengkap ada di [`docs/`](docs/) — mulai dari [`docs/00-overview.md`](docs/00-overview.md), progres di [`docs/ROADMAP.md`](docs/ROADMAP.md), aturan kerja di [`CLAUDE.md`](CLAUDE.md).

## Menjalankan (dev)

SEMUA komponen jalan di Docker — satu perintah, log terpusat.

```sh
# 1. Nyalakan semuanya: db + api (hot reload Air) + web (Vite dev server)
make docker-up
# → app: http://demo.localhost:5173 (tenant demo) & http://localhost:5173 (platform)
# → api: http://localhost:8210/api/health
# → db:  localhost:5434 (postgres/nouschool/nouschool)

# 2. Migrasi database (goose dijalankan dari host, connect ke db yang dipublish container)
make docker-migrate

# 3. Cek log kapan pun (semua service, atau sebut satu: `docker compose logs -f web`)
make docker-logs
```

`make docker-down` mematikan container (data db tetap tersimpan di volume `nouschool_pgdata`).

Hot reload dua-duanya otomatis: kode Go di-rebuild [Air](https://github.com/air-verse/air), frontend di-HMR Vite — keduanya memakai polling karena bind mount Windows tidak meneruskan file event. `node_modules` frontend hidup di volume container sendiri (binary native Linux ≠ Windows) — `npm install` di host hanya perlu untuk tooling editor.

<details>
<summary>Alternatif: jalankan backend langsung di host (tanpa Docker untuk api)</summary>

```sh
cp .env.example .env        # isi DATABASE_URL bila Postgres sudah siap
PORT=8210 go run ./cmd/server
# → http://localhost:8210/api/health

# Migrasi database (butuh Postgres + DATABASE_URL)
make migrate-up

# Generate kode type-safe dari SQL
make sqlc
```

</details>

Catatan Windows dev: port host 7929–8171 direserve Hyper-V (8080 tidak bisa dipublish) — backend host port dev pakai 8210. Port Postgres default 5432/5433 di mesin ini sudah dipakai project Docker lain, jadi `db` di-publish ke 5434 (lihat `docker-compose.yml`).

## Operasional

### Backup database

`scripts/backup.sh` — `pg_dump` database dari container `db` (docker-compose.yml), dikompres gzip ke `backups/nouschool-YYYY-MM-DD.sql.gz`. Retensi otomatis: hanya 14 file terakhir disimpan.

```sh
./scripts/backup.sh
# → backups/nouschool-2026-08-16.sql.gz
```

Direktori `backups/` diabaikan git (`.gitignore`) — file backup TIDAK pernah masuk repo.

**Saran cron VPS produksi** (backup harian jam 02:00, log ke file):

```
0 2 * * * cd /srv/nouschool && ./scripts/backup.sh >> /var/log/nouschool-backup.log 2>&1
```

### Restore database

`scripts/restore.sh <path-ke-file.sql.gz>` — meng-*restore* isi backup ke database lewat `psql` di container `db`.

```sh
./scripts/restore.sh backups/nouschool-2026-08-16.sql.gz
```

**PERINGATAN: restore MENIMPA seluruh data yang ada sekarang di database tujuan** — semua data saat ini akan hilang, digantikan isi file backup. Script meminta konfirmasi ketik `ya` sebelum jalan (tanpa `-y`/flag untuk lewati konfirmasi, disengaja). Selalu pastikan file & environment (dev vs prod) sudah benar sebelum menjalankan — TIDAK ADA cara membatalkan setelah `psql` mulai menulis.

Variabel env opsional untuk kedua script (default mengikuti `docker-compose.yml`): `DB_SERVICE=db`, `DB_USER=nouschool`, `DB_NAME=nouschool`; `RETENTION=14` khusus `backup.sh`.

### Custom domain (produksi)

Set `SERVER_IP` (IP publik VPS) di env server sebelum admin sekolah bisa memverifikasi custom domain (`PUT`/`POST /api/custom-domain/verify`) — kosong (default dev) membuat verifikasi selalu gagal dengan pesan jelas. Lihat `docs/01-tenant.md` "Custom domain & Caddy" dan blok Caddyfile PROD (komentar) untuk rute `/manifest.webmanifest`.
