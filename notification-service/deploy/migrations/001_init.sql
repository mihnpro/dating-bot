-- База данных: notification_db

CREATE TABLE IF NOT EXISTS notifications (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT        NOT NULL,
    type       VARCHAR(50)   NOT NULL,  -- match_created | new_like | new_message
    message    TEXT          NOT NULL,
    is_sent    BOOLEAN       NOT NULL DEFAULT FALSE,
    is_read    BOOLEAN       NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP     NOT NULL DEFAULT NOW(),
    sent_at    TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_user_id    ON notifications(user_id);
CREATE INDEX IF NOT EXISTS idx_notifications_is_read    ON notifications(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
