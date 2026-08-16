-- +goose Up
-- Impersonate USER oleh admin sekolah (Fase 14 Gelombang D,
-- docs/12-sion-parity.md): admin_sekolah bisa "masuk sebagai" member lain
-- sekolahnya (guru/siswa/orang tua/kepala sekolah/pegawai) utk mendukung,
-- BUKAN sesi impersonasi super admin fase 13 (yang memakai sentinel role
-- tanpa kolom ini). impersonator_user_id NULL = sesi normal;
-- terisi = sesi ini adalah "menyamar sebagai" user_id sesi, ATAS NAMA
-- admin_sekolah dgn id di kolom ini — dipakai RequireAuth menolak sliding
-- renewal (lihat internal/identity/middleware.go) & GET /api/me
-- (field impersonated_by).
ALTER TABLE sessions ADD COLUMN impersonator_user_id bigint REFERENCES users (id);

-- +goose Down
ALTER TABLE sessions DROP COLUMN impersonator_user_id;
