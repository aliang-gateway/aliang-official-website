# Sub2API 运维监控 Snapshot-V2 接口文档

## 概述

`GET /api/v1/admin/ops/dashboard/snapshot-v2` 是一个**运维监控聚合接口**，在**一次请求**中并行返回三项核心数据：

1. **Overview** — 综合健康状态概览（健康评分 + 系统指标 + 业务指标）
2. **ThroughputTrend** — 吞吐量时间序列趋势（请求量 / Token / QPS / TPS）
3. **ErrorTrend** — 错误量时间序列趋势（错误数 / SLA 失败 / 上游错误）

该接口内置 **30 秒缓存** + **ETag 条件请求**支持，可直接用于前端监控面板。

---

## 接口详情

### 基本信息

| 项目 | 说明 |
|------|------|
| **路径** | `GET /api/v1/admin/ops/dashboard/snapshot-v2` |
| **认证** | `x-api-key: <admin-key>` |
| **缓存** | 30 秒（按请求参数生成 CacheKey） |
| **条件请求** | 支持 `ETag` + `If-None-Match`（返回 304 Not Modified） |

### Query 参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `start_time` | ISO8601 时间字符串 | 默认为 `defaultRange`（默认 `1h`）往前推 | 起始时间，支持 RFC3339 / RFC3339Nano |
| `end_time` | ISO8601 时间字符串 | 默认 `now` | 结束时间 |
| `platform` | string | 空（全局） | 按平台筛选，如 `anthropic` / `openai` / `gemini` |
| `group_id` | int | 空（全局） | 按分组 ID 筛选 |
| `mode` | string | 空（使用服务端默认） | 查询模式：`auto` / `raw` / `preagg` |

> `start_time` 和 `end_time` 均支持**部分指定**（如只填日期），系统会自动补全。若两者均不填，则默认查询最近 1 小时。

---

## 响应结构

### 顶层结构

```json
{
  "data": {
    "generated_at": "2025-03-24T10:00:00Z",
    "overview": { ... },
    "throughput_trend": { ... },
    "error_trend": { ... }
  }
}
```

**响应头**：

| 头信息 | 说明 |
|--------|------|
| `X-Snapshot-Cache` | `hit` 或 `miss`，标识是否命中缓存 |
| `ETag` | 资源标识，用于条件请求 |
| `Vary` | `If-None-Match` |

---

## 1. Overview（综合健康概览）

> 等同于单独调用 `GET /api/v1/admin/ops/dashboard/overview`

### 1.1 健康评分与 SLA

| 字段 | 类型 | 说明 |
|------|------|------|
| `health_score` | int | 综合健康评分（0-100），由系统自动计算 |
| `sla` | float64 | SLA 合规率（0.0 ~ 1.0），通常要求 ≥ 0.99 |
| `error_rate` | float64 | 错误率（0.0 ~ 1.0） |
| `upstream_error_rate` | float64 | 上游错误率（不含 429/529） |
| `upstream_429_count` | int64 | 上游返回 429（限流）的总次数 |
| `upstream_529_count` | int64 | 上游返回 529（过载）的总次数 |
| `upstream_error_count_excl_429_529` | int64 | 上游其他错误次数（不含 429/529） |

### 1.2 请求量统计

| 字段 | 类型 | 说明 |
|------|------|------|
| `success_count` | int64 | 成功请求数 |
| `error_count_total` | int64 | 错误请求总数 |
| `business_limited_count` | int64 | 因业务限制被拒绝的请求数（如配额耗尽、限额到达） |
| `error_count_sla` | int64 | SLA 失败计数（通常为 4xx/5xx 上游错误） |
| `request_count_total` | int64 | 总请求数 |
| `request_count_sla` | int64 | 纳入 SLA 计算的总请求数 |

### 1.3 Token 与费用

| 字段 | 类型 | 说明 |
|------|------|------|
| `token_consumed` | int64 | 时间窗口内消耗的总 Token 数 |

### 1.4 QPS / TPS 速率摘要

