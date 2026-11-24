-- Dashboard 公告确认记录（portal 自有）：每用户一行，记录其最后确认的公告 id。
-- 公告内容本体存 als_portal_settings（见 0031，KV 免重启），id 取内容哈希——
-- admin 修改公告文案即换 id，旧确认不再匹配，所有用户（含已确认过的）重新弹窗。
CREATE TABLE IF NOT EXISTS als_dashboard_notice_acks (
    user_id   BIGINT PRIMARY KEY REFERENCES als_users(id) ON DELETE CASCADE,
    notice_id TEXT NOT NULL,
    acked_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
