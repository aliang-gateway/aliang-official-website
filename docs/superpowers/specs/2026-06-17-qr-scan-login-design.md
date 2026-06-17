# 扫码登录（QR Scan-to-Login）

## Overview

在现有密码登录基础上新增「扫码登录」。PC 端（浏览器）展示一个二维码，用户用**已登录的官方移动 App** 扫码并在 App 内确认，即完成 PC 端登录，复用现有 `als_sessions` 机制下发 Bearer session token。

本次交付：**后端全部接口 + 数据库迁移 + PC 端扫码登录前端页面**。App 端的扫码/确认交互由 App 团队自行对接（后端接口已就绪）。

## Background

- 后端（`ai-api-portal/backend`，Go 1.25，sqlite/postgres）已有本地账号体系：`als_users`（email/name/role/password_hash/email_verified）+ `als_sessions`（token_hash/expires_at/revoked_at）。登录 = bcrypt 校验 → 写一行 `als_sessions` → 返回明文 `st_<token>`。
- 认证中间件 `auth.RequireUser` 校验 `Authorization: Bearer <token>` → 查 `als_sessions.token_hash`。
- 前端（Next.js App Router）`/login`、`/register` 两处输密码；`app/api/auth/login/route.ts` 透传到后端 `/auth/login`；token 存 `localStorage["session_token"]`。
- 登录路由另有 upstream passthrough（sub2api `/api/v1/auth/*`），但扫码登录是**本地能力**，sub2api 无对应概念，故走本地 handler。

## Decision: 方案 A — 密钥分离 + 两阶段确认

核心安全设计：**二维码里只放 `scan_code`，PC 取 token 的密钥 `device_code` 永不进二维码**。这样即使二维码被拍照，攻击者也无法轮询盗取刚签发的 token（取 token 要 `device_code`）。

两阶段确认（扫描 → 确认）= 微信/GitHub 级 UX，用户可在手机上看到「即将登录 PC 端」并显式确认。

其余技术选型（已与用户确认）：
- **状态获取**：PC 端短轮询（每 ~2s `GET /auth/scan/status`），契合纯 `net/http` 架构，无需 WS/SSE 连接管理。
- **存储**：DB 表（多实例负载均衡友好；进程重启不丢失在途扫码）。新增迁移 `0023`。

## Database Migration

新迁移 `0023_add_scan_login_codes.sql`，sqlite + postgres 各一份：

```sql
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
```

### 字段说明

| 字段 | 说明 |
|---|---|
| `device_code_hash` | `device_code`（PC 密钥）的 SHA-256；**只回给 PC，永不进二维码**。沿用 `als_sessions.token_hash` 的哈希规范。 |
| `scan_code_hash` | `scan_code`（二维码内容）的 SHA-256；App 扫码得明文 → 哈希后查行。 |
| `status` | `pending` / `scanned` / `authorized` / `denied`（`expired` 由 `expires_at` 计算，不入库）。 |
| `user_id` | 扫码时写入，来自 App 的认证 session（不信任请求体）。 |
| `session_token_hash` | 确认时复用 `als_sessions` 签发的 token_hash，做关联审计。 |
| `session_token` | 确认时签发的明文 token，**短暂暂存**用于 PC 幂等取 token（仅 ~5min TTL，过期随行清理）。`als_sessions` 仍只存哈希，与现有一致。 |
| `init_ip` | 发起端 IP，审计用。 |
| `expires_at` | 默认 `created_at + 5min`。 |
| `scanned_at` / `authorized_at` / `denied_at` | 各阶段时间戳。 |

> postgres 版：`id SERIAL PRIMARY KEY`，`TIMESTAMPTZ`，其余同。

## Backend Changes

新增 `internal/scanlogin/` 包，封装状态机与 DB 操作；在 `internal/httpapi/routes.go` 注册 5 个本地 handler。

### 状态机（全部原子：`UPDATE ... WHERE status='X' AND expires_at > now`）

```
pending ──scan(App)──► scanned ──confirm(App)──► authorized   ← 成功，PC 取 token
   │                       │
   └──deny─────────────────┴──► denied
任一状态 expires_at 到点 → expired（计算得出，终态）
```

安全约束：
- `scan` 写入的 `user_id` 取自 `auth.UserFromContext`，**不信任 body**。
- `confirm` 校验 `confirmer.user_id == scanner.user_id`（SQL 内校验），防串号。
- 所有转移原子化、单次生效（已 `scanned` 再 scan → 409）。

### 接口契约

`scan/confirm/deny` 套用现有认证中间件（App 必须带合法 Bearer session）。实现时复用 `routes.go` 里已有的 dialect 感知包装器（`auth.RequireUserWithDialect(database, r.sqlDialect)` / 局部 `authenticated` 变量），**不要**直接调用 `auth.RequireUser`，以保持 postgres 兼容；`init/status` 公开。

**1. `POST /auth/scan/init`（公开）**

请求：可选 `{client_meta?:string}`。后端生成 `device_code`（`dc_`+32B hex）与 `scan_code`（`sc_`+24B hex），哈希后写一行 `pending`。

响应 `200`：
```json
{ "device_code":"dc_...", "scan_code":"sc_...", "qr_payload":"sc_...",
  "expires_in":300, "interval":2 }
```
`qr_payload` = `scan_code`（前端用 QR 库编码此串；App 读到的明文即 `scan_code`）。

