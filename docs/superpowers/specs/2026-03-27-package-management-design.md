# Package Management Enhancement

## Overview

Extend the admin packages feature to support pricing, value types (subscription days or balance credit), description, features list, and visibility toggle. Replace the hardcoded pricing plans on the services page with dynamic data from the backend.

## Background

The current `tiers` table has only `code`, `name`, and timestamps. The admin packages page (`/admin/packages`) lets admins create tiers with code/name and bind them to Sub2API groups. The services page (`/services`) has hardcoded pricing plans. This design extends tiers to be a full "package" concept.

## Decision: Extend tiers table (Approach A)

Add new columns to the existing `tiers` table rather than creating a new table. This minimizes code changes since `tiers` already represents "packages" throughout the codebase.

## Database Migration

New migration `0010_add_package_fields_to_tiers.sql` for both SQLite and PostgreSQL:

```sql
ALTER TABLE tiers ADD COLUMN price_micros INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tiers ADD COLUMN value_type TEXT NOT NULL DEFAULT '';
ALTER TABLE tiers ADD COLUMN value_amount INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tiers ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE tiers ADD COLUMN features_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE tiers ADD COLUMN is_enabled INTEGER NOT NULL DEFAULT 1;
```

### Field definitions

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `price_micros` | INTEGER | 0 | User payment price in CNY micros (e.g., 29900000 = 29.90 CNY) |
| `value_type` | TEXT | '' | `"days"` (subscription days) or `"balance"` (credit) or `""` (group-only) |
| `value_amount` | INTEGER | 0 | Days count or balance amount in micros |
| `description` | TEXT | '' | Package description |
| `features_json` | TEXT | '[]' | JSON array of feature strings, e.g., `["10 Global Nodes","500GB Traffic"]` |
| `is_enabled` | INTEGER | 1 | 1 = visible to users, 0 = hidden |

## Backend Changes

### Model update (`model/entities.go`)

Add new fields to `Tier` struct:
```go
type Tier struct {
    ID           int64
    Code         string
    Name         string
    PriceMicros  int64
    ValueType    string
    ValueAmount  int64
    Description  string
    FeaturesJSON string
    IsEnabled    bool
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### API request/response structs

**`adminPackageRequest`:**
- `code` (string, optional on update)
- `name` (string, required)
- `group_codes` ([]string, required)
- `price_micros` (int64, required)
- `value_type` (string, required, one of: `""`, `"days"`, `"balance"`)
- `value_amount` (int64, required, >0 when value_type is non-empty)
- `description` (string)
- `features_json` (string, valid JSON array of strings)
- `is_enabled` (*bool, pointer to distinguish unset from false)

**`adminPackageResponse`:**
- `code`, `name`, `group_codes` (existing)
- `price_micros`, `value_type`, `value_amount`, `description` (new)
- `features` ([]string, parsed from features_json)
- `is_enabled` (bool)
- `created_at`, `updated_at` (existing)

### New public API

**`GET /packages`** — no auth required, returns only `is_enabled=1` packages.

Response:
```json
{
  "packages": [
    {
      "code": "free",
      "name": "Free",
      "price_micros": 0,
      "value_type": "days",
      "value_amount": 30,
      "description": "Perfect for exploring...",
      "features": ["2 Global Nodes", "50GB Monthly Traffic"],
      "is_enabled": true
    }
  ]
}
```

### Handler changes

- `handleAdminCreatePackage` — INSERT with all new fields
- `handleAdminUpdatePackage` — UPDATE new fields (partial update)
- `listAdminPackages` — SELECT new columns, parse features_json
- New `handlePublicListPackages` — SELECT only enabled rows

### Validation rules

- `value_type` must be `""`, `"days"`, or `"balance"`
- If `value_type` is non-empty, `value_amount` must be > 0
- `price_micros` must be >= 0
- `features_json` must be a valid JSON array of strings
- `is_enabled` defaults to `true` on create

## Frontend Changes

### Admin packages page (`/admin/packages`)

Form fields (in order):
1. Package code (existing, read-only in edit)
2. Package name (existing)
3. Value type — radio/select: "none", "subscription days", "balance credit"
4. Value amount — number input (conditionally shown), label based on type
5. Price (CNY yuan) — number input (frontend converts to/from micros)
6. Description — textarea
7. Features — dynamic add/remove text lines
8. Is enabled — toggle switch
9. Bound groups (existing checkbox list)

Package list table — add columns: Price, Value type, Enabled toggle.

### Frontend proxy routes

- Update `/api/admin/packages` POST/PUT to pass new fields
- Update `/api/admin/packages/[code]` GET to return new fields
- New `/api/packages` GET route for public packages

### Services page (`/services`)

Replace hardcoded `pricingPlans` array with data from public packages API. Keep the same visual design. Display format:
- Price: `price_micros / 1000000` as CNY (e.g., ¥29.90)
- Value: if days, show "30 天"; if balance, show "¥50.00"

## Files to modify

### Backend
- `backend/internal/model/entities.go` — update Tier struct
- `backend/internal/httpapi/routes.go` — update request/response structs, handlers, add public endpoint
- `backend/migrations/sqlite/0010_add_package_fields_to_tiers.sql` — new migration
- `backend/migrations/postgres/0010_add_package_fields_to_tiers.sql` — new migration
- `backend/migrations/embed.go` — if needed for embed directives

### Frontend
- `frontend/app/admin/packages/page.tsx` — extend form and table
- `frontend/app/api/admin/packages/route.ts` — proxy updates (already passes through)
- `frontend/app/api/admin/packages/[code]/route.ts` — proxy updates
- `frontend/app/api/packages/route.ts` — new public packages proxy
- `frontend/app/services/page.tsx` — dynamic packages from API
