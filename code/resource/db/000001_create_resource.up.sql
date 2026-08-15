CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE resource_type (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_resource_type_name UNIQUE (name)
);

CREATE UNIQUE INDEX uq_resource_type_name_lower
    ON resource_type (lower(name));

CREATE TABLE resource (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_type      TEXT NOT NULL,
    org_id           UUID,
    user_id          UUID,
    resource_type_id UUID NOT NULL REFERENCES resource_type (id),
    name             TEXT NOT NULL,
    meeting_mode     SMALLINT NOT NULL,
    location         GEOGRAPHY(POINT, 4326),
    attributes       JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_resource_tenant_type CHECK (tenant_type IN ('individual', 'org')),
    CONSTRAINT chk_resource_tenant_consistency CHECK (
        (tenant_type = 'individual' AND org_id IS NULL) OR
        (tenant_type = 'org' AND org_id IS NOT NULL)
    ),
    CONSTRAINT chk_resource_meeting_mode CHECK (meeting_mode BETWEEN 1 AND 3),
    CONSTRAINT chk_resource_location_required
        CHECK (meeting_mode = 1 OR location IS NOT NULL)
);

CREATE INDEX idx_resource_org_id
    ON resource (org_id)
    WHERE org_id IS NOT NULL;

CREATE UNIQUE INDEX uq_resource_user_id
    ON resource (user_id)
    WHERE user_id IS NOT NULL;

CREATE INDEX idx_resource_org_type
    ON resource (org_id, resource_type_id);

CREATE INDEX idx_resource_attributes
    ON resource USING GIN (attributes);

CREATE INDEX idx_resource_location
    ON resource USING GIST (location);

CREATE INDEX idx_resource_name_trgm
    ON resource USING GIN (name gin_trgm_ops);

CREATE TABLE recurrence_rule (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id  UUID NOT NULL REFERENCES resource (id) ON DELETE CASCADE,
    day_of_week  SMALLINT NOT NULL,
    timezone     TEXT NOT NULL,
    slots        JSONB NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_recurrence_rule_day CHECK (day_of_week BETWEEN 1 AND 7),
    CONSTRAINT uq_recurrence_rule_resource_day UNIQUE (resource_id, day_of_week)
);

CREATE INDEX idx_recurrence_rule_resource_id
    ON recurrence_rule (resource_id);

CREATE TABLE slot (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_id  UUID NOT NULL REFERENCES resource (id) ON DELETE CASCADE,
    start_time   TIMESTAMPTZ NOT NULL,
    end_time     TIMESTAMPTZ NOT NULL,
    status       SMALLINT NOT NULL DEFAULT 1,
    source       TEXT NOT NULL,
    reason       TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_slot_status CHECK (status BETWEEN 1 AND 4),
    CONSTRAINT chk_slot_source CHECK (source IN ('recurring', 'exception')),
    CONSTRAINT chk_slot_time_order CHECK (end_time > start_time),
    CONSTRAINT uq_slot_resource_start UNIQUE (resource_id, start_time)
);

CREATE INDEX idx_slot_resource_time
    ON slot (resource_id, start_time);

CREATE INDEX idx_slot_open
    ON slot (resource_id, start_time)
    WHERE status = 1;
