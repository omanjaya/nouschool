-- +goose Up
-- Super admin bisa login di host platform (tanpa sekolah) — session-nya
-- tidak terikat school_id. Migrasi tambahan, TIDAK mengedit 00001.
ALTER TABLE sessions ALTER COLUMN school_id DROP NOT NULL;

-- +goose Down
ALTER TABLE sessions ALTER COLUMN school_id SET NOT NULL;
