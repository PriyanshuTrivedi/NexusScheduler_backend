CREATE TABLE object_storage (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES app_user (id) ON DELETE CASCADE,
    object_type         TEXT NOT NULL,
    visibility          TEXT NOT NULL,
    object_key          TEXT NOT NULL,
    dbbyte_file_id      TEXT,
    original_file_name  TEXT NOT NULL,
    mime_type           TEXT NOT NULL,
    size_bytes          BIGINT NOT NULL CHECK (size_bytes > 0),
    idempotency_key     UUID,
    content_sha256      CHAR(64) NOT NULL,
    pinecone_synced_at  TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_object_storage_content_sha256
        CHECK (content_sha256 ~ '^[0-9a-f]{64}$'),

    CONSTRAINT chk_object_storage_visibility
        CHECK (visibility IN ('PUBLIC', 'PRIVATE')),

    CONSTRAINT uq_object_storage_user_idempotency
        UNIQUE (user_id, idempotency_key),

    CONSTRAINT uq_object_storage_object_key
        UNIQUE (object_key)
);

CREATE INDEX idx_object_storage_user_updated
    ON object_storage (user_id, updated_at DESC);

CREATE INDEX idx_object_storage_private_lookup
    ON object_storage (id, user_id)
    WHERE visibility = 'PRIVATE';