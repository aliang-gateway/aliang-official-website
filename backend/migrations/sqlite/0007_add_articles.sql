CREATE TABLE IF NOT EXISTS als_articles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    legacy_id INTEGER,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    excerpt TEXT,
    cover_image_url TEXT,
    tag TEXT,
    read_time TEXT,
    author_name TEXT,
    author_avatar_url TEXT,
    author_icon TEXT,
    mdx_body TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('draft', 'published')),
    published_at TIMESTAMP,
    created_by_user_id INTEGER,
    updated_by_user_id INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_slug ON als_articles(slug);
CREATE INDEX IF NOT EXISTS idx_articles_status ON als_articles(status);
CREATE INDEX IF NOT EXISTS idx_articles_published_at ON als_articles(published_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_articles_legacy_id_non_null ON als_articles(legacy_id) WHERE legacy_id IS NOT NULL;
