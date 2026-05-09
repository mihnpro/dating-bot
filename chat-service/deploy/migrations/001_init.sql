CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- One conversation per match
CREATE TABLE IF NOT EXISTS conversations (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id    BIGINT      NOT NULL UNIQUE,
    user1_id    BIGINT      NOT NULL,
    user2_id    BIGINT      NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at  TIMESTAMP   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversations_user1 ON conversations(user1_id);
CREATE INDEX IF NOT EXISTS idx_conversations_user2 ON conversations(user2_id);

CREATE TABLE IF NOT EXISTS messages (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID        NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id       BIGINT      NOT NULL,
    content         TEXT        NOT NULL,
    content_type    VARCHAR(20) NOT NULL DEFAULT 'text',
    sent_at         TIMESTAMP   NOT NULL DEFAULT NOW(),
    is_read         BOOLEAN     NOT NULL DEFAULT FALSE
);

-- Optimised for paginated message history (newest-first lookups)
CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, sent_at ASC);
CREATE INDEX IF NOT EXISTS idx_messages_unread ON messages(conversation_id, sender_id, is_read)
    WHERE is_read = false;
