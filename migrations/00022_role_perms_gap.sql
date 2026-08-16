-- +goose Up
-- Fase 15 Gap 2 (docs/12-sion-parity.md "matrix permission per role per
-- sekolah, override — TANPA custom role baru"): school_role_permissions
-- menyimpan PENGECUALIAN dari peta rolePermissions statis
-- (internal/identity/rbac.go) per sekolah — BUKAN tabel role/permission baru
-- (custom role di DB tetap TIDAK dikerjakan, lihat "Ide tertunda" di
-- docs/ROADMAP.md). allowed=true/false = override paksa untuk (role,
-- permission) itu di sekolah ini; TIDAK ADA baris = pakai default statis
-- (lihat internal/identity/permoverride.go & middleware.go RequirePerm).
CREATE TABLE school_role_permissions (
    school_id  bigint NOT NULL REFERENCES schools (id),
    role       text NOT NULL,
    permission text NOT NULL,
    allowed    boolean NOT NULL,
    PRIMARY KEY (school_id, role, permission)
);

-- +goose Down
DROP TABLE school_role_permissions;
