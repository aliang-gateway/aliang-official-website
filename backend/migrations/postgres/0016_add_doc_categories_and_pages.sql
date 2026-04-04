CREATE TABLE IF NOT EXISTS als_doc_categories (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT,
    icon TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK(status IN ('draft', 'published')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS als_doc_pages (
    id BIGSERIAL PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    category_id BIGINT NOT NULL,
    mdx_body TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL CHECK(status IN ('draft', 'published')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (category_id) REFERENCES als_doc_categories(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_categories_slug ON als_doc_categories(slug);
CREATE INDEX IF NOT EXISTS idx_doc_categories_status ON als_doc_categories(status);
CREATE INDEX IF NOT EXISTS idx_doc_categories_sort_order ON als_doc_categories(sort_order);

CREATE UNIQUE INDEX IF NOT EXISTS idx_doc_pages_slug ON als_doc_pages(slug);
CREATE INDEX IF NOT EXISTS idx_doc_pages_category_id ON als_doc_pages(category_id);
CREATE INDEX IF NOT EXISTS idx_doc_pages_status ON als_doc_pages(status);
CREATE INDEX IF NOT EXISTS idx_doc_pages_sort_order ON als_doc_pages(sort_order);
