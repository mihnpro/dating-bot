CREATE TABLE IF NOT EXISTS media (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT NOT NULL,
    s3_key            VARCHAR(512) NOT NULL UNIQUE,
    original_filename VARCHAR(255),
    mime_type         VARCHAR(100),
    file_size         BIGINT,
    is_main           BOOLEAN NOT NULL DEFAULT FALSE,
    uploaded_at       TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_media_user_id   ON media(user_id);
CREATE INDEX idx_media_s3_key    ON media(s3_key);
CREATE INDEX idx_media_user_main ON media(user_id, is_main);
