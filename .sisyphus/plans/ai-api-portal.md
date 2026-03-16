# Work Plan: AI API Portal (Next.js + Go)

Goal: Build an AI API access website with Home, Pricing/Plans, Announcements, and Docs pages (Next.js frontend) plus a Go backend providing plan/pricing/admin endpoints and API key/quota enforcement.

## Implementation Tasks

- [x] T1. Scaffold projects and dev environment (Next.js app + Go service) with consistent env configuration and local run scripts.
- [x] T2. Backend: define data model + persistence (users, api keys, tiers, service items, tier defaults, subscription, overrides, unit prices, usage records) and migrations.
- [x] T3. Backend: auth + roles (user/admin) and API key issuance/revocation.
- [x] T4. Backend: public endpoints for tiers listing and price estimation (unit prices x quotas).
- [x] T5. Backend: user subscription endpoints (choose tier, override quotas, view effective quotas).
- [x] T6. Backend: admin unit price management endpoints (CRUD/update per service item) + basic guardrails.
- [x] T7. Backend: API key middleware + minimal quota/usage enforcement on a sample protected endpoint (e.g. /api/ai/proxy or /api/ai/request).
- [x] T8. Frontend: implement pages (Home, Pricing, Announcements, Docs) and wire to backend public APIs.
- [x] T9. Frontend: minimal auth UX (login) + API key management + subscription selection with per-item quota overrides.
- [x] T10. Docs/Announcements content pipeline (MD/MDX) and basic SEO metadata.

## Final Verification Wave (Approval Gates)

- [x] F1. Reviewer: Architecture & scope check — no unplanned features, endpoints and pages match requirements.
- [x] F2. Reviewer: Build/test check — frontend and backend build clean, lint/typecheck passes, and basic integration smoke tests.
- [x] F3. Reviewer: Security check — API key auth, admin guardrails, secrets handling, no obvious auth bypass.
- [x] F4. Reviewer: UX/content check — pages render, navigation works, docs/announcements accessible, no console errors.

## Acceptance Criteria

1) Visitors can access Home, Pricing, Announcements, Docs pages.
2) Pricing shows tiers (Basic/Standard/Premium) with default service items (name + quota) and allows user override (quota editable) and price estimation using admin-defined unit prices.
3) Backend exposes admin endpoints to set unit prices per service item.
4) Backend issues API keys and enforces API-key-auth + basic quota usage on at least one protected endpoint.
