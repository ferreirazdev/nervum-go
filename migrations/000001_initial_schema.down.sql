-- 000001_initial_schema.down.sql
-- Drops all tables created in the up migration, in reverse dependency order.

DROP TABLE IF EXISTS organization_services;
DROP TABLE IF EXISTS organization_repositories;
DROP TABLE IF EXISTS integrations;
DROP TABLE IF EXISTS relationships;
DROP TABLE IF EXISTS entities;
DROP TABLE IF EXISTS invitation_teams;
DROP TABLE IF EXISTS invitations;
DROP TABLE IF EXISTS user_environment_access;
DROP TABLE IF EXISTS user_teams;
DROP TABLE IF EXISTS team_environments;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS environments;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;
