# Database Migrations

Nervum uses [golang-migrate](https://github.com/golang-migrate/migrate) for schema management.

Migration files live in `migrations/` at the repository root and are **embedded into the binary** at compile time — no external files are needed at runtime.

The API server applies pending migrations automatically on every startup. The `cmd/migrate` CLI is available for manual control (rollbacks, version checks, recovery).

---

## How it works

1. On startup, `cmd/api` calls `database.RunMigrations` which applies any `.up.sql` files not yet recorded in the `schema_migrations` table.
2. golang-migrate tracks applied migrations in a `schema_migrations` table it manages automatically.
3. Unit tests (SQLite) continue to use GORM `AutoMigrate` and are unaffected by this system.

---

## File naming

```
migrations/
  000001_initial_schema.up.sql
  000001_initial_schema.down.sql
  000002_add_something.up.sql
  000002_add_something.down.sql
```

Rules:
- **Zero-padded 6-digit sequence** numbers (`000001`, `000002`, …)
- **Never edit or delete** an existing migration once it has been applied to any environment
- Always write a new migration to undo or alter past changes

---

## Common commands

```bash
# Apply all pending migrations
make migrate-up

# Roll back the last migration
make migrate-down

# Roll back 3 migrations
make migrate-down N=3

# Check current schema version
make migrate-version

# Force version (dirty state recovery — see below)
make migrate-force V=1

# Create a new migration stub (prompts for name)
make migrate-create
```

Or run the CLI directly:

```bash
go run ./cmd/migrate up
go run ./cmd/migrate down 2
go run ./cmd/migrate version
go run ./cmd/migrate force 1
```

---

## Adding a new migration

1. Run `make migrate-create` and enter a descriptive `snake_case` name (e.g. `add_sentry_provider`).
2. Write the forward SQL in the generated `.up.sql` file.
3. Write the exact reverse SQL in the `.down.sql` file.
4. Test locally:
   ```bash
   make migrate-up
   make migrate-down   # verify rollback works
   make migrate-up     # re-apply
   ```
5. Commit **both files** in the same PR as the Go code that requires the schema change.

---

## Transitioning an existing database

If the database already has tables created by GORM `AutoMigrate` (before golang-migrate was introduced), skip migration `000001` by baselining:

```sql
-- Run this once against the existing database to mark migration 1 as applied
-- without executing the SQL (tables already exist).
CREATE TABLE IF NOT EXISTS schema_migrations (version bigint NOT NULL, dirty boolean NOT NULL);
INSERT INTO schema_migrations (version, dirty) VALUES (1, false)
ON CONFLICT DO NOTHING;
```

For a **fresh database** (dev/CI/staging), just start the API or run `make migrate-up` — the full schema will be created from scratch.

---

## Dirty state recovery

If the API crashes mid-migration, the `schema_migrations` table is left with `dirty = true`. The app will refuse to start until this is resolved.

1. Identify the failing version: `make migrate-version`
2. Fix the SQL in the migration file (if needed)
3. Force the version back to the one before the failed migration:
   ```bash
   make migrate-force V=<previous-version>
   ```
4. Re-apply: `make migrate-up`

---

## CI integration

```yaml
# Example GitHub Actions step
- name: Run migrations
  run: make migrate-up
  env:
    DB_HOST: localhost
    DB_PORT: 5432
    DB_USER: postgres
    DB_PASSWORD: postgres
    DB_NAME: nervum
```

Add this step after the Postgres service is healthy and before running integration tests (`make test-integration`).