| 字段 | 子字段 | 类型 | 说明 |
|------|--------|------|------|
| `qps` | `current` | float64 | 当前 QPS（queries per second） |
| `qps` | `peak` | float64 | 峰值 QPS |
| `qps` | `avg` | float64 | 平均 QPS |
| `tps` | `current` | float64 | 当前 TPS（tokens per second） |
| `tps` | `peak` | float64 | 峰值 TPS |
| `tps` | `avg` | float64 | 平均 TPS |

### 1.5 延迟百分位数

| 字段 | 子字段 | 类型 | 说明 |
|------|--------|------|------|
| `duration` | `p50_ms` | int | P50 响应延迟（毫秒） |
| `duration` | `p90_ms` | int | P90 响应延迟 |
| `duration` | `p95_ms` | int | P95 响应延迟 |
| `duration` | `p99_ms` | int | P99 响应延迟 |
| `duration` | `avg_ms` | int | 平均响应延迟 |
| `duration` | `max_ms` | int | 最大响应延迟 |
| `ttft` | `p50_ms` | int | P50 首 Token 时间（毫秒） |
| `ttft` | `p90_ms` | int | P90 首 Token 时间 |
| `ttft` | `p95_ms` | int | P95 首 Token 时间 |
| `ttft` | `p99_ms` | int | P99 首 Token 时间 |

> `ttft`（Time To First Token）仅在流式请求场景有意义。

### 1.6 系统指标（System Metrics）

> 来自主机层的实时监控数据（CPU/内存/DB/Redis）

| 字段 | 类型 | 说明 |
|------|------|------|
| `system_metrics.cpu_usage_percent` | float64 | CPU 使用率 (%) |
| `system_metrics.memory_used_mb` | int64 | 内存使用量 (MB) |
| `system_metrics.memory_total_mb` | int64 | 内存总量 (MB) |
| `system_metrics.memory_usage_percent` | float64 | 内存使用率 (%) |
| `system_metrics.db_ok` | bool | PostgreSQL 连接是否正常 |
| `system_metrics.redis_ok` | bool | Redis 连接是否正常 |
| `system_metrics.db_conn_active` | int | DB 活跃连接数 |
| `system_metrics.db_conn_idle` | int | DB 空闲连接数 |
| `system_metrics.db_conn_waiting` | int | DB 等待连接数（>0 表示连接池紧张） |
| `system_metrics.db_max_open_conns` | int | DB 最大连接数上限（来自配置） |
| `system_metrics.redis_conn_total` | int | Redis 总连接数 |
| `system_metrics.redis_conn_idle` | int | Redis 空闲连接数 |
| `system_metrics.redis_pool_size` | int | Redis 连接池大小（来自配置） |
| `system_metrics.goroutine_count` | int | Go 协程数量 |
| `system_metrics.concurrency_queue_depth` | int | 并发请求队列深度 |
| `system_metrics.account_switch_count` | int64 | 账户切换次数（配额轮换相关） |

### 1.7 后台任务心跳（Job Heartbeats）

| 字段 | 子字段 | 类型 | 说明 |
|------|--------|------|------|
| `job_heartbeats[].job_name` | | string | 任务名称 |
| `job_heartbeats[].last_run_at` | | string\|null | 上次运行时间 |
| `job_heartbeats[].last_success_at` | | string\|null | 上次成功时间 |
| `job_heartbeats[].last_error_at` | | string\|null | 上次错误时间 |
| `job_heartbeats[].last_error` | | string\|null | 上次错误信息 |
| `job_heartbeats[].last_duration_ms` | | int64\|null | 上次运行时长（毫秒） |
| `job_heartbeats[].last_result` | | string\|null | 上次运行结果摘要 |

---

## 2. ThroughputTrend（吞吐量趋势）

### 2.1 顶层字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `bucket` | string | 数据分桶粒度，如 `"60"` 表示每 60 秒一个点 |
| `points` | array | 时间序列数据点数组 |
| `by_platform` | array | 按平台分组的汇总（仅未指定 platform 时返回） |
| `top_groups` | array | 分组排行（仅指定 platform 但未指定 group_id 时返回） |

