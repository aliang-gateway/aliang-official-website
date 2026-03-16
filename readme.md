## AI API Portal Scaffold

This repository now contains:

- `frontend/` — Next.js (App Router + TypeScript) scaffold
- `backend/` — Go HTTP service scaffold

## Environment Setup

```bash
cp frontend/.env.example frontend/.env.local
cp backend/.env.example backend/.env
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
