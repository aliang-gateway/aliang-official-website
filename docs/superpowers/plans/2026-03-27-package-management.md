# Package Management Enhancement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend admin packages with pricing, value types (days/balance), description, features list, and visibility toggle; make the services page dynamic.

**Architecture:** Extend the existing `tiers` table with new columns (price_micros, value_type, value_amount, description, features_json, is_enabled). Update backend CRUD handlers, add a public packages API, update the admin form, and replace hardcoded pricing on the services page.

**Tech Stack:** Go (net/http), SQLite/PostgreSQL, Next.js, Tailwind CSS

---

## File Structure

### Backend files to modify
- `backend/internal/model/entities.go` — add fields to `Tier` struct
- `backend/internal/httpapi/routes.go` — update request/response structs, handlers, queries, validation
- `backend/migrations/sqlite/0010_add_package_fields_to_tiers.sql` — new migration
- `backend/migrations/postgres/0010_add_package_fields_to_tiers.sql` — new migration

### Frontend files to modify
- `frontend/app/admin/packages/page.tsx` — extend form and table with new fields
- `frontend/app/api/packages/route.ts` — new public packages proxy route
- `frontend/app/services/page.tsx` — dynamic packages from API

---

### Task 1: Database Migration

**Files:**
- Create: `backend/migrations/sqlite/0010_add_package_fields_to_tiers.sql`
- Create: `backend/migrations/postgres/0010_add_package_fields_to_tiers.sql`

- [ ] **Step 1: Create the SQLite migration**

```sql
ALTER TABLE tiers ADD COLUMN price_micros INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tiers ADD COLUMN value_type TEXT NOT NULL DEFAULT '';
ALTER TABLE tiers ADD COLUMN value_amount INTEGER NOT NULL DEFAULT 0;
ALTER TABLE tiers ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE tiers ADD COLUMN features_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE tiers ADD COLUMN is_enabled INTEGER NOT NULL DEFAULT 1;
```

- [ ] **Step 2: Create the PostgreSQL migration**

```sql
ALTER TABLE tiers ADD COLUMN price_micros BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tiers ADD COLUMN value_type TEXT NOT NULL DEFAULT '';
ALTER TABLE tiers ADD COLUMN value_amount BIGINT NOT NULL DEFAULT 0;
ALTER TABLE tiers ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE tiers ADD COLUMN features_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE tiers ADD COLUMN is_enabled BOOLEAN NOT NULL DEFAULT TRUE;
```

- [ ] **Step 3: Verify migrations apply cleanly on SQLite**

Run: `cd backend && DB_DRIVER=sqlite go test ./internal/db/ -run TestMigrate -v`

- [ ] **Step 4: Commit**

```bash
git add backend/migrations/sqlite/0010_add_package_fields_to_tiers.sql backend/migrations/postgres/0010_add_package_fields_to_tiers.sql
git commit -m "feat: add package fields migration for tiers table"
```

---

### Task 2: Update Tier Model

**Files:**
- Modify: `backend/internal/model/entities.go:23-29`

- [ ] **Step 1: Add new fields to the Tier struct**

Replace the `Tier` struct at line 23 with:

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

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/model/entities.go
git commit -m "feat: extend Tier model with package fields"
```

---

### Task 3: Update Backend Request/Response Structs

**Files:**
- Modify: `backend/internal/httpapi/routes.go:136-152`

- [ ] **Step 1: Update adminPackageRequest struct**

Replace the `adminPackageRequest` struct (lines 136-140) with:

```go
type adminPackageRequest struct {
	Code          string   `json:"code,omitempty"`
	Name          string   `json:"name"`
	GroupCodes    []string `json:"group_codes"`
	PriceMicros   int64    `json:"price_micros"`
	ValueType     string   `json:"value_type"`
	ValueAmount   int64    `json:"value_amount"`
	Description   string   `json:"description"`
	FeaturesJSON  string   `json:"features_json"`
	IsEnabled     *bool    `json:"is_enabled,omitempty"`
}
```

- [ ] **Step 2: Update adminPackageResponse struct**

Replace the `adminPackageResponse` struct (lines 142-148) with:

```go
type adminPackageResponse struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	GroupCodes    []string `json:"group_codes"`
	PriceMicros   int64    `json:"price_micros"`
	ValueType     string   `json:"value_type"`
	ValueAmount   int64    `json:"value_amount"`
	Description   string   `json:"description"`
	Features      []string `json:"features"`
	IsEnabled     bool     `json:"is_enabled"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}
```

- [ ] **Step 3: Add public package response struct**

Add after the `listAdminPackagesResponse` struct:

```go
type publicPackageResponse struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	PriceMicros   int64    `json:"price_micros"`
	ValueType     string   `json:"value_type"`
	ValueAmount   int64    `json:"value_amount"`
	Description   string   `json:"description"`
	Features      []string `json:"features"`
}

type listPublicPackagesResponse struct {
	Packages []publicPackageResponse `json:"packages"`
}
```

- [ ] **Step 4: Verify compilation**

Run: `cd backend && go build ./...`

