# NouSchool

SaaS multi-tenant untuk sekolah: absensi siswa, monitoring guru mengajar, izin guru, dashboard. Backend Go + PostgreSQL, frontend React PWA. Dokumentasi desain lengkap ada di [`docs/`](docs/) — mulai dari [`docs/00-overview.md`](docs/00-overview.md), progres di [`docs/ROADMAP.md`](docs/ROADMAP.md), aturan kerja di [`CLAUDE.md`](CLAUDE.md).

## Menjalankan (dev)

```sh
# Backend (Go 1.24+)
cp .env.example .env        # isi DATABASE_URL bila Postgres sudah siap
PORT=8210 go run ./cmd/server
# → http://localhost:8210/api/health

# Frontend
cd web && npm install && npm run dev
# → http://localhost:5173 (proxy /api → backend)

# Migrasi database (butuh Postgres + DATABASE_URL)
make migrate-up

# Generate kode type-safe dari SQL
make sqlc
```

Catatan Windows dev: port 7929–8171 direserve Hyper-V — pakai 8210.
