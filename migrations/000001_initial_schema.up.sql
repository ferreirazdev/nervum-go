-- 000001_initial_schema.up.sql
-- Full initial schema for Nervum. Equivalent to what GORM AutoMigrate produced.
-- From this point forward, all schema changes must be new numbered migration files.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- organizations
CREATE TABLE organizations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    website     TEXT NOT NULL DEFAULT '',
    owner_id    UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);
CREATE INDEX idx_organizations_owner_id  ON organizations (owner_id);
CREATE INDEX idx_organizations_deleted_at ON organizations (deleted_at);

-- users
CREATE TABLE users (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email                TEXT NOT NULL,
    name                 TEXT NOT NULL DEFAULT '',
    role                 TEXT NOT NULL DEFAULT '',
    organization_id      UUID,
    onboarding_completed BOOLEAN NOT NULL DEFAULT false,
    password_hash        TEXT NOT NULL DEFAULT '',
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at           TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_users_email          ON users (email) WHERE deleted_at IS NULL;
CREATE INDEX        idx_users_organization_id ON users (organization_id);
CREATE INDEX        idx_users_deleted_at      ON users (deleted_at);

-- sessions (no updated_at — model does not declare it)
CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_sessions_user_id    ON sessions (user_id);
CREATE INDEX idx_sessions_deleted_at ON sessions (deleted_at);

-- environments (no updated_at — model does not declare it)
CREATE TABLE environments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE INDEX idx_environments_organization_id ON environments (organization_id);
CREATE INDEX idx_environments_deleted_at      ON environments (deleted_at);

-- teams
CREATE TABLE teams (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    name            TEXT NOT NULL,
    icon            TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE INDEX idx_teams_organization_id ON teams (organization_id);
CREATE INDEX idx_teams_deleted_at      ON teams (deleted_at);

-- team_environments (join table, no PK)
CREATE TABLE team_environments (
    team_id        UUID NOT NULL,
    environment_id UUID NOT NULL,
    CONSTRAINT uq_team_environments UNIQUE (team_id, environment_id)
);

-- user_teams
CREATE TABLE user_teams (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL,
    team_id    UUID NOT NULL,
    role       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT uq_user_teams UNIQUE (user_id, team_id)
);
CREATE INDEX idx_user_teams_deleted_at ON user_teams (deleted_at);

-- user_environment_access
CREATE TABLE user_environment_access (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL,
    environment_id UUID NOT NULL,
    role           TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at     TIMESTAMPTZ,
    CONSTRAINT uq_user_environment_access UNIQUE (user_id, environment_id)
);
CREATE INDEX idx_user_environment_access_deleted_at ON user_environment_access (deleted_at);

-- invitations
CREATE TABLE invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token           TEXT NOT NULL,
    email           TEXT NOT NULL,
    organization_id UUID NOT NULL,
    invited_by_id   UUID NOT NULL,
    role            TEXT NOT NULL DEFAULT 'member',
    environment_id  UUID,
    expires_at      TIMESTAMPTZ NOT NULL,
    status          TEXT NOT NULL,
    accepted_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX idx_invitations_token           ON invitations (token) WHERE deleted_at IS NULL;
CREATE INDEX        idx_invitations_organization_id ON invitations (organization_id);
CREATE INDEX        idx_invitations_deleted_at      ON invitations (deleted_at);

-- invitation_teams (join table, no PK)
CREATE TABLE invitation_teams (
    invitation_id UUID NOT NULL,
    team_id       UUID NOT NULL,
    CONSTRAINT uq_invitation_teams UNIQUE (invitation_id, team_id)
);

-- entities
CREATE TABLE entities (
    id                           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id              UUID NOT NULL,
    environment_id               UUID NOT NULL,
    type                         TEXT NOT NULL,
    name                         TEXT NOT NULL,
    status                       TEXT NOT NULL DEFAULT '',
    owner_team_id                UUID,
    metadata                     JSONB,
    health_check_url             TEXT NOT NULL DEFAULT '',
    health_check_method          TEXT NOT NULL DEFAULT '',
    health_check_headers         JSONB,
    health_check_expected_status INT NOT NULL DEFAULT 0,
    created_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at                   TIMESTAMPTZ
);
CREATE INDEX idx_entities_organization_id ON entities (organization_id);
CREATE INDEX idx_entities_environment_id  ON entities (environment_id);
CREATE INDEX idx_entities_deleted_at      ON entities (deleted_at);

-- relationships (no updated_at — model does not declare it)
CREATE TABLE relationships (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    from_entity_id  UUID NOT NULL,
    to_entity_id    UUID NOT NULL,
    type            TEXT NOT NULL,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE INDEX idx_relationships_organization_id ON relationships (organization_id);
CREATE INDEX idx_relationships_from_entity_id  ON relationships (from_entity_id);
CREATE INDEX idx_relationships_to_entity_id    ON relationships (to_entity_id);
CREATE INDEX idx_relationships_deleted_at      ON relationships (deleted_at);

-- integrations
CREATE TABLE integrations (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id         UUID NOT NULL,
    provider                TEXT NOT NULL,
    access_token            TEXT NOT NULL DEFAULT '',
    refresh_token           TEXT NOT NULL DEFAULT '',
    access_token_expires_at TIMESTAMPTZ NOT NULL DEFAULT '1970-01-01 00:00:00+00',
    scopes                  TEXT NOT NULL DEFAULT '',
    connected_at            TIMESTAMPTZ NOT NULL,
    metadata                JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_integrations_org_provider UNIQUE (organization_id, provider)
);

-- organization_repositories
CREATE TABLE organization_repositories (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    provider        TEXT NOT NULL,
    full_name       TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_org_repos UNIQUE (organization_id, provider, full_name)
);

-- organization_services
CREATE TABLE organization_services (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    provider        TEXT NOT NULL,
    kind            TEXT NOT NULL DEFAULT 'cloud_run',
    service_name    TEXT NOT NULL,
    location        TEXT NOT NULL DEFAULT '',
    instance_type   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_org_svcs UNIQUE (organization_id, provider, kind, service_name)
);
