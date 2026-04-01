-- Download Center: software download entries with version tracking

CREATE TABLE IF NOT EXISTS als_downloads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    software_name TEXT NOT NULL,
    platform TEXT NOT NULL,
    file_type TEXT NOT NULL,
    download_url TEXT NOT NULL,
    version TEXT NOT NULL,
    force_update INTEGER NOT NULL DEFAULT 0,
    changelog TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_downloads_software ON als_downloads(software_name, platform);
CREATE INDEX IF NOT EXISTS idx_downloads_platform ON als_downloads(platform);
