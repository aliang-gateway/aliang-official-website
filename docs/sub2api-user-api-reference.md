# Sub2API 用户侧接口文档（��服务调用版）

## 约定

| 项目 | 说明 |
|------|------|
| **Base URL** | `http://<sub2api-host>:8080` |
| **认证方式** | `Authorization: Bearer <jwt_token>`（透传用户 JWT） |
| **共享依赖** | 同一 PostgreSQL + 同一 Redis + 同一 JWT Secret |
| **响应格式** | JSON，统一包装 `{ "data": ..., "message": "success" }` |

---

## 一、认证 Auth

### 1.1 注册

```
POST /api/v1/auth/register
```

**认证**: 无需

**Request Body**:
```json
{
  "email": "user@example.com",
  "password": "123456",
  "verify_code": "123456",
  "turnstile_token": "xxx",
  "promo_code": "WELCOME",
  "invitation_code": "INV123"
}
```

**Response** (`200`):
```json
{
  "data": {
    "access_token": "eyJhbG...",
    "refresh_token": "rt_xxx",
    "expires_in": 1800,
    "token_type": "Bearer",
    "user": { "id": 1, "email": "user@example.com", "role": "user" }
  }
}
```

---

### 1.2 登录

```
POST /api/v1/auth/login
```

**认证**: 无需

**Request Body**:
```json
{
  "email": "user@example.com",
  "password": "123456",
  "turnstile_token": "xxx"
}
```

**Response** (`200`):
```json
{
  "data": {
    "access_token": "eyJhbG...",
    "refresh_token": "rt_xxx",
    "expires_in": 1800,
    "token_type": "Bearer",
    "user": { "id": 1, "email": "user@example.com", "role": "user" }
  }
}
```

> 若用户开启了 TOTP 二步验证，登录会返回特殊状态码，需再调用 `/auth/login/2fa`。

---

### 1.3 登录（二步验证）

```
POST /api/v1/auth/login/2fa
```

**认证**: 无需

**Request Body**:
```json
{
  "email": "user@example.com",
  "password": "123456",
  "totp_code": "123456"
}
```

---

### 1.4 刷新 Token

```
POST /api/v1/auth/refresh
```

**认证**: 无需（携带 refresh_token）

**Request Body**:
```json
{
  "refresh_token": "rt_xxx"
}
```

---

### 1.5 登出

```
POST /api/v1/auth/logout
```

**认证**: 无需

**Request Body**:
```json
{
  "refresh_token": "rt_xxx"
}
```

---

### 1.6 撤销所有会话

```
POST /api/v1/auth/revoke-all-sessions
```

**认证**: 需要 JWT

---

### 1.7 忘记密码（发送重置邮件）

```
POST /api/v1/auth/forgot-password
```

**认证**: 无需

**Request Body**:
```json
{ "email": "user@example.com" }
```

---

### 1.8 重置密码

```
POST /api/v1/auth/reset-password
```

**认证**: 无需

**Request Body**:
```json
{
  "email": "user@example.com",
  "code": "123456",
  "new_password": "newpass123"
}
```

---

### 1.9 发送邮件验证码

```
POST /api/v1/auth/send-verify-code
```

**认证**: 无需

**Request Body**:
```json
{ "email": "user@example.com", "turnstile_token": "xxx" }
```

---

### 1.10 获取当前用户信息

```
GET /api/v1/auth/me
```

**认证**: 需要 JWT

---

## 二、用户 User

### 2.1 获取用户资料（含余额）

```
GET /api/v1/user/profile
```

**认证**: 需要 JWT

**Response**:
```json
{
  "data": {
    "id": 1,
    "email": "user@example.com",
    "username": "张三",
    "role": "user",
    "balance": 99.50000000,
    "concurrency": 5,
    "status": "active",
    "allowed_groups": [1, 3],
    "created_at": "2025-01-01T00:00:00Z",
    "updated_at": "2025-03-20T00:00:00Z"
  }
}
```

> **`balance`** 即为账户余额（单位 USD）。

---

### 2.2 更新个人资料

```
PUT /api/v1/user
```

**认证**: 需要 JWT

