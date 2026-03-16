# Issues (append-only)

- 2026-03-15: Port `8080` was already in use during local run verification in this environment; workaround for smoke check was `PORT=18080 go run .`.
- 2026-03-15: Additional local port conflicts observed on `18080` and `18081` during backend runtime smoke checks; reliable workaround is selecting a dynamic free port before `go run .`.

- 2026-03-15: Local smoke tests can fail with misleading 404s when prior `go run` processes still hold requested ports; always clear listeners (`lsof -tiTCP:<port> | xargs -r kill`) before endpoint verification.
- 2026-03-15: LSP diagnostics initially reported `go list` workspace warnings for backend files; resolved by activating project and validating changed files with focused `lsp_diagnostics` (error severity) plus `go test ./...` in `backend/`.
- 2026-03-15: `lsp_diagnostics` can emit non-blocking `go list` workspace warnings in this environment even when files are valid; use severity `error` to confirm clean changed-file diagnostics, then rely on `go test ./...` for build/test verification.
- 2026-03-15: Admin unit price `DELETE` endpoint currently returns 404 when no active row exists for the requested `(service_item_code, optional tier_code)` scope; callers should treat it as expected idempotency feedback rather than transport failure.
- 2026-03-15: Frontend public proxy handlers hard-depend on `NEXT_PUBLIC_API_BASE_URL`; when unset they return 500 JSON error (`NEXT_PUBLIC_API_BASE_URL is not set`) and pricing page cannot load tiers/estimates.
- 2026-03-15: Enabling `@next/mdx` on Next 16 required explicitly installing `@mdx-js/loader`; without it, `next build` fails for `.mdx` routes with `Cannot find module '@mdx-js/loader'`.
