-- Migration: 001_create_matches_and_interactions
-- Database: matching_db

CREATE TABLE IF NOT EXISTS matches (
    id BIGSERIAL PRIMARY KEY,
    user1_id BIGINT NOT NULL,
    user2_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'expired', 'blocked')),
    conversation_started BOOLEAN DEFAULT FALSE,
    last_interaction_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_match UNIQUE (user1_id, user2_id)
);

CREATE INDEX idx_matches_user1_id ON matches(user1_id);
CREATE INDEX idx_matches_user2_id ON matches(user2_id);
CREATE INDEX idx_matches_created_at ON matches(created_at DESC);
CREATE INDEX idx_matches_status ON matches(status);

CREATE TABLE IF NOT EXISTS interactions (
    id BIGSERIAL PRIMARY KEY,
    from_user_id BIGINT NOT NULL,
    to_user_id BIGINT NOT NULL,
    type VARCHAR(10) NOT NULL CHECK (type IN ('like', 'pass')),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    time_of_day INT NOT NULL CHECK (time_of_day >= 0 AND time_of_day <= 23)
);

CREATE INDEX idx_interactions_from_user ON interactions(from_user_id);
CREATE INDEX idx_interactions_to_user ON interactions(to_user_id);
CREATE INDEX idx_interactions_created_at ON interactions(created_at DESC);
CREATE UNIQUE INDEX idx_interactions_unique_pair ON interactions(from_user_id, to_user_id);
