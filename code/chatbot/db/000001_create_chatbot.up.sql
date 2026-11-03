-- Conversation table
CREATE TABLE conversation (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID NOT NULL,
    title                TEXT NOT NULL DEFAULT 'New chat',
    last_message_preview TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'ACTIVE',
    close_reason         TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_conversation_status CHECK (status IN ('ACTIVE', 'CLOSED')),
    CONSTRAINT chk_conversation_close_reason CHECK (
        (status = 'ACTIVE' AND close_reason IS NULL)
        OR
        (
            status = 'CLOSED'
            AND close_reason IS NOT NULL
            AND close_reason IN ('USER', 'IDLE_TIMEOUT')
        )
    )
);

-- A user can have at most one active conversation.
CREATE UNIQUE INDEX uq_conversation_active_user
ON conversation (user_id)
WHERE status = 'ACTIVE';

-- Efficiently list a user's conversations, newest activity first.
CREATE INDEX idx_conversation_user_updated
ON conversation (user_id, updated_at DESC, id DESC);


-- Message table
CREATE TABLE message (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id         UUID NOT NULL REFERENCES conversation (id) ON DELETE CASCADE,
    role                    TEXT NOT NULL,
    content                 TEXT NOT NULL,
    client_idempotency_key  TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_message_role CHECK (role IN ('USER', 'ASSISTANT')),
    CONSTRAINT chk_message_content CHECK (length(trim(content)) > 0),
    CONSTRAINT chk_message_idempotency_key CHECK (
        client_idempotency_key IS NULL
        OR
        (role = 'USER' AND length(trim(client_idempotency_key)) > 0)
    )
);

-- Retrieve all messages belonging to a conversation in chronological order.
CREATE INDEX idx_message_conversation_created
ON message (conversation_id, created_at, id);

-- Prevent duplicate message submissions when the client retries.
CREATE UNIQUE INDEX uq_message_client_idempotency_key
ON message (client_idempotency_key)
WHERE client_idempotency_key IS NOT NULL;