**2. `GET /auth/scan/status?device_code=<PC密钥>`（公开，PC 每 ~2s 轮询）**

按 `device_code` 哈希查行，按状态递进返回：
- `pending` → `{status, expires_in, interval}`
- `scanned` → `{status:"scanned", ...}`（PC 提示"已扫描，请在手机确认"）
- `authorized` → `{status:"authorized", session_token:"st_...", user:{id,email,name,role}}`（**token 只此通道下发**；行未过期前重复轮询幂等返回同一 token，简化前端断线重连）
- 过期 → `{status:"expired"}`；查无 → `404`

**3. `POST /auth/scan/scan`（App Bearer）**

请求 `{code:"<scan_code>"}`。哈希查行，原子 `pending→scanned`，写入 `user_id`(App 用户)+`scanned_at`。响应 `{status:"scanned"}`。非 pending/过期 → `409`。

**4. `POST /auth/scan/confirm`（App Bearer）**

请求 `{code:"<scan_code>"}`。原子 `scanned→authorized`（校验 confirmer==scanner、未过期），**复用 `als_sessions` 机制为该 user 签发 session**：`token_hash` 写 `als_sessions`（只存哈希，与现有一致）+ `als_scan_codes.session_token_hash`（审计关联）；明文 token 写 `als_scan_codes.session_token` **短暂暂存**供 PC 幂等取用（仅 ~5min TTL，过期随行清理；这是随机 bearer token 可重试交付的必要条件，与 device/magic-link 流程一致）。响应 `{status:"authorized"}`（不返回 token）。状态不符 → `409`。

**5. `POST /auth/scan/deny`（App Bearer）**

请求 `{code:"<scan_code>"}`。`scanned|pending → denied`。响应 `{status:"denied"}`。PC 轮询显示"已取消"。

### session 签发复用

抽 `user.Service.MintSessionForUser(ctx, userID) (plaintext, tokenHash, error)`：复用现有 `auth.NewSessionToken()` + `als_sessions` INSERT 逻辑，供扫码确认与（可选）密码登录路径共用，避免重复。

### 后台清理

启动一个每分钟 tick 的 goroutine：`DELETE FROM als_scan_codes WHERE expires_at < now() - 10min`。已取 token 的 `authorized` 行在宽限期后回收；多实例各自清扫幂等。

## Frontend Changes（PC 端）

- `app/login/page.tsx` 增加 Tab：「密码登录 / 扫码登录」。
- 新增 `ScanLoginPanel` 组件：
  - 挂载 → `POST /api/auth/scan/init` 拿 `device_code` + `qr_payload`。
  - 用 QR 库把 `qr_payload` 渲染成二维码（实现时确认现有依赖，否则加轻量库如 `qrcode`）。
  - 每 ~2s `GET /api/auth/scan/status?device_code=...`，按状态切文案：等待扫码 / 已扫描请手机确认 / 登录成功 / 已取消 / 二维码已过期。
  - `authorized`：存 `localStorage["session_token"]`，按 `user.role` 跳转（与密码登录一致：admin→/admin/users、distributor→/distributor、其余→/dashboard）。
  - 过期/取消 → 自动重新 `init`；卸载清定时器。
- 新增前端透传路由 `app/api/auth/scan/init/route.ts`、`app/api/auth/scan/status/route.ts`（沿用 `app/api/auth/login/route.ts` 的 thin proxy 模式）。scan/confirm/deny 是 App 端，无需前端路由。
- i18n：`messages/`（zh + en）补扫码相关文案 key。

## 错误处理 / 边界

- **竞态**：两个 App session 扫同一码 → 第一个原子置 `scanned`，第二个 `409 already_scanned`。
- **串号**：A 扫码、B 确认 → `confirm` SQL 校验 `user_id` 不符 → `409`。
- **重复确认**：已 `authorized` 再 confirm → `409`。
- **过期**：`expires_at` 到点，所有转移 SQL 的 `WHERE expires_at>now` 天然挡住；`status` 返回 `expired`，前端重生码。
- **重启/多实例**：行持久化在 DB，在途扫码不受影响。
- **暴防**：`scan_code` 24B 随机不可猜；lookup miss 统一 `404`，不泄露存在性差异。
- **`device_code` 当凭据**：它等价于一次性取 token 的口令，PC 端按凭据处理、不得记入日志/埋点。

## Testing

- **service 层单测**（`internal/scanlogin/`）：init 写入双哈希行；scan/confirm/deny 全部状态转移 + 错误态 409（重复 scan、串号 confirm、过期转移）；status 各状态返回 + authorized 幂等返回 token + 过期/未知 404。
- **session 复用**：`MintSessionForUser` 产生有效 `als_sessions` 行，token 能通过现有 `RequireUser` 中间件。
- **集成**（沿用 `internal/httpapi/*_test.go` 的 httptest 模式）：init→scan(App 带 session)→confirm→status 取 token 全链路；未授权 scan/confirm 返回 401。
- **前端**：最小化（渲染 + 轮询 mock），不阻塞后端交付。

## 不在本次范围

- App 端扫码 / 确认 UI（App 团队自行对接，后端接口已就绪）。
- 手机浏览器确认页（当前模型依赖已登录 App；如未来需无 App 兜底再扩展）。
- 扫码登录开关 / 频控后台（always-on，与现有登录策略一致）。