### 2.2 ThroughputTrendPoint（每个数据点）

| 字段 | 类型 | 说明 |
|------|------|------|
| `bucket_start` | ISO8601 时间 | 该桶的起始时间 |
| `request_count` | int64 | 该时间窗口内的请求总数 |
| `token_consumed` | int64 | 该时间窗口内的 Token 消耗总量 |
| `switch_count` | int64 | 账户切换次数（配额轮换） |
| `qps` | float64 | 该窗口内的平均 QPS |
| `tps` | float64 | 该窗口内的平均 TPS |

### 2.3 ByPlatform（按平台汇总）

| 字段 | 类型 | 说明 |
|------|------|------|
| `platform` | string | 平台名称，如 `anthropic` |
| `request_count` | int64 | 该平台总请求数 |
| `token_consumed` | int64 | 该平台总 Token 消耗 |

### 2.4 TopGroups（分组排行）

| 字段 | 类型 | 说明 |
|------|------|------|
| `group_id` | int64 | 分组 ID |
| `group_name` | string | 分组名称 |
| `request_count` | int64 | 该分组总请求数 |
| `token_consumed` | int64 | 该分组总 Token 消耗 |

---

## 3. ErrorTrend（错误趋势）

### 3.1 顶层字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `bucket` | string | 数据分桶粒度（秒数） |
| `points` | array | 时间序列数据点数组 |

### 3.2 ErrorTrendPoint（每个数据点）

| 字段 | 类型 | 说明 |
|------|------|------|
| `bucket_start` | ISO8601 时间 | 该桶的起始时间 |
| `error_count_total` | int64 | 该窗口内错误请求总数 |
| `business_limited_count` | int64 | 该窗口内业务限制拒绝数 |
| `error_count_sla` | int64 | SLA 失败计数 |
| `upstream_error_count_excl_429_529` | int64 | 该窗口内上游其他错误数（不含 429/529） |
| `upstream_429_count` | int64 | 该窗口内上游 429 次数 |
| `upstream_529_count` | int64 | 该窗口内上游 529 次数 |

---

## 完整响应示例

