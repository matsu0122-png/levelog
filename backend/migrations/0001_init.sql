-- Initial schema for levelog.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    timezone      TEXT NOT NULL DEFAULT 'Asia/Tokyo',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);

CREATE TABLE mission_templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    difficulty  TEXT NOT NULL CHECK (difficulty IN ('EASY', 'NORMAL', 'HARD', 'EXTREME')),
    xp_reward   INTEGER NOT NULL CHECK (xp_reward > 0),
    active      BOOLEAN NOT NULL DEFAULT true,
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_mission_templates_user_id ON mission_templates(user_id) WHERE deleted_at IS NULL;

CREATE TABLE mission_schedule_days (
    mission_template_id UUID NOT NULL REFERENCES mission_templates(id) ON DELETE CASCADE,
    day_of_week          SMALLINT NOT NULL CHECK (day_of_week BETWEEN 1 AND 7), -- 1=Monday .. 7=Sunday
    PRIMARY KEY (mission_template_id, day_of_week)
);

CREATE TABLE daily_missions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mission_template_id  UUID NOT NULL REFERENCES mission_templates(id) ON DELETE RESTRICT,
    target_date          DATE NOT NULL,
    title_snapshot       TEXT NOT NULL,
    xp_reward_snapshot   INTEGER NOT NULL CHECK (xp_reward_snapshot > 0),
    status               TEXT NOT NULL CHECK (status IN ('PENDING', 'COMPLETED')) DEFAULT 'PENDING',
    completed_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, mission_template_id, target_date)
);

CREATE INDEX idx_daily_missions_user_date ON daily_missions(user_id, target_date);

CREATE TABLE xp_transactions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    daily_mission_id UUID NOT NULL REFERENCES daily_missions(id) ON DELETE CASCADE,
    amount           INTEGER NOT NULL,
    transaction_type TEXT NOT NULL CHECK (transaction_type IN ('COMPLETE', 'UNCOMPLETE')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_xp_transactions_user_id ON xp_transactions(user_id);
CREATE INDEX idx_xp_transactions_daily_mission_id ON xp_transactions(daily_mission_id);

-- A given daily mission may only ever have one COMPLETE transaction that
-- lacks a matching UNCOMPLETE reversal at a time; the application enforces
-- exact pairing via the PENDING/COMPLETED status guard in a single
-- transaction, so no additional partial-unique index is required here.
