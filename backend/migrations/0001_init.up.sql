-- 0001_init — identidad, auditoría y plataforma.
--
-- Un esquema por módulo (ADR-0003): la frontera entre módulos se ve en la base,
-- no solo en el árbol de paquetes.

CREATE EXTENSION IF NOT EXISTS citext;

CREATE SCHEMA IF NOT EXISTS identity;
CREATE SCHEMA IF NOT EXISTS audit;
CREATE SCHEMA IF NOT EXISTS platform;

-- ---------------------------------------------------------------- identity --

CREATE TABLE identity.users (
    id                uuid        PRIMARY KEY,
    email             citext      NOT NULL UNIQUE,
    email_verified_at timestamptz,
    password_hash     text        NOT NULL,
    full_name         text        NOT NULL,
    role              text        NOT NULL CHECK (role IN ('admin', 'teacher', 'student')),
    status            text        NOT NULL CHECK (status IN ('active', 'suspended', 'deleted')),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX users_role_status_idx ON identity.users (role, status);

-- Espejo de las sesiones para listado y auditoría. Redis manda sobre la
-- validez; esta tabla guarda la historia (ADR-0006).
CREATE TABLE identity.sessions (
    id           uuid        PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    token_sha256 bytea       NOT NULL UNIQUE,
    ip           text,
    user_agent   text,
    created_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz,
    revoked_by   uuid        REFERENCES identity.users (id)
);

CREATE INDEX sessions_user_idx ON identity.sessions (user_id, created_at DESC);

CREATE TABLE identity.one_time_tokens (
    id           uuid        PRIMARY KEY,
    user_id      uuid        NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    purpose      text        NOT NULL CHECK (purpose IN ('email_verify', 'password_reset')),
    token_sha256 bytea       NOT NULL,
    expires_at   timestamptz NOT NULL,
    consumed_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (purpose, token_sha256)
);

CREATE INDEX one_time_tokens_user_idx ON identity.one_time_tokens (user_id, purpose);

-- ------------------------------------------------------------------- audit --

CREATE TABLE audit.events (
    id          bigserial   PRIMARY KEY,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_id    uuid,
    actor_role  text        NOT NULL,
    action      text        NOT NULL,
    entity_type text        NOT NULL,
    entity_id   uuid,
    ip          text,
    user_agent  text,
    trace_id    text,
    metadata    jsonb       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX events_entity_idx ON audit.events (entity_type, entity_id, occurred_at DESC);
CREATE INDEX events_actor_idx  ON audit.events (actor_id, occurred_at DESC);
CREATE INDEX events_action_idx ON audit.events (action, occurred_at DESC);

-- La inmutabilidad de la auditoría vive donde están los datos: un UPDATE de
-- mantenimiento mal escrito debe fallar ruidosamente (ADR-0003, CE-02).
CREATE OR REPLACE FUNCTION audit.reject_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit.events es inmutable: operación % no permitida', TG_OP
        USING ERRCODE = 'restrict_violation';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER events_immutable
    BEFORE UPDATE OR DELETE ON audit.events
    FOR EACH ROW EXECUTE FUNCTION audit.reject_mutation();

-- ---------------------------------------------------------------- platform --

-- job_runs es a la vez outbox, registro de idempotencia y DLQ (ADR-0004, 0008).
CREATE TABLE platform.job_runs (
    id           bigserial   PRIMARY KEY,
    job_key      text        NOT NULL UNIQUE,
    type         text        NOT NULL,
    queue        text        NOT NULL,
    status       text        NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'dead')),
    attempt      int         NOT NULL DEFAULT 0,
    payload      jsonb       NOT NULL DEFAULT '{}'::jsonb,
    last_error   text,
    heartbeat_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    started_at   timestamptz,
    finished_at  timestamptz
);

-- Índice del reaper: trabajos vivos sin heartbeat reciente (ADR-0004).
CREATE INDEX job_runs_running_idx ON platform.job_runs (heartbeat_at)
    WHERE status = 'running';
CREATE INDEX job_runs_pending_idx ON platform.job_runs (created_at)
    WHERE status IN ('queued', 'failed');
CREATE INDEX job_runs_dead_idx ON platform.job_runs (finished_at DESC)
    WHERE status = 'dead';

CREATE TABLE platform.idempotency_keys (
    key             text        NOT NULL,
    user_id         uuid        NOT NULL,
    endpoint        text        NOT NULL,
    request_hash    bytea       NOT NULL,
    status          text        NOT NULL CHECK (status IN ('in_progress', 'completed')),
    response_status int,
    response_body   bytea,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, endpoint, key)
);

CREATE INDEX idempotency_keys_created_idx ON platform.idempotency_keys (created_at);
