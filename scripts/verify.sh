#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

usage() {
  cat <<'EOF'
Usage: scripts/verify.sh [backend-sqlite|backend-postgres|backend-all|frontend-build|all]

Commands:
  backend-sqlite   Run backend test suite with SQLite configuration.
  backend-postgres Run backend test suite with PostgreSQL driver selection.
  backend-all      Run both backend SQLite and PostgreSQL test passes.
  frontend-build   Run the frontend production build.
  all              Run backend-all, then frontend-build.

Notes:
  - backend-postgres expects DB_DSN to point at a reachable PostgreSQL instance.
  - frontend-build expects frontend dependencies to be installed.
EOF
}

run_backend_sqlite() {
  echo "==> backend/sqlite: go test ./..."
  (
    cd "$BACKEND_DIR"
    DB_DRIVER=sqlite go test ./...
  )
}

run_backend_postgres() {
  if [[ -z "${DB_DSN:-}" ]]; then
    echo "DB_DSN is required for backend-postgres" >&2
    exit 1
  fi

  echo "==> backend/postgres: go test ./..."
  (
    cd "$BACKEND_DIR"
    DB_DRIVER=postgres DB_DSN="$DB_DSN" go test ./...
  )
}

run_frontend_build() {
  echo "==> frontend: npm run build"
  (
    cd "$FRONTEND_DIR"
    npm run build
  )
}

COMMAND="${1:-all}"

case "$COMMAND" in
  backend-sqlite)
    run_backend_sqlite
    ;;
  backend-postgres)
    run_backend_postgres
    ;;
  backend-all)
    run_backend_sqlite
    run_backend_postgres
    ;;
  frontend-build)
    run_frontend_build
    ;;
  all)
    run_backend_sqlite
    run_backend_postgres
    run_frontend_build
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    echo "Unknown command: $COMMAND" >&2
    usage >&2
    exit 1
    ;;
esac
