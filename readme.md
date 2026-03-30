## AI API Portal Scaffold

This repository now contains:

- `frontend/` — Next.js (App Router + TypeScript) scaffold
- `backend/` — Go HTTP service scaffold

## Environment Setup

```bash
cp frontend/.env.example frontend/.env.local
cp backend/.env.example backend/.env
```

Backend config now supports both SQLite and PostgreSQL. The simplest local defaults remain SQLite:

```bash
DB_DRIVER=sqlite
DB_PATH=./data.db
```

For PostgreSQL, set:

```bash
DB_DRIVER=postgres
DB_DSN=postgres://user:password@127.0.0.1:5432/ai_api_portal?sslmode=disable
```

## Run Locally

Terminal 1 (frontend):

```bash
npm -C frontend run dev
```

Terminal 2 (backend):

```bash
go run ./backend
```

## Verification Commands

```bash
npm -C frontend run build
go test ./...    # run from backend/
```

Repository verification helper:

```bash
scripts/verify.sh backend-sqlite
scripts/verify.sh backend-postgres   # requires DB_DSN
scripts/verify.sh frontend-build
```

See `docs/admin-package-rollout.md` for rollout order, dual-engine verification, and package/fulfillment operational checks.

Health check once backend is running:

```bash
curl http://localhost:8080/healthz
```

## Backend Auth (current minimal scheme)

- Authenticated endpoints use `X-User-Id: <numeric-user-id>` request header.
- Create users with `POST /users` (role defaults to `user`, can be `admin`).
- API key endpoints:
  - `POST /api-keys` issues a new key for the authenticated user and returns plaintext key once.
  - `DELETE /api-keys/{id}` revokes a key owned by the user (admins can revoke any key).
