# Sub2API API Key 管理接口文档（用户侧）

## 约定

| 项目 | 说明 |
|------|------|
| **Base URL** | `http://<sub2api-host>:8080` |
| **认证方式** | `Authorization: Bearer <jwt_token>` |
| **响应格式** | JSON，统一包装 `{ "data": ..., "message": "success" }` |

> 所有接口均为**用户侧接口**，通过透传用户的 JWT Token 调用，无需 Admin API Key。
> 用户只能操作自己的 API Key，无法访问其他用户的 Key。

---

## 一、查询 API Key

### 1.1 获取可用分组列表

创建 Key 前需要先获取当前用户可用的分组列表，`group_id` 用于创建 Key 时指定归属分组。

```
GET /api/v1/groups/available
```

**Response**:
```json
{
  "data": [
    {
      "id": 3,
      "name": "Claude 基础组",
      "description": "Claude 系列模型基础分组",
      "platform": "anthropic",
      "rate_multiplier": 1.0,
      "is_exclusive": false,
      "status": "active",
      "subscription_type": "",
      "daily_limit_usd": null,
      "weekly_limit_usd": null,
      "monthly_limit_usd": null,
      "claude_code_only": false,
      "allow_messages_dispatch": true
    }
  ]
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int64 | 分组 ID，创建 Key 时用 |
| `name` | string | 分组名称 |
| `platform` | string | 平台：`anthropic` / `openai` / `gemini` / `antigravity` |
| `rate_multiplier` | float64 | 倍率，实际计费 = 原始费用 × 倍率 |
| `is_exclusive` | bool | 是否为专属分组（专属分组需管理员授权才能绑定） |
| `subscription_type` | string | 订阅类型，空字符串表示非订阅制 |
| `daily_limit_usd` | float64\|null | 日限额 (USD)，null 表示不限制 |
| `weekly_limit_usd` | float64\|null | 周限额 (USD) |
| `monthly_limit_usd` | float64\|null | 月限额 (USD) |
| `claude_code_only` | bool | 是否仅限 Claude Code 客户端使用 |
| `allow_messages_dispatch` | bool | 是否开启 OpenAI Messages 调度 |

---

### 1.2 列出我的所有 API Key

```
GET /api/v1/api-keys
```

**Query 参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | 否 | 页码，默认 1 |
| `per_page` | int | 否 | 每页条数，默认 20 |
| `status` | string | 否 | 筛选状态：`active` / `inactive` |
| `group_id` | int | 否 | 按分组 ID 筛选 |
| `search` | string | 否 | 按 Key 名称模糊搜索（最长 100 字符） |

**Response**:
```json
{
  "data": [
    {
      "id": 10,
      "user_id": 1,
      "key": "sk-xxx...***",
      "name": "我的 Claude Key",
      "group_id": 3,
      "status": "active",
      "ip_whitelist": ["1.2.3.4"],
      "ip_blacklist": [],
      "last_used_at": "2025-03-24T10:30:00Z",
      "quota": 100.0,
      "quota_used": 25.50,
      "expires_at": "2025-04-22T00:00:00Z",
      "created_at": "2025-03-23T00:00:00Z",
      "updated_at": "2025-03-24T10:30:00Z",
      "rate_limit_5h": 50.0,
      "rate_limit_1d": 100.0,
      "rate_limit_7d": 500.0,
      "usage_5h": 12.30,
      "usage_1d": 45.60,
      "usage_7d": 200.00,
      "window_5h_start": "2025-03-24T08:00:00Z",
      "window_1d_start": "2025-03-24T00:00:00Z",
      "window_7d_start": "2025-03-18T00:00:00Z"
    }
  ],
  "pagination": {
    "total": 3,
    "page": 1,
    "page_size": 20,
    "pages": 1
  }
}
```

**APIKey 对象字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | int64 | Key ID |
| `user_id` | int64 | 所属用户 ID |
| `key` | string | Key 掩码（列表中不返回明文） |
| `name` | string | Key 名称 |
| `group_id` | int64\|null | 所属分组 ID |
| `status` | string | 状态：`active`（启用）/ `inactive`（禁用） |
| `ip_whitelist` | string[] | IP 白名单，空数组表示不限制 |
| `ip_blacklist` | string[] | IP 黑名单 |
| `last_used_at` | string\|null | 最后使用时间 |
| `quota` | float64 | 配额上限 (USD)，0 表示不限制 |
| `quota_used` | float64 | 已用配额 (USD) |
| `expires_at` | string\|null | 过期时间，null 表示永不过期 |
| `rate_limit_5h` | float64 | 5 小时请求金额限制 (USD)，0 不限制 |
| `rate_limit_1d` | float64 | 1 天请求金额限制 (USD) |
| `rate_limit_7d` | float64 | 7 天请求金额限制 (USD) |
| `usage_5h` | float64 | 近 5 小时已用金额 (USD) |
| `usage_1d` | float64 | 近 1 天已用金额 (USD) |
| `usage_7d` | float64 | 近 7 天已用金额 (USD) |
| `window_5h_start` | string | 5 小时窗口起始时间 |
| `window_1d_start` | string | 1 天窗口起始时间 |
| `window_7d_start` | string | 7 天窗口起始时间 |

---

### 1.3 获取单个 API Key 详情

```
GET /api/v1/api-keys/:id
```

**Response**:
```json
{
  "data": {
    "id": 10,
    "user_id": 1,
    "key": "sk-xxx...***",
    "name": "我的 Claude Key",
    "group_id": 3,
    "status": "active",
    "ip_whitelist": ["1.2.3.4"],
    "ip_blacklist": [],
    "last_used_at": "2025-03-24T10:30:00Z",
    "quota": 100.0,
    "quota_used": 25.50,
    "expires_at": "2025-04-22T00:00:00Z",
    "created_at": "2025-03-23T00:00:00Z",
    "updated_at": "2025-03-24T10:30:00Z",
    "rate_limit_5h": 50.0,
    "rate_limit_1d": 100.0,
    "rate_limit_7d": 500.0,
    "usage_5h": 12.30,
    "usage_1d": 45.60,
    "usage_7d": 200.00,
    "user": { "id": 1, "email": "user@example.com" },
    "group": { "id": 3, "name": "Claude 基础组", "platform": "anthropic" }
  }
}
```

> 与列表接口相比，详情接口额外返回了 `user` 和 `group` 关联对象。

---

## 二、创建 API Key

### 2.1 创建 API Key

```
POST /api/v1/api-keys
```

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

**字段说明**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | Key 名称 |
| `group_id` | int64 | 否 | 所属分组 ID（为 null 则不绑定分组） |
| `custom_key` | string | 否 | 自定义 Key 前缀，需以 `sk-` 开头 |
| `quota` | float64 | 否 | 配额上限 (USD)，0 或不传表示不限制 |
| `expires_in_days` | int | 否 | 有效天数，不传表示永不过期 |
| `ip_whitelist` | string[] | 否 | IP 白名单，空数组或省略表示不限制 |
| `ip_blacklist` | string[] | 否 | IP 黑名单 |
| `rate_limit_5h` | float64 | 否 | 5 小时内请求金额限制 (USD)，0 或不传不限制 |
| `rate_limit_1d` | float64 | 否 | 1 天内请求金额限制 (USD) |
| `rate_limit_7d` | float64 | 否 | 7 天内请求金额限制 (USD) |

**Response** (`200`):
```json
{
  "data": {
    "id": 10,
    "key": "sk-xxx...yyy",
    "name": "我的 Claude Key",
    "group_id": 3,
    "status": "active",
    "ip_whitelist": ["1.2.3.4"],
    "ip_blacklist": [],
    "last_used_at": null,
    "quota": 100.0,
    "quota_used": 0,
    "expires_at": "2025-04-22T00:00:00Z",
    "created_at": "2025-03-23T00:00:00Z",
    "updated_at": "2025-03-23T00:00:00Z",
    "rate_limit_5h": 50.0,
    "rate_limit_1d": 100.0,
    "rate_limit_7d": 500.0,
    "usage_5h": 0,
    "usage_1d": 0,
    "usage_7d": 0
  }
}
```

> **重要**：`key` 字段的**明文仅在创建时返回一次**，之后无法再查看。务必在创建后保存。

---

## 三、禁用与启用 API Key

### 3.1 更新 API Key（禁用 / 启用）

```
PUT /api/v1/api-keys/:id
```

**禁用 Key**:

```json
{ "status": "inactive" }
```

**启用 Key**:

```json
{ "status": "active" }
```

**完整字段说明**（可按需传入需要修改的字段，未传入的字段保持不变）:

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | Key 名称 |
| `group_id` | int64 | 切换所属分组（传 null 则解绑分组） |
| `status` | string | `active`（启用）/ `inactive`（禁用） |
| `quota` | float64 | 配额上限 (USD)，0 表示不限制 |
| `reset_quota` | bool | 设为 `true` 时重置已用配额为 0 |
| `expires_at` | string | 过期时间 (ISO 8601)，传空字符串 `""` 表示取消过期 |
| `ip_whitelist` | string[] | IP 白名单 |
| `ip_blacklist` | string[] | IP 黑名单 |
| `rate_limit_5h` | float64 | 5 小时限速 (USD) |
| `rate_limit_1d` | float64 | 1 天限速 (USD) |
| `rate_limit_7d` | float64 | 7 天限速 (USD) |
| `reset_rate_limit_usage` | bool | 设为 `true` 时重置所有限速用量 |

**Response** (`200`):
```json
{
  "data": {
    "id": 10,
    "key": "sk-xxx...***",
    "name": "我的 Claude Key",
    "group_id": 3,
    "status": "inactive",
    "quota": 100.0,
    "quota_used": 25.50,
    "expires_at": "2025-04-22T00:00:00Z",
    "created_at": "2025-03-23T00:00:00Z",
    "updated_at": "2025-03-24T12:00:00Z"
  }
}
```

---

### 3.2 删除 API Key

```
DELETE /api/v1/api-keys/:id
```

**Response**:
```json
{
  "data": { "message": "API key deleted successfully" }
}
```

> 删除为永久操作，不可恢复。如���临时停用，建议使用更新接口将 `status` 设为 `inactive`。

---

## 四、查询分组倍率

```
GET /api/v1/groups/rates
```

获取当前用户的专属分组倍率配置。

**Response**:
```json
{
  "data": {
    "3": 1.0,
    "5": 1.5
  }
}
```

> 返回 `map<groupID, rateMultiplier>`，表示各分组的实际计费倍率。
> 未配置专属倍率的分组不会出现在返回结果中（使用系统默认倍率）。

---

## 接口总览

| 操作 | 方法 | 路径 | 说明 |
|------|------|------|------|
| 查询可用分组 | GET | `/api/v1/groups/available` | 创建 Key 前获取可选分组 |
| 查询分组倍率 | GET | `/api/v1/groups/rates` | 获取用户专属倍率配置 |
| 列出所有 Key | GET | `/api/v1/api-keys` | 支持分页、筛选、搜索 |
| 查询单个 Key | GET | `/api/v1/api-keys/:id` | 获取 Key 详情 |
| 创建 Key | POST | `/api/v1/api-keys` | 创建后返回明文（仅一次） |
| 禁用/启用 Key | PUT | `/api/v1/api-keys/:id` | 修改 `status` 字段 |
| 删除 Key | DELETE | `/api/v1/api-keys/:id` | 永久删除 |

---

## 调用示例

### 创建 Key 并绑定到指定分组

```bash
curl -X POST http://localhost:8080/api/v1/api-keys \
  -H "Authorization: Bearer eyJhbG..." \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Claude 测试 Key",
    "group_id": 3,
    "quota": 50.0,
    "expires_in_days": 90
  }'
```

### 禁用 Key

```bash
curl -X PUT http://localhost:8080/api/v1/api-keys/10 \
  -H "Authorization: Bearer eyJhbG..." \
  -H "Content-Type: application/json" \
  -d '{ "status": "inactive" }'
```

### 查询所有 Key

```bash
curl http://localhost:8080/api/v1/api-keys?page=1&per_page=20&status=active \
  -H "Authorization: Bearer eyJhbG..."
```

### 查询指定 Key 详情

```bash
curl http://localhost:8080/api/v1/api-keys/10 \
  -H "Authorization: Bearer eyJhbG..."
```
