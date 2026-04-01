-- Download Center: software download entries with version tracking

CREATE TABLE IF NOT EXISTS als_downloads (
    id BIGSERIAL PRIMARY KEY,
    software_name TEXT NOT NULL,
    platform TEXT NOT NULL,
    file_type TEXT NOT NULL,
    download_url TEXT NOT NULL,
    version TEXT NOT NULL,
    force_update BOOLEAN NOT NULL DEFAULT FALSE,
    changelog TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_downloads_software ON als_downloads(software_name, platform);
CREATE INDEX IF NOT EXISTS idx_downloads_platform ON als_downloads(platform);