Expected: compilation errors in handlers that construct `adminPackageResponse` — that's OK, we fix them in Task 4.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/httpapi/routes.go
git commit -m "feat: update package request/response structs with new fields"
```

---

### Task 4: Update listAdminPackages Query and Response Builder

**Files:**
- Modify: `backend/internal/httpapi/routes.go:3715-3773` (`listAdminPackages` function)

- [ ] **Step 1: Update the SELECT query and Scan to include new columns**

Replace the entire `listAdminPackages` function (lines 3715-3773) with:

```go
func (r *routes) listAdminPackages(ctx context.Context) ([]adminPackageResponse, error) {
	const query = `
		SELECT
			t.id,
			t.code,
			t.name,
			t.price_micros,
			t.value_type,
			t.value_amount,
			t.description,
			t.features_json,
			t.is_enabled,
			t.created_at,
			t.updated_at,
			tgb.group_code
		FROM tiers t
		LEFT JOIN tier_group_bindings tgb ON tgb.tier_id = t.id
		ORDER BY t.id ASC, tgb.group_code ASC;
	`

	rows, err := r.db.QueryContext(ctx, db.Rebind(r.sqlDialect, query))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	packages := make([]adminPackageResponse, 0)
	packageIndex := make(map[int64]int)
	for rows.Next() {
		var (
			tierID       int64
			pkgCode      string
			pkgName      string
			priceMicros  int64
			valueType    string
			valueAmount  int64
			description  string
			featuresJSON string
			isEnabled    bool
			createdAt    string
			updatedAt    string
			groupCode    sql.NullString
		)
		if err := rows.Scan(&tierID, &pkgCode, &pkgName, &priceMicros, &valueType, &valueAmount, &description, &featuresJSON, &isEnabled, &createdAt, &updatedAt, &groupCode); err != nil {
			return nil, err
		}

		idx, found := packageIndex[tierID]
		if !found {
			idx = len(packages)
			packageIndex[tierID] = idx
			packages = append(packages, adminPackageResponse{
				Code:         pkgCode,
				Name:         pkgName,
				PriceMicros:  priceMicros,
				ValueType:    valueType,
				ValueAmount:  valueAmount,
				Description:  description,
				Features:     parseFeaturesJSON(featuresJSON),
				IsEnabled:    isEnabled,
				GroupCodes:   []string{},
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
			})
		}

		if groupCode.Valid {
			packages[idx].GroupCodes = append(packages[idx].GroupCodes, groupCode.String)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return packages, nil
}

func parseFeaturesJSON(raw string) []string {
	if raw == "" || raw == "[]" {
		return []string{}
	}
	var features []string
	if err := json.Unmarshal([]byte(raw), &features); err != nil {
		return []string{}
	}
	return features
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/httpapi/routes.go
git commit -m "feat: update listAdminPackages to select new tier columns"
```

---

### Task 5: Update normalizeAdminPackageRequest Validation

**Files:**
- Modify: `backend/internal/httpapi/routes.go:4154-4186` (`normalizeAdminPackageRequest` function)

- [ ] **Step 1: Add validation for new fields**

Replace the entire `normalizeAdminPackageRequest` function (lines 4154-4186) with:

```go
func normalizeAdminPackageRequest(payload adminPackageRequest, requireCode bool) (adminPackageRequest, error) {
	payload.Code = strings.TrimSpace(payload.Code)
	payload.Name = strings.TrimSpace(payload.Name)
	payload.ValueType = strings.TrimSpace(payload.ValueType)
	payload.Description = strings.TrimSpace(payload.Description)
	payload.FeaturesJSON = strings.TrimSpace(payload.FeaturesJSON)

	if requireCode && payload.Code == "" {
		return adminPackageRequest{}, errors.New("code is required")
	}
	if payload.Name == "" {
		return adminPackageRequest{}, errors.New("name is required")
	}
	if len(payload.GroupCodes) == 0 {
		return adminPackageRequest{}, errors.New("group_codes is required")
	}
	if payload.PriceMicros < 0 {
		return adminPackageRequest{}, errors.New("price_micros must be >= 0")
	}

	switch payload.ValueType {
	case "", "days", "balance":
		// valid
	default:
		return adminPackageRequest{}, errors.New("value_type must be empty, 'days', or 'balance'")
	}
	if payload.ValueType != "" && payload.ValueAmount <= 0 {
		return adminPackageRequest{}, errors.New("value_amount must be > 0 when value_type is set")
	}

	if payload.FeaturesJSON != "" && payload.FeaturesJSON != "[]" {
		if !json.Valid([]byte(payload.FeaturesJSON)) {
			return adminPackageRequest{}, errors.New("features_json must be valid JSON")
		}
		var arr []string
		if err := json.Unmarshal([]byte(payload.FeaturesJSON), &arr); err != nil {
			return adminPackageRequest{}, errors.New("features_json must be a JSON array of strings")
		}
	} else {
		payload.FeaturesJSON = "[]"
	}

	normalizedGroups := make([]string, 0, len(payload.GroupCodes))
	seen := make(map[string]struct{}, len(payload.GroupCodes))
	for _, raw := range payload.GroupCodes {
		groupCode := strings.TrimSpace(raw)
		if groupCode == "" {
			return adminPackageRequest{}, errors.New("group_codes must not contain empty values")
		}
		if _, exists := seen[groupCode]; exists {
			continue
		}
		seen[groupCode] = struct{}{}
		normalizedGroups = append(normalizedGroups, groupCode)
	}
	if len(normalizedGroups) == 0 {
		return adminPackageRequest{}, errors.New("group_codes is required")
	}
	sort.Strings(normalizedGroups)
	payload.GroupCodes = normalizedGroups
	return payload, nil
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/httpapi/routes.go
git commit -m "feat: add validation for value_type, features_json, price_micros"
```

---

### Task 6: Update handleAdminCreatePackage

**Files:**
- Modify: `backend/internal/httpapi/routes.go:960-1007` (`handleAdminCreatePackage` function)

- [ ] **Step 1: Update INSERT to include new columns**

Replace the entire `handleAdminCreatePackage` function (lines 960-1007) with:

```go
func (r *routes) handleAdminCreatePackage(w http.ResponseWriter, req *http.Request) {
	var payload adminPackageRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	normalized, err := normalizeAdminPackageRequest(payload, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	isEnabled := true
	if normalized.IsEnabled != nil {
		isEnabled = *normalized.IsEnabled
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create package")
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	tierID, err := db.InsertID(req.Context(), r.sqlDialect, tx, `
		INSERT INTO tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "id", normalized.Code, normalized.Name, normalized.PriceMicros, normalized.ValueType, normalized.ValueAmount, normalized.Description, normalized.FeaturesJSON, isEnabled, now, now)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("failed to create package: %v", err))
		return
	}

	if err := r.replaceTierGroupBindingsTx(req.Context(), tx, tierID, normalized.GroupCodes, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save package groups")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create package")
		return
	}

	pkg, err := r.loadAdminPackageByCode(req.Context(), normalized.Code)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load package")
		return
	}

	writeJSON(w, http.StatusCreated, pkg)
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/httpapi/routes.go
git commit -m "feat: update create package handler with new fields"
```

---

### Task 7: Update handleAdminUpdatePackage

**Files:**
- Modify: `backend/internal/httpapi/routes.go:1009-1079` (`handleAdminUpdatePackage` function)

- [ ] **Step 1: Update UPDATE to include new columns**

Replace the entire `handleAdminUpdatePackage` function (lines 1009-1079) with:

```go
func (r *routes) handleAdminUpdatePackage(w http.ResponseWriter, req *http.Request) {
	packageCode := strings.TrimSpace(req.PathValue("code"))
	if packageCode == "" {
		writeError(w, http.StatusBadRequest, "package code is required")
		return
	}

	var payload adminPackageRequest
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	payload.Code = packageCode

	normalized, err := normalizeAdminPackageRequest(payload, false)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	tierID, err := r.lookupTierID(req.Context(), packageCode)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}

	tx, err := r.db.BeginTx(req.Context(), nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `UPDATE tiers SET name = ?, price_micros = ?, value_type = ?, value_amount = ?, description = ?, features_json = ?, is_enabled = ?, updated_at = ? WHERE id = ?;`), normalized.Name, normalized.PriceMicros, normalized.ValueType, normalized.ValueAmount, normalized.Description, normalized.FeaturesJSON, normalized.IsEnabled, now, tierID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	affected, err := result.RowsAffected()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}
	if affected == 0 {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}

	if err := r.replaceTierGroupBindingsTx(req.Context(), tx, tierID, normalized.GroupCodes, now); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save package groups")
		return
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update package")
		return
	}

	pkg, err := r.loadAdminPackageByCode(req.Context(), packageCode)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load package")
		return
	}

	writeJSON(w, http.StatusOK, pkg)
}
```

Note: The UPDATE for `is_enabled` passes `*bool` directly. SQLite will coerce `false` to 0 and `true` to 1. PostgreSQL will handle `bool` natively.

- [ ] **Step 2: Fix is_enabled serialization for SQLite**

The `*bool` pointer will serialize as JSON `true`/`false` but SQLite stores as 0/1. We need to handle the nil case and ensure the SQL placeholder receives the right value. Add a helper before the handler or inline it:

Actually, `database/sql` maps `bool` → SQLite INTEGER (1/0) and `*bool` → NULL if nil. Since `is_enabled` is NOT NULL DEFAULT 1, we should pass `false` instead of nil when unset. Update the handler to default to `false` when nil — but that would make it impossible to leave `is_enabled` unchanged. The cleanest approach: always include `is_enabled` in the update. The frontend sends the current value.

The current code with `normalized.IsEnabled` being `*bool` passed to ExecContext will work: nil→NULL (SQLite treats as NULL, but column is NOT NULL — this will fail). Fix by defaulting:

Replace the ExecContext line in the handler with:
```go
	isEnabledVal := false
	if normalized.IsEnabled != nil {
		isEnabledVal = *normalized.IsEnabled
	}
	result, err := tx.ExecContext(req.Context(), db.Rebind(r.sqlDialect, `UPDATE tiers SET name = ?, price_micros = ?, value_type = ?, value_amount = ?, description = ?, features_json = ?, is_enabled = ?, updated_at = ? WHERE id = ?;`), normalized.Name, normalized.PriceMicros, normalized.ValueType, normalized.ValueAmount, normalized.Description, normalized.FeaturesJSON, isEnabledVal, now, tierID)
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add backend/internal/httpapi/routes.go
git commit -m "feat: update package handler with new fields and is_enabled fix"
```

---

### Task 8: Add Public Packages API

**Files:**
- Modify: `backend/internal/httpapi/routes.go` (route registration + handler)

- [ ] **Step 1: Register the public route**

Add this line after line 490 (after the `GET /public/articles/{slug}` route), before the subscription routes:

```go
mux.HandleFunc("GET /public/packages", http.HandlerFunc(r.handlePublicListPackages))
```

- [ ] **Step 2: Add the handler function**

Add the `handlePublicListPackages` function near the other list functions (e.g., after `handleAdminGetPackage`):

```go
func (r *routes) handlePublicListPackages(w http.ResponseWriter, req *http.Request) {
	rows, err := r.db.QueryContext(req.Context(), db.Rebind(r.sqlDialect, `
		SELECT code, name, price_micros, value_type, value_amount, description, features_json
		FROM tiers
		WHERE is_enabled = 1
		ORDER BY price_micros ASC;
	`))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}
	defer rows.Close()

	packages := make([]publicPackageResponse, 0)
	for rows.Next() {
		var (
			code         string
			name         string
			priceMicros  int64
			valueType    string
			valueAmount  int64
			description  string
			featuresJSON string
		)
		if err := rows.Scan(&code, &name, &priceMicros, &valueType, &valueAmount, &description, &featuresJSON); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to list packages")
			return
		}
		packages = append(packages, publicPackageResponse{
			Code:         code,
			Name:         name,
			PriceMicros:  priceMicros,
			ValueType:    valueType,
			ValueAmount:  valueAmount,
			Description:  description,
			Features:     parseFeaturesJSON(featuresJSON),
		})
	}

	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}

	writeJSON(w, http.StatusOK, listPublicPackagesResponse{Packages: packages})
}
```

- [ ] **Step 3: Verify compilation**

Run: `cd backend && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add backend/internal/httpapi/routes.go
git commit -m "feat: add public packages API endpoint"
```

---

### Task 9: Backend Tests

**Files:**
- Modify: `backend/internal/httpapi/routes_test.go`

- [ ] **Step 1: Write a test for the admin package CRUD with new fields**

Add a new test function:

```go
func TestAdminPackageCRUDWithNewFields(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	mux := http.NewServeMux()
	RegisterRoutesWithOptions(mux, database, RoutesOptions{AdminBootstrapSecret: "test-admin-secret"})

	// Create admin session
	adminSession := createAdminSession(t, mux, "admin@test.com", "test-admin-secret")

	// Create package with all new fields
	createBody, _ := json.Marshal(map[string]any{
		"code":          "pro-monthly",
		"name":          "Pro Monthly",
		"group_codes":   ["claude-basic", "gpt-4o"],
		"price_micros":  29900000,
		"value_type":    "days",
		"value_amount":  30,
		"description":   "Enhanced speed for dedicated developers.",
		"features_json": `["10 Global Nodes","500GB Monthly Traffic","Priority Email Support"]`,
		"is_enabled":    true,
	})
	req := httptest.NewRequest(http.MethodPost, "/admin/packages", bytes.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminSession)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var created struct {
		Code         string   `json:"code"`
		Name         string   `json:"name"`
		GroupCodes   []string `json:"group_codes"`
		PriceMicros  int64    `json:"price_micros"`
		ValueType    string   `json:"value_type"`
		ValueAmount  int64    `json:"value_amount"`
		Description  string   `json:"description"`
		Features     []string `json:"features"`
		IsEnabled    bool     `json:"is_enabled"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Code != "pro-monthly" {
		t.Fatalf("expected code pro-monthly, got %s", created.Code)
	}
	if created.PriceMicros != 29900000 {
		t.Fatalf("expected price_micros 29900000, got %d", created.PriceMicros)
	}
	if created.ValueType != "days" {
		t.Fatalf("expected value_type days, got %s", created.ValueType)
	}
	if created.ValueAmount != 30 {
		t.Fatalf("expected value_amount 30, got %d", created.ValueAmount)
	}
	if len(created.Features) != 3 {
		t.Fatalf("expected 3 features, got %d", len(created.Features))
	}
	if !created.IsEnabled {
		t.Fatal("expected is_enabled true")
	}

	// List packages — should include new fields
	req = httptest.NewRequest(http.MethodGet, "/admin/packages", nil)
	req.Header.Set("Authorization", "Bearer "+adminSession)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}

	// Update package
	updateBody, _ := json.Marshal(map[string]any{
		"name":          "Pro Monthly Updated",
		"group_codes":   ["claude-basic"],
		"price_micros":  19900000,
		"value_type":    "days",
		"value_amount":  90,
		"description":   "Better deal.",
		"features_json": `["20 Global Nodes","Unlimited Traffic"]`,
		"is_enabled":    false,
	})
	req = httptest.NewRequest(http.MethodPut, "/admin/packages/pro-monthly", bytes.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminSession)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var updated struct {
		PriceMicros int64    `json:"price_micros"`
		ValueAmount int64    `json:"value_amount"`
		IsEnabled   bool     `json:"is_enabled"`
		Features    []string `json:"features"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.PriceMicros != 19900000 {
		t.Fatalf("expected updated price_micros 19900000, got %d", updated.PriceMicros)
	}
	if updated.IsEnabled {
		t.Fatal("expected is_enabled false after update")
	}
	if len(updated.Features) != 2 {
		t.Fatalf("expected 2 features after update, got %d", len(updated.Features))
	}
}
```

- [ ] **Step 2: Write a test for the public packages endpoint**

```go
func TestPublicPackagesReturnsOnlyEnabled(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	// Insert tiers directly — one enabled, one disabled
	_, err := database.ExecContext(ctx, `INSERT INTO tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled) VALUES ('free', 'Free', 0, 'days', 30, 'Perfect for exploring.', '["2 Global Nodes"]', 1);`)
	if err != nil {
		t.Fatalf("insert free tier: %v", err)
	}
	_, err = database.ExecContext(ctx, `INSERT INTO tiers(code, name, price_micros, value_type, value_amount, description, features_json, is_enabled) VALUES ('hidden', 'Hidden', 99000000, 'days', 365, 'Hidden tier.', '["Everything"]', 0);`)
	if err != nil {
		t.Fatalf("insert hidden tier: %v", err)
	}

	mux := http.NewServeMux()
	RegisterRoutes(mux, database)

	req := httptest.NewRequest(http.MethodGet, "/public/packages", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var payload struct {
		Packages []struct {
			Code string `json:"code"`
		} `json:"packages"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Packages) != 1 {
		t.Fatalf("expected 1 public package, got %d", len(payload.Packages))
	}
	if payload.Packages[0].Code != "free" {
		t.Fatalf("expected package code 'free', got %s", payload.Packages[0].Code)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd backend && DB_DRIVER=sqlite go test ./internal/httpapi/ -run "TestAdminPackageCRUDWithNewFields|TestPublicPackagesReturnsOnlyEnabled" -v`

Expected: Both tests PASS

- [ ] **Step 4: Run all backend tests to check for regressions**

Run: `cd backend && DB_DRIVER=sqlite go test ./...`

- [ ] **Step 5: Commit**

```bash
git add backend/internal/httpapi/routes_test.go
git commit -m "test: add package CRUD and public packages tests"
```

---

### Task 10: Frontend Admin Packages Page Update

**Files:**
- Modify: `frontend/app/admin/packages/page.tsx`

This is the largest task. The admin page needs to:
1. Add new form fields (value_type, value_amount, price, description, features, is_enabled)
2. Update the package list table to show new columns
3. Pass new fields in create/update requests
4. Parse new fields in list/detail responses

- [ ] **Step 1: Update the AdminPackage and PackageFormState types**

Replace the `AdminPackage` type (around line 19-25) with:

```typescript
type AdminPackage = {
  code: string;
  name: string;
  group_codes: string[];
  price_micros: number;
  value_type: string;
  value_amount: number;
  description: string;
  features: string[];
  is_enabled: boolean;
  created_at: string;
  updated_at: string;
};
```

Replace the `PackageFormState` type (around line 35-39) with:

```typescript
type PackageFormState = {
  code: string;
  name: string;
  groupCodes: string[];
  priceMicros: number;
  valueType: string;
  valueAmount: number;
  description: string;
  features: string[];
  isEnabled: boolean;
};
```

Update `defaultFormState`:

```typescript
const defaultFormState: PackageFormState = {
  code: "",
  name: "",
  groupCodes: [],
  priceMicros: 0,
  valueType: "",
  valueAmount: 0,
  description: "",
  features: [],
  isEnabled: true,
};
```

- [ ] **Step 2: Update handleFormChange to handle new fields**

Update the `handleFormChange` function to handle `priceMicros`, `valueType`, `valueAmount`, `description`, and `isEnabled`:

```typescript
const handleFormChange = (key: keyof PackageFormState, value: string | string[] | boolean) => {
    setFormError(null);
    setGlobalSuccess(null);
    setFormState((previous: PackageFormState) => {
      if (key === "code") {
        return { ...previous, code: normalizeCode(String(value)) };
      }
      if (key === "groupCodes") {
        return { ...previous, groupCodes: Array.isArray(value) ? value : previous.groupCodes };
      }
      if (key === "isEnabled") {
        return { ...previous, isEnabled: value as boolean };
      }
      if (key === "priceMicros" || key === "valueAmount") {
        return { ...previous, [key]: Math.max(0, parseInt(String(value), 10) || 0) };
      }
      return { ...previous, [key]: String(value) };
    });
  };
```

- [ ] **Step 3: Add feature management helpers**

Add these functions before the component:

```typescript
function addFeature(features: string[]): string[] {
  return [...features, ""];
}

function updateFeature(features: string[], index: number, value: string): string[] {
  return features.map((f: string, i: number) => (i === index ? value : f));
}

function removeFeature(features: string[], index: number): string[] {
  return features.filter((_: string, i: number) => i !== index);
}
```

- [ ] **Step 4: Update handleEdit to populate new form fields**

In `handleEdit`, update the `setFormState` call to include new fields:

```typescript
setFormState({
  code: pkg.code,
  name: pkg.name,
  groupCodes: Array.isArray(pkg.group_codes) ? pkg.group_codes : [],
  priceMicros: Number(pkg.price_micros) || 0,
  valueType: String(pkg.value_type ?? ""),
  valueAmount: Number(pkg.value_amount) || 0,
  description: String(pkg.description ?? ""),
  features: Array.isArray(pkg.features) ? pkg.features : [],
  isEnabled: pkg.is_enabled !== false,
});
```

- [ ] **Step 5: Update handleCreateOrUpdate payload**

In `handleCreateOrUpdate`, update the payload construction to include new fields:

```typescript
const payload = editingCode
  ? {
      name: trimmedName,
      group_codes: uniqueGroupCodes,
      price_micros: formState.priceMicros,
      value_type: formState.valueType,
      value_amount: formState.valueAmount,
      description: formState.description,
      features_json: JSON.stringify(formState.features.filter((f: string) => f.trim() !== "")),
      is_enabled: formState.isEnabled,
    }
  : {
      code: normalizedCode,
      name: trimmedName,
      group_codes: uniqueGroupCodes,
      price_micros: formState.priceMicros,
      value_type: formState.valueType,
      value_amount: formState.valueAmount,
      description: formState.description,
      features_json: JSON.stringify(formState.features.filter((f: string) => f.trim() !== "")),
      is_enabled: formState.isEnabled,
    };
```

- [ ] **Step 6: Add new form fields to the JSX**

After the "Package name" field and before the "Bound groups" fieldset, add:

```tsx
{/* Value Type */}
<label className="grid gap-1 text-sm text-[var(--portal-muted)]">
  <span>Value Type</span>
  <select
    className="field"
    value={formState.valueType}
    onChange={(event: ChangeEvent<HTMLSelectElement>) => handleFormChange("valueType", event.target.value)}
    disabled={isBlocked || isSubmittingForm}
  >
    <option value="">None (group-only)</option>
    <option value="days">Subscription (days)</option>
    <option value="balance">Balance credit</option>
  </select>
</label>

{/* Value Amount */}
{formState.valueType ? (
  <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
    <span>{formState.valueType === "days" ? "Days" : "Amount (micros)"}</span>
    <input
      className="field"
      type="number"
      min="1"
      value={formState.valueAmount || ""}
      onChange={(event: ChangeEvent<HTMLInputElement>) => handleFormChange("valueAmount", event.target.value)}
      disabled={isBlocked || isSubmittingForm}
      required
    />
  </label>
) : null}

{/* Price */}
<label className="grid gap-1 text-sm text-[var(--portal-muted)]">
  <span>Price (CNY, yuan)</span>
  <input
    className="field"
    type="number"
    min="0"
    step="0.01"
    value={formState.priceMicros ? (formState.priceMicros / 1000000).toString() : ""}
    onChange={(event: ChangeEvent<HTMLInputElement>) => {
      const yuan = parseFloat(event.target.value) || 0;
      handleFormChange("priceMicros", String(Math.round(yuan * 1000000)));
    }}
    disabled={isBlocked || isSubmittingForm}
  />
</label>

{/* Description */}
<label className="grid gap-1 text-sm text-[var(--portal-muted)]">
  <span>Description</span>
  <textarea
    className="field min-h-[80px] resize-y"
    rows={3}
    value={formState.description}
    onChange={(event: ChangeEvent<HTMLTextAreaElement>) => handleFormChange("description", event.target.value)}
    disabled={isBlocked || isSubmittingForm}
  />
</label>

{/* Features */}
<fieldset className="grid gap-3 rounded-xl border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
  <legend className="px-1 text-sm font-semibold text-[var(--portal-ink)]">Features</legend>
  {formState.features.map((feature: string, index: number) => (
    <div key={index} className="flex gap-2">
      <input
        className="field flex-1"
        type="text"
        placeholder={`Feature ${index + 1}`}
        value={feature}
        onChange={(event: ChangeEvent<HTMLInputElement>) =>
          setFormState((prev: PackageFormState) => ({
            ...prev,
            features: updateFeature(prev.features, index, event.target.value),
          }))
        }
        disabled={isBlocked || isSubmittingForm}
      />
      <button
        type="button"
        className="btn-ghost px-2 text-xs text-red-500 hover:text-red-700"
        onClick={() => setFormState((prev: PackageFormState) => ({
          ...prev,
          features: removeFeature(prev.features, index),
        }))}
        disabled={isBlocked || isSubmittingForm}
      >
        Remove
      </button>
    </div>
  ))}
  <button
    type="button"
    className="btn-ghost text-xs"
    onClick={() => setFormState((prev: PackageFormState) => ({
      ...prev,
      features: addFeature(prev.features),
    }))}
    disabled={isBlocked || isSubmittingForm}
  >
    + Add Feature
  </button>
</fieldset>

{/* Is Enabled */}
<label className="flex items-center gap-3 text-sm text-[var(--portal-muted)]">
  <input
    className="size-4 accent-emerald-500"
    type="checkbox"
    checked={formState.isEnabled}
    onChange={(event: ChangeEvent<HTMLInputElement>) => handleFormChange("isEnabled", event.target.checked)}
    disabled={isBlocked || isSubmittingForm}
  />
  <span>Visible to users (enabled)</span>
</label>
```

- [ ] **Step 7: Add new columns to the package list table**

In the table `<thead>`, add after the "Name" column:

```tsx
<th className="px-2 py-1">Price</th>
<th className="px-2 py-1">Value</th>
<th className="px-2 py-1">Enabled</th>
```

In the table `<tbody>`, add after the name `<td>`:

```tsx
<td className="px-2 py-2 text-sm text-[var(--portal-ink)]">
  {pkg.price_micros > 0 ? `¥${(pkg.price_micros / 1000000).toFixed(2)}` : "Free"}
</td>
<td className="px-2 py-2 text-xs text-[var(--portal-muted)]">
  {pkg.value_type
    ? `${pkg.value_type === "days" ? pkg.value_amount + "d" : "¥" + ((pkg.value_amount / 1000000).toFixed(2))}`
    : "-"}
</td>
<td className="px-2 py-2">
  <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${pkg.is_enabled ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300" : "bg-slate-500/10 text-slate-500 dark:text-slate-400"}`}>
    {pkg.is_enabled ? "On" : "Off"}
  </span>
</td>
```

- [ ] **Step 8: Verify frontend build**

Run: `cd frontend && npm run build`

- [ ] **Step 9: Commit**

```bash
git add frontend/app/admin/packages/page.tsx
git commit -m "feat: extend admin packages form and table with pricing, features, visibility"
```

---

### Task 11: Frontend Public Packages Proxy Route

**Files:**
- Create: `frontend/app/api/packages/route.ts`

- [ ] **Step 1: Create the public packages proxy route**

```typescript
import { NextResponse } from "next/server";

function getApiBaseUrl() {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  if (!baseUrl) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return baseUrl.replace(/\/$/, "");
}

export async function GET() {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const upstream = await fetch(`${apiBaseUrl}/public/packages`, {
    method: "GET",
    headers: {
      accept: "application/json",
    },
    cache: "no-store",
  });

  try {
    const payload = await upstream.json();
    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json(
      { error: "invalid json response from upstream" },
      { status: 502 },
    );
  }
}
```

- [ ] **Step 2: Verify frontend build**

Run: `cd frontend && npm run build`

- [ ] **Step 3: Commit**

```bash
git add frontend/app/api/packages/route.ts
git commit -m "feat: add public packages API proxy route"
```

---

### Task 12: Update Services Page to Use Dynamic Packages

**Files:**
- Modify: `frontend/app/services/page.tsx`

- [ ] **Step 1: Convert to client component and fetch packages from API**

Replace the hardcoded `pricingPlans` and the static page export with a client component that fetches from `/api/packages`. The existing `platforms` data and layout stays the same — only the pricing section changes.

Replace the entire file content with:

```tsx
"use client";

import Link from "next/link";
import Image from "next/image";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { useEffect, useState } from "react";

const platforms = [
  {
    name: "macOS",
    icon: "laptop_mac",
    description: "Compatible with Apple Silicon (M1/M2/M3) and Intel processors.",
    downloadExt: ".dmg",
    version: "v2.4.0",
  },
  {
    name: "Windows",
    icon: "window",
    description: "Support for Windows 10 & 11. Available in EXE and MSI installers.",
    downloadExt: ".exe",
    version: "v2.4.0",
  },
  {
    name: "Linux",
    icon: "terminal",
    description: "Universal support via DEB, RPM, and portable AppImage formats.",
    downloadExt: ".deb",
    version: "v2.4.0",
  },
];

type DynamicPackage = {
  code: string;
  name: string;
  price_micros: number;
  value_type: string;
  value_amount: number;
  description: string;
  features: string[];
};

function PlatformIcon({ name }: { name: string }) {
  if (name === "laptop_mac") {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="4" y="5" width="16" height="11" rx="1.5" />
        <path d="M2.5 19h19" />
      </svg>
    );
  }

  if (name === "window") {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="4" width="18" height="16" rx="1.5" />
        <path d="M3 10h18" />
        <path d="M12 10v10" />
      </svg>
    );
  }

  if (name === "terminal") {
    return (
      <svg aria-hidden="true" viewBox="0 0 24 24" className="size-10" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
        <rect x="3" y="4" width="18" height="16" rx="1.5" />
        <path d="M7 9l3 3-3 3" />
        <path d="M12.5 15H17" />
      </svg>
    );
  }

  return <MaterialIcon name={name} size={40} className="text-[var(--stitch-text)] transition-colors group-hover:text-white" />;
}

function formatPrice(priceMicros: number): string {
  if (priceMicros <= 0) return "0";
  return (priceMicros / 1000000).toFixed(priceMicros % 1000000 === 0 ? 0 : 2);
}

function formatValue(pkg: DynamicPackage): string | null {
  if (!pkg.value_type) return null;
  if (pkg.value_type === "days") return `${pkg.value_amount} Days`;
  return `¥${(pkg.value_amount / 1000000).toFixed(2)} Credit`;
}

export default function ServicesPage() {
  const [packages, setPackages] = useState<DynamicPackage[]>([]);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await fetch("/api/packages");
        if (!res.ok) return;
        const data = await res.json();
        if (!cancelled && Array.isArray(data.packages)) {
          setPackages(data.packages);
        }
      } catch {
        // silent — falls back to empty state
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    }
    void load();
    return () => { cancelled = true; };
  }, []);

  return (
    <>
      <section className="relative overflow-hidden py-20 px-6">
        <div className="mx-auto max-w-7xl grid grid-cols-1 lg:grid-cols-2 gap-12 items-center">
          <div className="space-y-8">
            <div className="inline-flex items-center gap-2 rounded-full border border-[var(--stitch-primary)]/20 bg-[var(--stitch-primary)]/10 px-3 py-1 text-xs font-bold uppercase tracking-wider text-[var(--stitch-primary)]">
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75"></span>
                <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--stitch-primary)]"></span>
              </span>
              Now v2.4.0 Available
            </div>
            <h1 className="text-5xl font-black leading-[1.1] tracking-tight text-[var(--stitch-text)] md:text-6xl">
              Powering Your <span className="text-[var(--stitch-primary)]">AI Workflow</span> Everywhere
            </h1>
            <p className="max-w-xl text-lg leading-relaxed text-[var(--stitch-text-muted)]">
              Experience seamless multi-platform availability with ALiang Gateway. High-performance connectivity for your AI models, wherever you are. Unified, secure, and blazingly fast.
            </p>
            <div className="flex flex-wrap gap-4">
              <Link
                href="/register"
                className="rounded-xl bg-[var(--stitch-primary)] px-8 py-4 text-lg font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-all hover:-translate-y-0.5"
              >
                Get Started Free
              </Link>
              <Link
                href="/docs"
                className="rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-8 py-4 text-lg font-bold text-[var(--stitch-text)] transition-all hover:bg-[var(--stitch-bg)]/80"
              >
                View Documentation
              </Link>
            </div>
          </div>
          <div className="relative">
            <div className="aspect-video overflow-hidden rounded-2xl border-4 border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] shadow-2xl">
              <Image
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuBdbfe62AqCJSKa5V7u1se0IJGHIFUWK-fOmLPZ7MMaQwIyWYRTfpjRcDAxXxQoJypZFckiH1wbkf9e0P_UnsH-S1aNF65HAJX77TbNHSYo1hqtEpBgpeKai3qqu6V98jhIvmYZg-uEQ93BsCudtfwvmyYY9jxRYEz0H9HRnj4_jyBfHBIIJcM_2CJrPEDYRjFORR64yGaJNyaPdBEdXLZ-0LPUkAE4o7-ZVKeOOFJvmJnPJd6F3lVt90b2xYE8IZxbTdXtULknYrE"
                alt="Futuristic AI neural network visualization with green accents"
                fill
                className="object-cover"
                unoptimized
              />
            </div>
            <div className="absolute -bottom-6 -left-6 hidden rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] p-6 shadow-xl md:block">
              <div className="flex items-center gap-4">
                <div className="rounded-lg bg-[var(--stitch-primary)]/20 p-3 text-[var(--stitch-primary)]">
                  <MaterialIcon name="speed" size={24} />
                </div>
                <div>
                  <p className="text-xs font-bold uppercase text-[var(--stitch-text-muted)]">Average Latency</p>
                  <p className="text-2xl font-black text-[var(--stitch-text)]">&lt; 15ms</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="bg-[var(--stitch-bg-elevated)] py-20">
        <div className="mx-auto max-w-7xl px-6">
          <div className="mb-16 text-center">
            <h2 className="mb-4 text-3xl font-black text-[var(--stitch-text)]">Choose Your Platform</h2>
            <p className="text-[var(--stitch-text-muted)]">Download the native client for your operating system</p>
          </div>
          <div className="grid grid-cols-1 gap-8 md:grid-cols-3">
            {platforms.map((platform) => (
              <div
                key={platform.name}
                className="group rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-8 text-center transition-all hover:border-[var(--stitch-primary)]"
              >
                <div className="mx-auto mb-6 flex size-20 items-center justify-center rounded-2xl bg-[var(--stitch-bg-elevated)] shadow-sm transition-all group-hover:bg-[var(--stitch-primary)] group-hover:text-white">
                  <PlatformIcon name={platform.icon} />
                </div>
                <h3 className="mb-2 text-xl font-bold text-[var(--stitch-text)]">{platform.name}</h3>
                <p className="mb-6 text-sm text-[var(--stitch-text-muted)]">{platform.description}</p>
                <div className="space-y-2">
                  <button type="button" className="w-full rounded-lg bg-[var(--stitch-text)] py-2 font-medium text-[var(--stitch-bg)] transition-colors hover:bg-[var(--stitch-text)]/80">
                    Download {platform.downloadExt}
                  </button>
                  <p className="text-[10px] font-bold uppercase tracking-widest text-[var(--stitch-text-muted)]">
                    Latest: {platform.version}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Pricing Plans — Dynamic */}
      <section className="bg-[var(--stitch-bg)] py-24 px-6">
        <div className="mx-auto max-w-7xl">
          <div className="mb-16 text-center">
            <h2 className="mb-4 text-4xl font-black text-[var(--stitch-text)]">Flexible Service Plans</h2>
            <p className="text-[var(--stitch-text-muted)]">Scalable solutions for developers, researchers, and enterprises</p>
          </div>
          {isLoading ? (
            <p className="text-center text-[var(--stitch-text-muted)]">Loading plans...</p>
          ) : packages.length === 0 ? (
            <p className="text-center text-[var(--stitch-text-muted)]">No plans available at this time.</p>
          ) : (
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-4">
              {packages.map((pkg) => (
                <div
                  key={pkg.code}
                  className="flex flex-col rounded-2xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-8 shadow-sm transition-all hover:shadow-md"
                >
                  <div className="mb-8">
                    <h3 className="mb-2 text-lg font-bold uppercase tracking-tight text-[var(--stitch-text-muted)]">
                      {pkg.name}
                    </h3>
                    <div className="flex items-baseline gap-1">
                      <span className="text-4xl font-black text-[var(--stitch-text)]">¥{formatPrice(pkg.price_micros)}</span>
                      {pkg.value_type === "days" ? <span className="text-sm text-[var(--stitch-text-muted)]">/ {pkg.value_amount}d</span> : null}
                    </div>
                    {pkg.description ? <p className="mt-4 text-sm text-[var(--stitch-text-muted)]">{pkg.description}</p> : null}
                  </div>
                  <ul className="mb-8 flex-grow space-y-4">
                    {pkg.features.map((feature) => (
                      <li key={feature} className="flex items-center gap-3 text-sm">
                        <MaterialIcon name="check_circle" size={18} className="text-[var(--stitch-primary)]" />
                        {feature}
                      </li>
                    ))}
                  </ul>
                  <button
                    type="button"
                    className="w-full rounded-lg py-3 font-bold transition-colors bg-[var(--stitch-text)] text-[var(--stitch-bg)] hover:bg-[var(--stitch-text)]/80"
                  >
                    {pkg.price_micros > 0 ? "Get Started" : "Current Plan"}
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </section>
    </>
  );
}
```

- [ ] **Step 2: Verify frontend build**

Run: `cd frontend && npm run build`

- [ ] **Step 3: Commit**

```bash
git add frontend/app/services/page.tsx
git commit -m "feat: replace hardcoded pricing with dynamic packages from API"
```

---

### Task 13: Final Verification

- [ ] **Step 1: Run backend tests**

Run: `cd backend && DB_DRIVER=sqlite go test ./...`

Expected: All tests PASS

- [ ] **Step 2: Run frontend build**

Run: `cd frontend && npm run build`

Expected: Build succeeds with no errors

- [ ] **Step 3: Manual smoke test**

1. Start backend: `cd backend && go run .`
2. Start frontend: `cd frontend && npm run dev`
3. Visit `http://localhost:3000/admin/packages` — create a package with all new fields
4. Visit `http://localhost:3000/services` — verify the package appears in pricing
5. Disable the package — verify it disappears from services page

- [ ] **Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "fix: final adjustments from smoke testing"
```
