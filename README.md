# NouSchool

SaaS multi-tenant untuk sekolah: absensi siswa, monitoring guru mengajar, izin guru, dashboard. Backend Go + PostgreSQL, frontend React PWA. Dokumentasi desain lengkap ada di [`docs/`](docs/) — mulai dari [`docs/00-overview.md`](docs/00-overview.md), progres di [`docs/ROADMAP.md`](docs/ROADMAP.md), aturan kerja di [`CLAUDE.md`](CLAUDE.md).

## Menjalankan (dev)

Cara tercepat: Docker untuk backend + Postgres, frontend jalan langsung di host.

```sh
# 1. Nyalakan db + api (hot reload via Air) di Docker
make docker-up
# → api: http://localhost:8210/api/health
# → db:  localhost:5434 (postgres/nouschool/nouschool)

# 2. Migrasi database (goose dijalankan dari host, connect ke db yang dipublish container)
make docker-migrate

# 3. Frontend — tetap dijalankan di host, bukan di container
cd web && npm install && npm run dev
# → http://localhost:5173 (proxy /api → backend, lihat VITE_API_PROXY)
```

Perintah Docker lain: `make docker-logs` (ikuti log db+api), `make docker-down` (matikan container, data db tetap tersimpan di volume `nouschool_pgdata`).

Kode Go di-mount ke container `api` dan di-rebuild otomatis oleh [Air](https://github.com/air-verse/air) tiap kali file `.go` berubah — tidak perlu restart manual.

Ingin frontend juga ikut di container (mis. testing dari device lain di jaringan yang sama)? `docker compose --profile full up -d` menambahkan service `web` di `http://localhost:5173`.

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
