CREATE TABLE IF NOT EXISTS als_scan_codes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_code_hash TEXT NOT NULL UNIQUE,
    scan_code_hash   TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'pending',
    user_id          INTEGER,
    session_token_hash TEXT,
    session_token    TEXT,
    init_ip          TEXT,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at       TIMESTAMP NOT NULL,
    scanned_at       TIMESTAMP,
    authorized_at    TIMESTAMP,
    denied_at        TIMESTAMP,
    FOREIGN KEY(user_id) REFERENCES als_users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_scan_codes_device_hash ON als_scan_codes(device_code_hash);
CREATE INDEX IF NOT EXISTS idx_scan_codes_scan_hash   ON als_scan_codes(scan_code_hash);
CREATE INDEX IF NOT EXISTS idx_scan_codes_expires_at  ON als_scan_codes(expires_at);