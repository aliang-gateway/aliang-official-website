-- 注册邮箱验证码（portal 自有，不依赖 sub2api）
-- als_portal_settings: 运行时开关的通用存储（admin 可切换，免重启）
CREATE TABLE IF NOT EXISTS als_portal_settings (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 注册前邮箱验证码：邮箱为键（每邮箱同时只有一个有效码，重发覆盖旧码），
-- attempts 防拖库爆破（连错 5 次作废须重发），一次性消费。
CREATE TABLE IF NOT EXISTS als_register_email_codes (
    email      TEXT PRIMARY KEY,
    code       TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at    TIMESTAMP,
    attempts   INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
