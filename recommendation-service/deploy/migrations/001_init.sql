-- ============================================================
-- Recommendation Service — initial schema
-- Database: rating_db
-- ============================================================

CREATE TABLE IF NOT EXISTS ratings (
    user_id          BIGINT       PRIMARY KEY,
    gender           VARCHAR(10)  NOT NULL DEFAULT '',
    age              INT          NOT NULL DEFAULT 0,
    city             VARCHAR(255) NOT NULL DEFAULT '',
    primary_rating   FLOAT        NOT NULL DEFAULT 0,
    behavioral_rating FLOAT       NOT NULL DEFAULT 0,
    combined_rating  FLOAT        NOT NULL DEFAULT 0,
    calculated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Fast lookup for feed ranking queries (ordered by score DESC)
CREATE INDEX IF NOT EXISTS idx_ratings_combined
    ON ratings (combined_rating DESC);

-- Fast gender-filtered feed queries
CREATE INDEX IF NOT EXISTS idx_ratings_gender_combined
    ON ratings (gender, combined_rating DESC);

-- Fast age-range queries
CREATE INDEX IF NOT EXISTS idx_ratings_age
    ON ratings (age);

-- ============================================================

CREATE TABLE IF NOT EXISTS rating_log (
    id           BIGSERIAL    PRIMARY KEY,
    user_id      BIGINT       NOT NULL REFERENCES ratings(user_id) ON DELETE CASCADE,
    old_combined FLOAT,
    new_combined FLOAT,
    reason       TEXT,
    changed_at   TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rating_log_user_id
    ON rating_log (user_id);

CREATE INDEX IF NOT EXISTS idx_rating_log_changed_at
    ON rating_log (changed_at DESC);
