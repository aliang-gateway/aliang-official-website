-- Service directions: services timeline items (research + done), bilingual zh/en.
-- Backs the /services marketing page; managed via /admin/services.

CREATE TABLE IF NOT EXISTS als_service_directions (
    id BIGSERIAL PRIMARY KEY,
    status TEXT NOT NULL CHECK(status IN ('research','done')),
    phase_zh TEXT NOT NULL,
    phase_en TEXT NOT NULL,
    title_zh TEXT NOT NULL,
    title_en TEXT NOT NULL,
    desc_zh TEXT NOT NULL,
    desc_en TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0,
    is_published BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_service_directions_status ON als_service_directions(status);
CREATE INDEX IF NOT EXISTS idx_service_directions_sort ON als_service_directions(sort_order);

-- Seed the 6 existing items (migrated from editorial.services.items i18n).
INSERT INTO als_service_directions (status, phase_zh, phase_en, title_zh, title_en, desc_zh, desc_en, sort_order, is_published) VALUES
('research', '最新 01 / 研究中', 'Latest 01 / In research', '多 AI 会话并发协同', 'Multi-AI concurrent collaboration', '研究多个 AI 会话并行处理同一工程任务时，如何分工、互相校验、合并结论，并保持上下文不互相污染。', 'Researching how multiple AI sessions divide work, cross-check, and merge conclusions on the same engineering task — without polluting each other''s context.', 1, TRUE),
('research', '最新 02 / 研究中', 'Latest 02 / In research', '状态同步', 'State sync', '研究桌面、移动端和云端会话之间的任务状态同步，确保用户从任意设备回到同一个 coding 现场。', 'Researching task-state sync across desktop, mobile, and cloud sessions so users return to the same coding scene from any device.', 2, TRUE),
('done', '已完成 04 / 最近落地', 'Shipped 04 / Recent launch', '手机 vibecoding', 'Mobile vibecoding', '把 coding 任务从桌面延伸到手机端，让用户可以远程发起、跟进、整理和继续推进开发任务。', 'Extend coding tasks from desktop to phone — remotely start, track, organize, and continue dev tasks.', 3, TRUE),
('done', '已完成 03 / 稳定交付', 'Shipped 03 / Stable delivery', 'API 中转站', 'API proxy hub', '把 OpenAI、Anthropic 等主流格式整理成更稳定的调用入口，服务开发者工具、自动化脚本和内部产品接入。', 'Organize OpenAI, Anthropic, and other mainstream formats into a more stable call entry for dev tools, automation scripts, and internal product integration.', 4, TRUE),
('done', '已完成 02 / 稳定交付', 'Shipped 02 / Stable delivery', 'VSCode Copilot 破解服务', 'VSCode Copilot setup service', '针对 VSCode 与 Copilot 工作流做环境整理、使用路径配置和开发场景适配，降低团队上手成本。', 'Environment prep, usage-path config, and dev-scenario adaptation for VSCode and Copilot workflows — lower team onboarding cost.', 5, TRUE),
('done', '已完成 01 / 最早基础', 'Shipped 01 / Earliest foundation', 'Cursor Pro 破解服务', 'Cursor Pro setup service', '围绕 Cursor Pro 使用场景提供接入、配置和维护支持，让开发者把 AI coding 能力放进真实项目节奏。', 'Onboarding, configuration, and maintenance for Cursor Pro scenarios — put AI coding capability into real project cadence.', 6, TRUE);
