-- +goose Up
-- Fondasi multi-tenant + identity. Lihat docs/01-tenant.md dan docs/02-identity.md.

CREATE TABLE schools (
    id            bigserial PRIMARY KEY,
    name          text NOT NULL,
    slug          text NOT NULL UNIQUE,
    custom_domain text UNIQUE,
    timezone      text NOT NULL DEFAULT 'Asia/Jakarta',
    status        text NOT NULL DEFAULT 'active',
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE academic_years (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    name       text NOT NULL,
    starts_on  date NOT NULL,
    ends_on    date NOT NULL,
    is_active  boolean NOT NULL DEFAULT false,
    UNIQUE (school_id, name)
);
-- Hanya satu tahun ajaran aktif per sekolah.
CREATE UNIQUE INDEX academic_years_one_active
    ON academic_years (school_id) WHERE is_active;

CREATE TABLE school_settings (
    school_id  bigint NOT NULL REFERENCES schools (id),
    module     text NOT NULL,
    settings   jsonb NOT NULL,
    updated_by bigint,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (school_id, module)
);

CREATE TABLE users (
    id             bigserial PRIMARY KEY,
    email          text UNIQUE,
    username       text UNIQUE,
    password_hash  text NOT NULL,
    name           text NOT NULL,
    phone          text,
    is_super_admin boolean NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CHECK (email IS NOT NULL OR username IS NOT NULL)
);

CREATE TABLE memberships (
    id        bigserial PRIMARY KEY,
    user_id   bigint NOT NULL REFERENCES users (id),
    school_id bigint NOT NULL REFERENCES schools (id),
    role      text NOT NULL,
    status    text NOT NULL DEFAULT 'active',
    UNIQUE (user_id, school_id, role)
);
CREATE INDEX memberships_school ON memberships (school_id);

CREATE TABLE sessions (
    id         bigserial PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id),
    school_id  bigint NOT NULL REFERENCES schools (id),
    token_hash bytea NOT NULL UNIQUE,
    role       text NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    ip         inet,
    user_agent text
);
CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE invitations (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    code       text NOT NULL UNIQUE,
    role       text NOT NULL,
    target_id  bigint,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);

CREATE TABLE audit_log (
    id        bigserial PRIMARY KEY,
    school_id bigint,
    user_id   bigint,
    action    text NOT NULL,
    entity    text NOT NULL,
    entity_id bigint,
    old_value jsonb,
    new_value jsonb,
    at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_school_at ON audit_log (school_id, at);

-- +goose Down
DROP TABLE audit_log;
DROP TABLE invitations;
DROP TABLE sessions;
DROP TABLE memberships;
DROP TABLE users;
DROP TABLE school_settings;
DROP TABLE academic_years;
DROP TABLE schools;
