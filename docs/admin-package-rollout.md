# Admin package rollout and verification

## Scope covered

This repository now supports:

- admin package CRUD on top of existing `tiers`
- package-to-group bindings stored in `tier_group_bindings`
- server-side filtering of user-visible groups and API keys by active package authorization
- payment-success fulfillment with retryable state, replay controls, and observability
- dual-stack database boot for `sqlite` and `postgres`

## Runtime configuration

### Backend

Use `backend/config.example.yaml` or `backend/.env.example` as the source of truth.

Important fields:

- `database.driver`: `sqlite` or `postgres`
- `database.path`: SQLite file path
- `database.dsn`: PostgreSQL DSN or optional SQLite DSN override
- `sub2api_base_url`: upstream Sub2API base URL
- `sub2api_admin_key`: admin key used for admin-side upstream calls
- `auth.admin_bootstrap_secret`: bootstrap secret for creating the first admin

### Frontend

The frontend still builds as a standard Next.js app and talks to the backend through `/api/...` proxy routes.

## Verification commands

### Fast local backend check (SQLite)

```bash
scripts/verify.sh backend-sqlite
```

Equivalent manual command:

```bash
cd backend && DB_DRIVER=sqlite go test ./...
```

### PostgreSQL verification

Requires a reachable PostgreSQL instance and `DB_DSN`.

```bash
export DB_DSN='postgres://user:password@127.0.0.1:5432/ai_api_portal?sslmode=disable'
scripts/verify.sh backend-postgres
```

Equivalent manual command:

```bash
cd backend && DB_DRIVER=postgres DB_DSN="$DB_DSN" go test ./...
```

### Full backend matrix

```bash
export DB_DSN='postgres://user:password@127.0.0.1:5432/ai_api_portal?sslmode=disable'
scripts/verify.sh backend-all
```

### Frontend build

```bash
scripts/verify.sh frontend-build
```

## Rollout order

Use an expand-verify-cutover pattern.

1. **Expand**
   - deploy backend with dual-stack config support
   - keep production on SQLite if that is the current source of truth
   - verify new migrations apply cleanly in a PostgreSQL staging environment
2. **Verify**
   - run `scripts/verify.sh backend-all`
   - confirm admin package CRUD works
   - confirm user `/groups/available` and `/api-keys` responses are package-filtered
   - confirm payment-success replay endpoint only accepts `failed_retryable`
3. **Cut over**
   - set `database.driver=postgres`
   - provide `database.dsn`
   - keep SQLite data file untouched until rollback window closes
4. **Rollback**
   - revert `database.driver` to `sqlite`
   - point back to the last known good SQLite database file
   - do not delete PostgreSQL data until reconciliation is complete

## Operational checks

### Fulfillment

- replay endpoint: `POST /admin/fulfillment/jobs/{id}/replay`
- only jobs in `failed_retryable` should be accepted
- fulfillment state is independent from payment receipt

### Package authorization

- admin package editor uses:
  - `GET /admin/packages`
  - `POST /admin/packages`
  - `GET /admin/packages/{code}`
  - `PUT /admin/packages/{code}`
  - `GET /admin/groups/available`
- user-visible filtering applies to:
  - `GET /groups/available`
  - `GET /api-keys`
  - `GET|PUT|DELETE /api-keys/{id}`

### Known environment caveat

In this workspace, backend verification is runnable, but frontend build/type verification depends on installed Next.js dependencies. If `npm run build` fails with `next: command not found`, install frontend dependencies first.
