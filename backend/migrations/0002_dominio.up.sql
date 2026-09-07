-- 0002_dominio — autoría, medios, evaluación, progreso e insignias.
--
-- Un esquema por módulo (ADR-0003). Las reglas que más cuestan si se rompen
-- viven en la base, no solo en Go: inmutabilidad de las versiones publicadas,
-- unicidad de la insignia y del intento en curso.

CREATE SCHEMA IF NOT EXISTS authoring;
CREATE SCHEMA IF NOT EXISTS media;
CREATE SCHEMA IF NOT EXISTS assessment;
CREATE SCHEMA IF NOT EXISTS progress;
CREATE SCHEMA IF NOT EXISTS badges;

-- ------------------------------------------------------------------ media --
-- Va primero: authoring.resources la referencia.

CREATE TABLE media.assets (
    id                uuid        PRIMARY KEY,
    owner_id          uuid        NOT NULL REFERENCES identity.users (id),
    kind              text        NOT NULL CHECK (kind IN ('video','audio','image','pdf','slides','file')),
    original_key      text        NOT NULL,
    original_filename text        NOT NULL,
    declared_mime     text        NOT NULL,
    detected_mime     text,
    size_bytes        bigint      NOT NULL,
    declared_sha256   text        NOT NULL,
    sha256            text,
    duration_seconds  numeric,
    width             int,
    height            int,
    status            text        NOT NULL CHECK (status IN
                          ('uploading','uploaded','scanning','clean','processing',
                           'ready','rejected','infected','failed')),
    scan_result       text,
    scanned_at        timestamptz,
    last_error        text,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX assets_owner_idx  ON media.assets (owner_id, created_at DESC);
CREATE INDEX assets_status_idx ON media.assets (status);

CREATE TABLE media.uploads (
    id             uuid        PRIMARY KEY,
    asset_id       uuid        NOT NULL UNIQUE REFERENCES media.assets (id) ON DELETE CASCADE,
    s3_upload_id   text        NOT NULL,
    part_size      int         NOT NULL,
    total_parts    int         NOT NULL,
    parts_received jsonb       NOT NULL DEFAULT '[]'::jsonb,
    expires_at     timestamptz NOT NULL,   -- 24 h (RNF-07)
    completed_at   timestamptz,
    aborted_at     timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX uploads_expiry_idx ON media.uploads (expires_at)
    WHERE completed_at IS NULL AND aborted_at IS NULL;

CREATE TABLE media.asset_derivatives (
    id         uuid        PRIMARY KEY,
    asset_id   uuid        NOT NULL REFERENCES media.assets (id) ON DELETE CASCADE,
    kind       text        NOT NULL CHECK (kind IN ('hls_master','hls_variant','poster','pdf')),
    variant    text,
    key        text        NOT NULL,
    bytes      bigint      NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (asset_id, kind, variant)
);

-- -------------------------------------------------------------- authoring --

CREATE TABLE authoring.courses (
    id                 uuid        PRIMARY KEY,
    slug               text        NOT NULL UNIQUE,
    title              text        NOT NULL,
    summary            text        NOT NULL DEFAULT '',
    category           text,
    language           text,
    cover_asset_id     uuid        REFERENCES media.assets (id),
    owner_id           uuid        NOT NULL REFERENCES identity.users (id),
    status             text        NOT NULL CHECK (status IN ('draft','published','unpublished','archived')),
    current_version_id uuid,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX courses_owner_idx  ON authoring.courses (owner_id, updated_at DESC);
CREATE INDEX courses_status_idx ON authoring.courses (status, category, language);
-- Búsqueda del catálogo (RF-10).
CREATE INDEX courses_fts_idx ON authoring.courses
    USING gin (to_tsvector('spanish', title || ' ' || summary));

CREATE TABLE authoring.course_versions (
    id                uuid        PRIMARY KEY,
    course_id         uuid        NOT NULL REFERENCES authoring.courses (id) ON DELETE CASCADE,
    version_number    int         NOT NULL,
    status            text        NOT NULL CHECK (status IN ('draft','published','unpublished','archived')),
    approval_criteria jsonb       NOT NULL DEFAULT '{"required_completion_pct":100,"min_quiz_score":70}'::jsonb,
    published_at      timestamptz,
    published_by      uuid        REFERENCES identity.users (id),
    created_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (course_id, version_number)
);

ALTER TABLE authoring.courses
    ADD CONSTRAINT courses_current_version_fk
    FOREIGN KEY (current_version_id) REFERENCES authoring.course_versions (id);

CREATE TABLE authoring.modules (
    id                uuid        PRIMARY KEY,
    course_version_id uuid        NOT NULL REFERENCES authoring.course_versions (id) ON DELETE CASCADE,
    stable_id         uuid        NOT NULL,
    position          int         NOT NULL,
    title             text        NOT NULL,
    UNIQUE (course_version_id, stable_id)
);

CREATE TABLE authoring.units (
    id                uuid        PRIMARY KEY,
    module_id         uuid        NOT NULL REFERENCES authoring.modules (id) ON DELETE CASCADE,
    course_version_id uuid        NOT NULL REFERENCES authoring.course_versions (id) ON DELETE CASCADE,
    stable_id         uuid        NOT NULL,
    position          int         NOT NULL,
    title             text        NOT NULL,
    UNIQUE (course_version_id, stable_id)
);

CREATE TABLE authoring.resources (
    id                uuid        PRIMARY KEY,
    unit_id           uuid        NOT NULL REFERENCES authoring.units (id) ON DELETE CASCADE,
    course_version_id uuid        NOT NULL REFERENCES authoring.course_versions (id) ON DELETE CASCADE,
    stable_id         uuid        NOT NULL,
    position          int         NOT NULL,
    title             text        NOT NULL,
    type              text        NOT NULL CHECK (type IN
                          ('rich_text','image','video','audio','pdf','slides',
                           'download','iframe','link','quiz')),
    visible           boolean     NOT NULL DEFAULT true,
    required          boolean     NOT NULL DEFAULT true,
    downloadable      boolean     NOT NULL DEFAULT false,
    content_md        text,
    asset_refs        uuid[]      NOT NULL DEFAULT '{}',
    asset_id          uuid        REFERENCES media.assets (id),
    external_url      text,
    updated_at        timestamptz NOT NULL DEFAULT now(),
    UNIQUE (course_version_id, stable_id)
);

-- El cálculo del progreso consulta justo por aquí.
CREATE INDEX resources_required_idx
    ON authoring.resources (course_version_id, required, visible);

CREATE TABLE authoring.content_revisions (
    id          uuid        PRIMARY KEY,
    resource_id uuid        NOT NULL REFERENCES authoring.resources (id) ON DELETE CASCADE,
    content_md  text        NOT NULL,
    author_id   uuid        NOT NULL REFERENCES identity.users (id),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX content_revisions_idx ON authoring.content_revisions (resource_id, created_at DESC);

-- Una versión publicada es inmutable (ADR-0007). La regla vive en la base
-- porque es la que más fácil se salta un UPDATE de mantenimiento.
CREATE OR REPLACE FUNCTION authoring.reject_if_published() RETURNS trigger AS $$
DECLARE
    v_id     uuid;
    v_status text;
BEGIN
    v_id := COALESCE(NEW.course_version_id, OLD.course_version_id);
    SELECT status INTO v_status FROM authoring.course_versions WHERE id = v_id;
    IF v_status = 'published' THEN
        RAISE EXCEPTION 'la versión % está publicada y es inmutable', v_id
            USING ERRCODE = 'restrict_violation';
    END IF;
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER modules_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON authoring.modules
    FOR EACH ROW EXECUTE FUNCTION authoring.reject_if_published();
CREATE TRIGGER units_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON authoring.units
    FOR EACH ROW EXECUTE FUNCTION authoring.reject_if_published();
CREATE TRIGGER resources_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON authoring.resources
    FOR EACH ROW EXECUTE FUNCTION authoring.reject_if_published();

-- ------------------------------------------------------------- assessment --

CREATE TABLE assessment.quizzes (
    id                 uuid        PRIMARY KEY,
    resource_id        uuid        NOT NULL UNIQUE REFERENCES authoring.resources (id) ON DELETE CASCADE,
    course_version_id  uuid        NOT NULL REFERENCES authoring.course_versions (id) ON DELETE CASCADE,
    stable_id          uuid        NOT NULL,
    max_attempts       int         NOT NULL DEFAULT 0,
    time_limit_seconds int,
    pass_score         numeric     NOT NULL DEFAULT 70,
    feedback_policy    text        NOT NULL DEFAULT 'correctness'
                                   CHECK (feedback_policy IN ('none','score_only','correctness','full')),
    shuffle            boolean     NOT NULL DEFAULT false,
    partial_credit     boolean     NOT NULL DEFAULT false
);

CREATE TABLE assessment.quiz_questions (
    id             uuid    PRIMARY KEY,
    quiz_id        uuid    NOT NULL REFERENCES assessment.quizzes (id) ON DELETE CASCADE,
    stable_id      uuid    NOT NULL,
    position       int     NOT NULL,
    statement_md   text    NOT NULL,
    kind           text    NOT NULL CHECK (kind IN ('single','multiple')),
    points         numeric NOT NULL DEFAULT 1,
    explanation_md text,
    UNIQUE (quiz_id, stable_id)
);

CREATE TABLE assessment.quiz_options (
    id          uuid    PRIMARY KEY,
    question_id uuid    NOT NULL REFERENCES assessment.quiz_questions (id) ON DELETE CASCADE,
    stable_id   uuid    NOT NULL,
    position    int     NOT NULL,
    text_md     text    NOT NULL,
    -- Esta columna NUNCA sale por las rutas del estudiante (ADR-0013).
    is_correct  boolean NOT NULL,
    UNIQUE (question_id, stable_id)
);

CREATE TABLE assessment.quiz_attempts (
    id              uuid        PRIMARY KEY,
    quiz_stable_id  uuid        NOT NULL,
    course_id       uuid        NOT NULL REFERENCES authoring.courses (id) ON DELETE CASCADE,
    user_id         uuid        NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    enrollment_id   uuid        NOT NULL,
    attempt_number  int         NOT NULL,
    snapshot        jsonb       NOT NULL,   -- sin claves correctas
    status          text        NOT NULL CHECK (status IN
                        ('in_progress','submitted','expired','expired_submitted')),
    started_at      timestamptz NOT NULL DEFAULT now(),
    expires_at      timestamptz,
    submitted_at    timestamptz,
    score           numeric,
    score_detail    jsonb,
    grading_version int         NOT NULL DEFAULT 1
);

-- Un solo intento en curso por quiz y estudiante (ADR-0008).
CREATE UNIQUE INDEX attempts_one_in_progress
    ON assessment.quiz_attempts (quiz_stable_id, user_id)
    WHERE status = 'in_progress';

CREATE INDEX attempts_enrollment_idx ON assessment.quiz_attempts (enrollment_id, quiz_stable_id);

CREATE TABLE assessment.attempt_answers (
    attempt_id                 uuid   NOT NULL REFERENCES assessment.quiz_attempts (id) ON DELETE CASCADE,
    question_stable_id         uuid   NOT NULL,
    selected_option_stable_ids uuid[] NOT NULL DEFAULT '{}',
    saved_at                   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (attempt_id, question_stable_id)
);

-- --------------------------------------------------------------- progress --

CREATE TABLE progress.enrollments (
    id                  uuid        PRIMARY KEY,
    user_id             uuid        NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    course_id           uuid        NOT NULL REFERENCES authoring.courses (id) ON DELETE CASCADE,
    status              text        NOT NULL CHECK (status IN ('active','withdrawn')),
    state               text        NOT NULL CHECK (state IN ('in_progress','completed','approved')),
    progress_pct        numeric     NOT NULL DEFAULT 0,
    approved_version_id uuid        REFERENCES authoring.course_versions (id),
    enrolled_at         timestamptz NOT NULL DEFAULT now(),
    withdrawn_at        timestamptz,
    completed_at        timestamptz,
    approved_at         timestamptz
);

-- Una inscripción activa por curso y estudiante; reinscribir reactiva la fila.
CREATE UNIQUE INDEX enrollments_one_active
    ON progress.enrollments (user_id, course_id);

CREATE INDEX enrollments_course_idx ON progress.enrollments (course_id) WHERE status = 'active';

CREATE TABLE progress.resource_progress (
    enrollment_id         uuid    NOT NULL REFERENCES progress.enrollments (id) ON DELETE CASCADE,
    -- stable_id, no id de fila: es lo que hace que el progreso sobreviva a una
    -- versión nueva del curso (ADR-0007).
    resource_stable_id    uuid    NOT NULL,
    state                 text    NOT NULL CHECK (state IN ('not_started','in_progress','completed')),
    dwell_ms_total        bigint  NOT NULL DEFAULT 0,
    last_position_seconds numeric NOT NULL DEFAULT 0,
    max_position_seconds  numeric NOT NULL DEFAULT 0,
    opened_at             timestamptz,
    completed_at          timestamptz,
    updated_at            timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (enrollment_id, resource_stable_id)
);

CREATE TABLE progress.progress_events (
    id                 bigserial   PRIMARY KEY,
    enrollment_id      uuid        NOT NULL REFERENCES progress.enrollments (id) ON DELETE CASCADE,
    resource_stable_id uuid        NOT NULL,
    kind               text        NOT NULL CHECK (kind IN ('open','heartbeat','close')),
    position_seconds   numeric,
    server_ts          timestamptz NOT NULL DEFAULT now(),
    client_ts          timestamptz,
    accepted           boolean     NOT NULL,
    reject_reason      text
);

-- Las evidencias rechazadas también se conservan: son la prueba de CA-05.
CREATE INDEX progress_events_idx
    ON progress.progress_events (enrollment_id, resource_stable_id, server_ts DESC);

-- ----------------------------------------------------------------- badges --

CREATE TABLE badges.badges (
    id                uuid        PRIMARY KEY,
    -- Una insignia por inscripción. La restricción está aquí además del
    -- reclamo idempotente del trabajo: la segunda barrera es la que hace que
    -- la primera no tenga que ser perfecta (ADR-0008, CA-07).
    enrollment_id     uuid        NOT NULL UNIQUE REFERENCES progress.enrollments (id) ON DELETE CASCADE,
    user_id           uuid        NOT NULL REFERENCES identity.users (id) ON DELETE CASCADE,
    course_id         uuid        NOT NULL REFERENCES authoring.courses (id) ON DELETE CASCADE,
    course_version_id uuid        NOT NULL REFERENCES authoring.course_versions (id),
    public_code       text        NOT NULL UNIQUE,
    image_key         text        NOT NULL,
    issued_at         timestamptz NOT NULL DEFAULT now(),
    revoked_at        timestamptz,
    revoked_by        uuid        REFERENCES identity.users (id),
    revocation_reason text
);

CREATE INDEX badges_user_idx ON badges.badges (user_id, issued_at DESC);
