CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE organization (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE app_user (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_type   TEXT NOT NULL,
    org_id        UUID REFERENCES organization (id),
    name          TEXT NOT NULL,
    email         TEXT,
    phone         TEXT,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_app_user_identifier CHECK ((email IS NOT NULL) <> (phone IS NOT NULL)),
    CONSTRAINT chk_app_user_role CHECK (role IN ('client', 'resource')),
    CONSTRAINT chk_app_user_tenant_type CHECK (tenant_type IN ('individual', 'org')),
    CONSTRAINT chk_app_user_tenant_consistency CHECK (
        (tenant_type = 'individual' AND org_id IS NULL) OR
        (tenant_type = 'org' AND org_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX uq_app_user_email
    ON app_user (lower(email))
    WHERE email IS NOT NULL;

CREATE UNIQUE INDEX uq_app_user_phone
    ON app_user (phone)
    WHERE phone IS NOT NULL;

CREATE INDEX idx_app_user_org_id
    ON app_user (org_id)
    WHERE org_id IS NOT NULL;

CREATE UNIQUE INDEX uq_organization_name_lower
    ON organization (lower(name));
