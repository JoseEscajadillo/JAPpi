-- Esquema del control-plane. Cada servicio es dueño de su esquema: ningún
-- otro servicio lee ni escribe estas tablas (se enteran por eventos).

CREATE SCHEMA IF NOT EXISTS control_plane;
SET search_path TO control_plane;

CREATE TABLE accounts (
    id          TEXT PRIMARY KEY,
    github_id   BIGINT UNIQUE NOT NULL,
    email       TEXT NOT NULL,
    plan        TEXT NOT NULL DEFAULT 'trial',        -- lo actualiza subscription.changed
    plan_status TEXT NOT NULL DEFAULT 'trialing',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE projects (
    id              TEXT PRIMARY KEY,
    account_id      TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    slug            TEXT NOT NULL,
    repository      TEXT NOT NULL,                    -- owner/name
    branch          TEXT NOT NULL DEFAULT 'main',
    installation_id BIGINT,
    plan_json       JSONB NOT NULL,                   -- stack.Plan detectado (ADR-0008)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, slug)
);
CREATE INDEX projects_repo_branch ON projects (repository, branch);

CREATE TABLE services (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    path       TEXT NOT NULL,
    role       TEXT NOT NULL CHECK (role IN ('frontend', 'backend')),
    -- ADR-0009: se reserva al crear el proyecto y no cambia al renombrar.
    domain     TEXT NOT NULL UNIQUE,
    UNIQUE (project_id, name)
);

-- Variables definidas por el usuario. Las inyectadas por JAPpi no se guardan:
-- se recalculan con wiring.Resolve.
CREATE TABLE env_vars (
    service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    -- TODO(equipo): cifrar en reposo (pgcrypto o KMS) antes de la fase 2.
    value      TEXT NOT NULL,
    build_time BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (service_id, name)
);

CREATE TABLE deployments (
    id         TEXT PRIMARY KEY,                      -- deployment.IDFor: determinista
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    service_id TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    commit_sha TEXT NOT NULL,
    image      TEXT,                                  -- registry/repo@sha256:...
    status     TEXT NOT NULL CHECK (status IN
                 ('queued','building','deploying','healthy','failed','superseded','rolled_back')),
    detail     TEXT,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX deployments_service_created ON deployments (service_id, created_at DESC);

CREATE TABLE addons (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('postgres', 'redis')),
    -- Nombre del Secret de Kubernetes con las credenciales; nunca las credenciales.
    secret_ref TEXT NOT NULL,
    status     TEXT NOT NULL,
    UNIQUE (project_id, kind)
);
