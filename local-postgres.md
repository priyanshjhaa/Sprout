# Local PostgreSQL Setup

This setup runs PostgreSQL in Docker for local development. The database port is bound to `127.0.0.1`, so it is not exposed on other network interfaces.

## Prerequisites

- Docker Desktop or another Docker installation with Compose support
- Node.js dependencies installed with `npm install`

## 1. Create the untracked environment file

```bash
cp .env.example .env
```

Generate a local password:

```bash
openssl rand -hex 24
```

Replace `replace-with-a-long-random-local-password` in both `POSTGRES_PASSWORD` and `DATABASE_URL` with the generated value. Using the hexadecimal output avoids URL-encoding problems in `DATABASE_URL`.

Never commit `.env`. The repository ignores `.env*` except for `.env.example`.

## 2. Start PostgreSQL

```bash
npm run db:up
```

The first run may download `postgres:16-alpine` and can take a little longer.

## 3. Verify container health

```bash
npm run db:status
```

The `postgres` service should report `healthy`. If it does not, inspect its output:

```bash
npm run db:logs
```

Press `Ctrl+C` to leave the log stream without stopping PostgreSQL.

## 4. Apply the Drizzle migration

```bash
npm run db:migrate
```

Drizzle records applied migrations separately, so running the command again should not recreate existing tables.

## 5. Verify the connection and tables manually

Open PostgreSQL inside the container:

```bash
docker compose exec postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"'
```

At the `psql` prompt, run:

```sql
select current_database(), current_user, version();
\dt
```

The public schema should contain these 12 Sprout tables:

```text
agent_run_events
agent_runs
application_access_grants
application_resources
applications
deployment_stage_events
deployments
environment_variable_metadata
source_connections
users
workspace_memberships
workspaces
```

Confirm that no credential or secret-value columns exist:

```sql
select table_name, column_name
from information_schema.columns
where table_schema = 'public'
  and column_name ~* '(password|token|credential|secret.*value|private_key|api_key|prompt|transcript|payload)';
```

The security query should return zero rows.

Exit PostgreSQL with:

```text
\q
```

## 6. Stop PostgreSQL

```bash
npm run db:down
```

This stops the container but preserves database contents in the `sprout_postgres_data` Docker volume.

## Optional clean reset

Only run this when you intentionally want to delete all local Sprout database data:

```bash
docker compose down --volumes
```

After a reset, start PostgreSQL and run `npm run db:migrate` again.

## Common issues

### Port 5432 is already in use

Change both values in `.env` to the same available port:

```dotenv
POSTGRES_PORT=5433
DATABASE_URL=postgresql://sprout:YOUR_PASSWORD@127.0.0.1:5433/sprout
```

### Authentication fails after changing the password

PostgreSQL initializes its user and password only when the data volume is first created. Either restore the original password or intentionally perform the clean reset above and initialize a new local volume.

### Migration cannot connect

Check `npm run db:status`, confirm the service is healthy, and make sure the username, password, database, and port in `DATABASE_URL` match the Compose settings.