```json
{
  "data": {
    "generated_at": "2025-03-24T10:00:00Z",
    "overview": {
      "health_score": 98,
      "system_metrics": {
        "cpu_usage_percent": 45.2,
        "memory_used_mb": 2048,
        "memory_total_mb": 8192,
        "memory_usage_percent": 25.0,
        "db_ok": true,
        "redis_ok": true,
        "db_conn_active": 12,
        "db_conn_idle": 8,
        "db_conn_waiting": 0,
        "db_max_open_conns": 100,
        "redis_conn_total": 15,
        "redis_conn_idle": 10,
        "redis_pool_size": 100,
        "goroutine_count": 248,
        "concurrency_queue_depth": 5,
        "account_switch_count": 120
      },
      "job_heartbeats": [
        {
          "job_name": "usage-aggregator",
          "last_run_at": "2025-03-24T09:59:00Z",
          "last_success_at": "2025-03-24T09:59:00Z",
          "last_error_at": null,
          "last_error": null,
          "last_duration_ms": 3200,
          "last_result": "aggregated 1500 records"
        }
      ],
      "success_count": 125000,
      "error_count_total": 320,
      "business_limited_count": 45,
      "error_count_sla": 180,
      "request_count_total": 125320,
      "request_count_sla": 125180,
      "token_consumed": 850000000,
      "sla": 0.9986,
      "error_rate": 0.0026,
      "upstream_error_rate": 0.0014,
      "upstream_429_count": 50,
      "upstream_529_count": 2,
      "upstream_error_count_excl_429_529": 128,
      "qps": {
        "current": 42.5,
        "peak": 128.0,
        "avg": 35.2
      },
      "tps": {
        "current": 8500.0,
        "peak": 22000.0,
        "avg": 7200.0
      },
      "duration": {
        "p50_ms": 320,
        "p90_ms": 850,
        "p95_ms": 1200,
        "p99_ms": 2500,
        "avg_ms": 480,
        "max_ms": 8500
      },
      "ttft": {
        "p50_ms": 180,
        "p90_ms": 450,
        "p95_ms": 620,
        "p99_ms": 1100
      }
    },
    "throughput_trend": {
      "bucket": "60",
      "points": [
        {
          "bucket_start": "2025-03-24T09:00:00Z",
          "request_count": 2100,
          "token_consumed": 14400000,
          "switch_count": 2,
          "qps": 35.0,
          "tps": 7200.0
        },
        {
          "bucket_start": "2025-03-24T09:01:00Z",
          "request_count": 2150,
          "token_consumed": 14700000,
          "switch_count": 1,
          "qps": 35.8,
          "tps": 7350.0
        }
      ],
      "by_platform": [
        { "platform": "anthropic", "request_count": 80000, "token_consumed": 550000000 },
        { "platform": "openai", "request_count": 45320, "token_consumed": 300000000 }
      ]
    },
    "error_trend": {
      "bucket": "60",
      "points": [
        {
          "bucket_start": "2025-03-24T09:00:00Z",
          "error_count_total": 5,
          "business_limited_count": 1,
          "error_count_sla": 3,
          "upstream_error_count_excl_429_529": 2,
          "upstream_429_count": 1,
          "upstream_529_count": 0
        },
        {
          "bucket_start": "2025-03-24T09:01:00Z",
          "error_count_total": 4,
          "business_limited_count": 0,
          "error_count_sla": 2,
          "upstream_error_count_excl_429_529": 2,
          "upstream_429_count": 0,
          "upstream_529_count": 0
        }
      ]
    }
  }
}
```

---

## 调用示例

### 基础调用（最近 1 小时）

```bash
curl "http://localhost:8080/api/v1/admin/ops/dashboard/snapshot-v2" \
  -H "x-api-key: <admin-key>"
```

### 指定时间范围

```bash
curl "http://localhost:8080/api/v1/admin/ops/dashboard/snapshot-v2?start_time=2025-03-24T00:00:00Z&end_time=2025-03-24T12:00:00Z" \
  -H "x-api-key: <admin-key>"
```

### 按平台筛选

```bash
curl "http://localhost:8080/api/v1/admin/ops/dashboard/snapshot-v2?platform=anthropic" \
  -H "x-api-key: <admin-key>"
```

### 条件请求（使用 ETag 节省带宽）

```bash
# 首次请求获取 ETag
curl -I "http://localhost:8080/api/v1/admin/ops/dashboard/snapshot-v2" \
  -H "x-api-key: <admin-key>"
# 返回: ETag: "abc123"

# 后续请求带上 If-None-Match，数据无变化时返回 304
curl "http://localhost:8080/api/v1/admin/ops/dashboard/snapshot-v2" \
  -H "x-api-key: <admin-key>" \
  -H "If-None-Match: abc123"
```

---

## 分桶规则

系统根据时间窗口自动选择分桶粒度：

| 时间窗口长度 | 分桶粒度 |
|-------------|----------|
| ≤ 2 小时 | 每 60 秒（1 分钟）一个点 |
| ≤ 24 小时 | 每 300 秒（5 分钟）一个点 |
| > 24 小时 | 每 3600 秒（1 小时）一个点 |

---

## 注意事项

1. **监控功能需开启**：该接口依赖后台监控服务（OpsService），若未启用则返回 `Ops service not available`（503）。
2. **缓存机制**：30 秒缓存意味着同一参数的请求在 30 秒内只会真实执行一次，可用于前端轮询。
3. **数据来源**：系统指标（CPU/内存/DB/Redis）通过 `gopsutil` 采集；业务指标来自数据库查询。
4. **Pre-aggregated 降级**：若预聚合表未准备好（`OPS_PREAGG_NOT_READY`），系统会自动回退到 raw 模式查询。