**Request Body**:
```json
{ "username": "新名字" }
```

---

### 2.3 修改密码

```
PUT /api/v1/user/password
```

**认证**: 需要 JWT

**Request Body**:
```json
{
  "old_password": "oldpass",
  "new_password": "newpass123"
}
```

> 注意：修改密码后所有已签发的 JWT 立即失效（TokenVersion 递增机制）。

---

### 2.4 TOTP 二步验证

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/user/totp/status` | 查询 TOTP 状态 |
| GET | `/api/v1/user/totp/verification-method` | 获取当前验证方式 |
| POST | `/api/v1/user/totp/send-code` | 发送备用验证码 |
| POST | `/api/v1/user/totp/setup` | 发起 TOTP 绑定 |
| POST | `/api/v1/user/totp/enable` | 启用 TOTP |
| POST | `/api/v1/user/totp/disable` | 禁用 TOTP |

---

## 三、API Key 管理

### 3.1 获取可用分组列表

```
GET /api/v1/groups/available
```

**认证**: 需要 JWT

**Response**:
```json
{
  "data": [
    {
      "id": 3,
      "name": "Claude 基础组",
      "platform": "anthropic",
      "rate_multiplier": 1.0,
      "is_exclusive": false,
      "status": "active",
      "subscription_type": "",
      "daily_limit_usd": null,
      "weekly_limit_usd": null,
      "monthly_limit_usd": null
    }
  ]
}
```

---

### 3.2 查询分组倍率

```
GET /api/v1/groups/rates
```

**认证**: 需要 JWT

---

### 3.3 创建 API Key（指定分组）

```
POST /api/v1/keys
```

**认证**: 需要 JWT

**Request Body**:
```json
{
  "name": "我的 Claude Key",
  "group_id": 3,
  "custom_key": "sk-my-custom-key",
  "quota": 100.0,
  "expires_in_days": 30,
  "ip_whitelist": ["1.2.3.4"],
  "ip_blacklist": [],
  "rate_limit_5h": 50.0,
  "rate_limit_1d": 100.0,
  "rate_limit_7d": 500.0
}
```

**Response** (`200`):
```json
{
  "data": {
    "id": 10,
    "key": "sk-xxx...yyy",
    "name": "我的 Claude Key",
    "group_id": 3,
    "status": "active",
    "quota": 100.0,
    "quota_used": 0,
    "expires_at": "2025-04-22T00:00:00Z",
    "rate_limit_5h": 50.0,
    "rate_limit_1d": 100.0,
    "rate_limit_7d": 500.0,
    "created_at": "2025-03-23T00:00:00Z"
  }
}
```

> **`key` 明文仅在创建时返回一次**，之后不可再查看。

---

### 3.4 列出我的所有 API Key

```
GET /api/v1/keys?page=1&per_page=20&status=active&group_id=3&search=claude
```

**认证**: 需要 JWT

---

### 3.5 获取单个 API Key 详情

```
GET /api/v1/keys/:id
```

**认证**: 需要 JWT

---

### 3.6 更新 API Key

```
PUT /api/v1/keys/:id
```

**认证**: 需要 JWT

**Request Body**:
```json
{
  "name": "新名称",
  "group_id": 5,
  "status": "inactive",
  "quota": 200.0,
  "reset_quota": true,
  "expires_at": "2025-12-31T23:59:59Z",
  "rate_limit_1d": 200.0
}
```

---

### 3.7 删除 API Key

```
DELETE /api/v1/keys/:id
```

**认证**: 需要 JWT

---

## 四、使用统计与查询

### 4.1 仪表盘概览

```
GET /api/v1/usage/dashboard/stats
```

**认证**: 需要 JWT

**Response**:
```json
{
  "data": {
    "total_api_keys": 5,
    "active_api_keys": 3,
    "total_requests": 12000,
    "total_input_tokens": 5000000,
    "total_output_tokens": 2000000,
    "total_tokens": 7000000,
    "total_cost": 150.50,
    "total_actual_cost": 145.20,
    "today_requests": 300,
    "today_input_tokens": 120000,
    "today_output_tokens": 50000,
    "today_tokens": 170000,
    "today_cost": 3.75,
    "today_actual_cost": 3.60,
    "average_duration_ms": 1250.5,
    "rpm": 12,
    "tpm": 6800
  }
}
```

---

### 4.2 使用趋势（不同时间粒度）

```
GET /api/v1/usage/dashboard/trend?start_date=2025-03-01&end_date=2025-03-23&granularity=day
```

**认证**: 需要 JWT

**Query 参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `start_date` | string | 起始日期 `YYYY-MM-DD`，默认 7 天前 |
| `end_date` | string | 结束日期 `YYYY-MM-DD`，默认今天 |
| `granularity` | string | 聚合粒度：`day`（默认）/ `week` / `month` |
| `timezone` | string | 时区，如 `Asia/Shanghai` |

**Response**:
```json
{
  "data": {
    "trend": [
      {
        "date": "2025-03-01",
        "requests": 450,
        "input_tokens": 180000,
        "output_tokens": 72000,
        "cache_creation_tokens": 5000,
        "cache_read_tokens": 12000,
        "total_tokens": 269000,
        "cost": 5.62,
        "actual_cost": 5.40
      },
      { "date": "2025-03-02", "..." : "..." }
    ],
    "start_date": "2025-03-01",
    "end_date": "2025-03-23",
    "granularity": "day"
  }
}
```

---

### 4.3 模型使用分布

```
GET /api/v1/usage/dashboard/models?start_date=2025-03-01&end_date=2025-03-23
```

**认证**: 需要 JWT

**Response**:
```json
{
  "data": {
    "models": [
      {
        "model": "claude-sonnet-4-20250514",
        "requests": 5000,
        "input_tokens": 2000000,
        "output_tokens": 800000,
        "total_tokens": 2800000,
        "cost": 60.00,
        "actual_cost": 57.50
      },
      {
        "model": "gpt-4o",
        "requests": 3000,
        "input_tokens": 1200000,
        "output_tokens": 500000,
        "total_tokens": 1700000,
        "cost": 45.00,
        "actual_cost": 43.00
      }
    ],
    "start_date": "2025-03-01",
    "end_date": "2025-03-23"
  }
}
```

---

### 4.4 按 API Key 批量查询用量

```
POST /api/v1/usage/dashboard/api-keys-usage
```

**认证**: 需要 JWT

**Request Body**:
```json
{
  "api_key_ids": [10, 11, 12]
}
```

**Response**:
```json
{
  "data": {
    "stats": {
      "10": { "api_key_id": 10, "today_actual_cost": 1.50, "total_actual_cost": 25.00 },
      "11": { "api_key_id": 11, "today_actual_cost": 0.80, "total_actual_cost": 12.00 }
    }
  }
}
```

---

### 4.5 使用���计汇总（支持时间范围）

```
GET /api/v1/usage/stats?period=month&api_key_id=10
GET /api/v1/usage/stats?start_date=2025-03-01&end_date=2025-03-23
```

**认证**: 需要 JWT

**Query 参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `period` | string | `today` / `week` / `month`（与 start_date/end_date 二选一） |
| `start_date` | string | 起始日期 `YYYY-MM-DD` |
| `end_date` | string | 结束日期 `YYYY-MM-DD` |
| `api_key_id` | int | 按 API Key 过滤 |
| `timezone` | string | 时区 |

**Response**:
```json
{
  "data": {
    "total_requests": 5000,
    "total_input_tokens": 2000000,
    "total_output_tokens": 800000,
    "total_cache_tokens": 15000,
    "total_tokens": 2815000,
    "total_cost": 60.00,
    "total_actual_cost": 57.50,
    "average_duration_ms": 1200.0,
    "endpoints": [
      { "endpoint": "/v1/messages", "requests": 3000, "total_tokens": 1500000, "cost": 30.00, "actual_cost": 28.50 },
      { "endpoint": "/v1/chat/completions", "requests": 2000, "total_tokens": 1315000, "cost": 30.00, "actual_cost": 29.00 }
    ],
    "upstream_endpoints": []
  }
}
```

---

### 4.6 使用记录列表

```
GET /api/v1/usage?page=1&per_page=20&model=claude-sonnet-4-20250514&start_date=2025-03-01&end_date=2025-03-23&api_key_id=10&request_type=chat&stream=true
```

**认证**: 需要 JWT

**Query 参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码，默认 1 |
| `per_page` | int | 每页条数，默认 20 |
| `model` | string | 按模型筛选 |
| `api_key_id` | int | 按 API Key 筛选 |
| `start_date` | string | 起始日期 |
| `end_date` | string | 结束日期 |
| `request_type` | string | 请求类型：`chat` / `image` / `sora` 等 |
| `stream` | bool | 按流式筛选 |
| `billing_type` | int | 按计费类型筛选 |
| `timezone` | string | 时区 |

**Response** (数组中每条记录):
```json
{
  "data": [
    {
      "id": 1001,
      "user_id": 1,
      "api_key_id": 10,
      "request_id": "req_xxx",
      "model": "claude-sonnet-4-20250514",
      "inbound_endpoint": "/v1/messages",
      "group_id": 3,
      "subscription_id": 5,
      "input_tokens": 1500,
      "output_tokens": 800,
      "cache_creation_tokens": 0,
      "cache_read_tokens": 200,
      "total_cost": 0.03,
      "actual_cost": 0.0285,
      "rate_multiplier": 1.0,
      "request_type": "chat",
      "stream": true,
      "duration_ms": 3200,
      "first_token_ms": 450,
      "created_at": "2025-03-20T10:30:00Z",
      "user": { "id": 1, "email": "user@example.com" },
      "api_key": { "id": 10, "name": "我的 Claude Key" },
      "group": { "id": 3, "name": "Claude 基础组" }
    }
  ]
}
```

---

### 4.7 单条使用记录详情

```
GET /api/v1/usage/:id
```

**认证**: 需要 JWT

---

## 五、充值与余额

### 5.1 卡密兑换（充值）

```
POST /api/v1/redeem
```

**认证**: 需要 JWT

**Request Body**:
```json
{ "code": "REDEEM-XXXX-YYYY" }
```

**Response**:
```json
{
  "data": {
    "id": 50,
    "code": "REDEEM-XXXX-YYYY",
    "type": "admin_balance",
    "value": 50.00,
    "status": "used",
    "used_by": 1,
    "used_at": "2025-03-23T10:00:00Z"
  }
}
```

> 卡密类型 `type` 包括：
> - `admin_balance` — 充值余额
> - `admin_concurrency` — 增加并发
> - `subscription` — 获得订阅（含 `group_id` 和 `validity_days`）

---

### 5.2 充值历史

```
GET /api/v1/redeem/history
```

**认证**: 需要 JWT

**Response**: 最近 25 条兑换记录

---

### 5.3 余额查询

余额包含在 `/api/v1/user/profile` 的 `balance` 字段中，参见 **2.1**。

---

## 六、订阅管理

### 6.1 订阅列表（全部）

```
GET /api/v1/subscriptions
```

**认证**: 需要 JWT

**Response**:
```json
{
  "data": [
    {
      "id": 5,
      "user_id": 1,
      "group_id": 3,
      "starts_at": "2025-03-01T00:00:00Z",
      "expires_at": "2025-04-01T00:00:00Z",
      "status": "active",
      "daily_usage_usd": 2.50,
      "weekly_usage_usd": 15.00,
      "monthly_usage_usd": 45.00,
      "created_at": "2025-03-01T00:00:00Z",
      "group": {
        "id": 3,
        "name": "Claude 基础组",
        "daily_limit_usd": 5.0,
        "weekly_limit_usd": 30.0,
        "monthly_limit_usd": 100.0
      }
    }
  ]
}
```

---

### 6.2 活跃订阅列表

```
GET /api/v1/subscriptions/active
```

**认证**: 需要 JWT

---

### 6.3 订阅进度（用量/限额对比）

```
GET /api/v1/subscriptions/progress
```

**认证**: 需要 JWT

**Response**:
```json
{
  "data": [
    {
      "subscription": { "id": 5, "group_id": 3, "status": "active" },
      "progress": {
        "daily": { "used": 2.50, "limit": 5.00, "percentage": 50.0 },
        "weekly": { "used": 15.00, "limit": 30.00, "percentage": 50.0 },
        "monthly": { "used": 45.00, "limit": 100.00, "percentage": 45.0 }
      }
    }
  ]
}
```

---

### 6.4 订阅汇总

```
GET /api/v1/subscriptions/summary
```

**认证**: 需要 JWT

**Response**:
```json
{
  "data": {
    "active_count": 2,
    "total_used_usd": 60.00,
    "subscriptions": [
      {
        "id": 5,
        "group_id": 3,
        "group_name": "Claude 基础组",
        "status": "active",
        "daily_used_usd": 2.50,
        "daily_limit_usd": 5.00,
        "weekly_used_usd": 15.00,
        "weekly_limit_usd": 30.00,
        "monthly_used_usd": 45.00,
        "monthly_limit_usd": 100.00,
        "expires_at": "2025-04-01T00:00:00Z"
      }
    ]
  }
}
```

---

### 6.5 加入订阅（通过卡密兑换）

用户侧没有直接"加入订阅"的接口。订阅通过以下方式获得：

1. **卡密兑换** `POST /api/v1/redeem` — 卡密类型为 `subscription` 时自动获得订阅
2. **管理员分配** — 通过 Admin API `POST /api/v1/admin/subscriptions/assign`

如果你的微服务需要为用户分配订阅，使用 Admin API Key 调用：

```
POST /api/v1/admin/subscriptions/assign
x-api-key: <admin-key>
```

---

### 6.6 取消订阅

用户侧没有直接取消订阅的接口。取消需要管理员操作：

```
DELETE /api/v1/admin/subscriptions/:id
x-api-key: <admin-key>
```

---

## 七、公开设置

```
GET /api/v1/settings/public
```

**认证**: 无需

返回注册是否开放、邮件验证是否开启等公开配置。

---

## 附录：需求与接口对照表

| 需求 | 接口 | 状态 |
|------|------|------|
| 注册 | `POST /api/v1/auth/register` | 支持 |
| 登录 | `POST /api/v1/auth/login` | 支持 |
| 忘记密码 | `POST /api/v1/auth/forgot-password` + `reset-password` | 支持 |
| 刷新 Token | `POST /api/v1/auth/refresh` | 支持 |
| 登出 | `POST /api/v1/auth/logout` | 支持 |
| 个人信息查看（含余额） | `GET /api/v1/user/profile` | 支持 |
| 在分组下创建 Key | `POST /api/v1/keys` | 支持 |
| API 调用次数统计 | `GET /api/v1/usage/dashboard/stats` | 支持 |
| Token 统计（日/周/月） | `GET /api/v1/usage/dashboard/trend` | 支持 |
| 模型使用分布 | `GET /api/v1/usage/dashboard/models` | 支持 |
| 按 Key 查用量 | `POST /api/v1/usage/dashboard/api-keys-usage` | 支持 |
| 统计汇总（含端点分布） | `GET /api/v1/usage/stats` | 支持 |
| 使用记录列表 | `GET /api/v1/usage` | 支持 |
| 单条记录详情 | `GET /api/v1/usage/:id` | 支持 |
| 订阅使用情况 | `GET /api/v1/subscriptions/progress` | 支持 |
| 订阅汇总 | `GET /api/v1/subscriptions/summary` | 支持 |
| 充值（卡密兑换） | `POST /api/v1/redeem` | 支持 |
| 余额查询 | `GET /api/v1/user/profile` -> `balance` | 支持 |
| 加入订阅 | 卡密兑换 / Admin API 分配 | 间接支持 |
| 取消订阅 | Admin API `DELETE /admin/subscriptions/:id` | 间接支持 |

> **关于加入/取消订阅的说明**：用户侧没有直接暴露"加入/取消"接口，这是设计上的安全考虑 — 订阅分配和回收由管理员控制。你的微服务可以通过 **Admin API Key** 调用 `POST /api/v1/admin/subscriptions/assign` 和 `DELETE /api/v1/admin/subscriptions/:id` 来实现这两个操作。
