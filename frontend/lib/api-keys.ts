import { asRecord, asString } from "@/lib/api-response";

export type ApiKeyItem = {
  id: number;
  name: string;
  key: string;
  group_id: number;
  group_name: string;
  group_platform: string;
  /** 计费倍率：实际计费 = 原始费用 × 倍率。上游缺失时回退 1。 */
  group_rate_multiplier: number;
  status: string;
  quota: number;
  quota_used: number;
  expires_at: string;
  created_at: string;
};

export type AvailableGroup = {
  id: number;
  name: string;
  platform: string;
  rate_multiplier: number;
};

export type ApiKeyFormatFilter = "all" | "openai" | "anthropic";

export type PaginationInfo = {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
};

/** Name reserved for the auto-provisioned key that cannot be deleted/renamed. */
export const PROTECTED_API_KEY_NAME = "auto-key";

export function asNumber(value: unknown, fallback = 0): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return fallback;
}

export function maskApiKey(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return "***";
  if (trimmed.length <= 10) return `${trimmed.slice(0, 3)}***`;
  return `${trimmed.slice(0, 8)}***${trimmed.slice(-4)}`;
}

export function isProtectedApiKeyName(name: string): boolean {
  return name.trim() === PROTECTED_API_KEY_NAME;
}

export function normalizeGroupPlatform(platform: string): string {
  return platform.trim().toLowerCase();
}

export function platformBadgeLabel(platform: string): string {
  const normalized = normalizeGroupPlatform(platform);
  if (normalized === "openai") return "OpenAI";
  if (normalized === "anthropic") return "Anthropic";
  return platform || "其他";
}

export function matchesFormatFilter(platform: string, filter: ApiKeyFormatFilter): boolean {
  const normalized = normalizeGroupPlatform(platform);
  if (filter === "all") return true;
  if (filter === "openai") return normalized === "openai";
  if (filter === "anthropic") return normalized === "anthropic";
  return false;
}

export function authHeaders(sessionToken: string): Record<string, string> {
  return {
    "content-type": "application/json",
    accept: "application/json",
    Authorization: `Bearer ${sessionToken}`,
  };
}

export function parsePagination(payload: unknown): PaginationInfo {
  const root = asRecord(payload);
  const pag = asRecord(root?.pagination);
  const total = asNumber(pag?.total ?? root?.total);
  const page = Math.max(1, asNumber(pag?.page ?? root?.page, 1));
  const per_page = Math.max(1, asNumber(pag?.per_page ?? pag?.page_size ?? root?.per_page, 20));
  const total_pages = Math.max(1, asNumber(pag?.pages ?? root?.pages, Math.ceil(total / per_page)));
  return { page, per_page, total, total_pages, has_next: page < total_pages, has_prev: page > 1 };
}

/**
 * Parse the various shapes the upstream can return for GET /api-keys into a
 * flat ApiKeyItem[]. Tolerates data-wrapped, items/list/api_keys/array forms.
 */
export function parseApiKeysList(payload: unknown): ApiKeyItem[] {
  const root = asRecord(payload);
  const inner = asRecord(root?.data) ?? root;
  const list =
    Array.isArray(inner?.data) ? inner.data
      : Array.isArray(inner?.items) ? inner.items
        : Array.isArray(inner?.list) ? inner.list
          : Array.isArray(inner?.api_keys) ? inner.api_keys
            : Array.isArray(root?.data) ? root.data
              : Array.isArray(root?.items) ? root.items
                : [];
  return (Array.isArray(list) ? list : [])
    .map((item: unknown) => asRecord(item))
    .filter((item): item is Record<string, unknown> => Boolean(item))
    .map((item) => {
      const group = asRecord(item.group);
      return {
        id: asNumber(item.id),
        name: asString(item.name) || asString(item.label),
        key: asString(item.key) || asString(item.api_key),
        group_id: asNumber(item.group_id),
        group_name: asString(group?.name) || asString(item.group_name) || `Group #${item.group_id}`,
        group_platform: asString(group?.platform) || asString(item.group_platform),
        group_rate_multiplier: asNumber(group?.rate_multiplier ?? item.rate_multiplier, 1),
        status: asString(item.status, "active"),
        quota: asNumber(item.quota),
        quota_used: asNumber(item.quota_used),
        expires_at: asString(item.expires_at),
        created_at: asString(item.created_at),
      };
    });
}

/** Parse GET /api/groups/available (BFF) into AvailableGroup[]. */
export function parseAvailableGroups(payload: unknown): AvailableGroup[] {
  const root = asRecord(payload);
  const inner = asRecord(root?.data) ?? root;
  const list = Array.isArray(inner?.data)
    ? inner.data
    : Array.isArray(inner)
      ? inner
      : Array.isArray(root?.data)
        ? root.data
        : [];
  return (Array.isArray(list) ? list : [])
    .map((item: unknown) => asRecord(item))
    .filter((item): item is Record<string, unknown> => Boolean(item))
    .map((item) => ({
      id: asNumber(item.id),
      name: asString(item.name) || `Group #${item.id}`,
      platform: asString(item.platform) || asString(item.group_platform),
      rate_multiplier: asNumber(item.rate_multiplier, 1),
    }));
}

/** 渲染计费倍率徽章文案：×1、×1.5……异常值按无倍率（×1）处理。 */
export function formatRateMultiplier(rate: number): string {
  if (!Number.isFinite(rate) || rate <= 0) {
    return "×1";
  }
  return `×${Number.isInteger(rate) ? rate : Number(rate.toFixed(2))}`;
}

export type KeySelectOption = {
  value: string;
  label: string;
  hint: string;
};

/** 配置面板「选择已有密钥」下拉选项:label=名称·略写,hint=类型 倍率(缺失走各自回退)。 */
export function buildKeySelectOption(item: ApiKeyItem): KeySelectOption {
  const badge = item.group_platform.trim() ? platformBadgeLabel(item.group_platform) : "";
  const rate = formatRateMultiplier(item.group_rate_multiplier);
  return {
    value: String(item.id),
    label: `${item.name || `Key #${item.id}`} · ${maskApiKey(item.key)}`,
    hint: badge ? `${badge} ${rate}` : rate,
  };
}

/** /keys 密钥行按平台区分的附加类(globals.css 中 .clay-panel 之后定义的未分层类,
 *  能覆盖 .clay-panel 的 border/background);未知/缺失平台返回空串(默认样式)。 */
export function platformRowClasses(platform: string): string {
  const normalized = normalizeGroupPlatform(platform);
  if (normalized === "anthropic") {
    return "key-row-anthropic";
  }
  if (normalized === "openai") {
    return "key-row-openai";
  }
  return "";
}
