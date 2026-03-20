"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

type CreateUserResponse = {
  id: number;
  email: string;
  name: string;
  role: string;
  session_token: string;
};

type TierItem = {
  code: string;
  name: string;
  unit: string;
  included_units: number;
};

type Tier = {
  code: string;
  name: string;
  default_items: TierItem[];
};

type TiersResponse = {
  tiers: Tier[];
};

type CreateApiKeyResponse = {
  id: number;
  label: string;
  api_key: string;
  created_at: string;
};

type SubscriptionQuota = {
  service_item_code: string;
  service_item_name: string;
  unit: string;
  included_units: number;
};

type SubscriptionData = {
  tier_code: string;
  tier_name: string;
  quotas: SubscriptionQuota[];
};

type SubscriptionResponse = {
  subscription: SubscriptionData;
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

function getApiKeyIdsStorageKey(sessionToken: string) {
  return `api_key_ids:${sessionToken}`;
}

function parseApiKeyIds(raw: string | null) {
  if (!raw) {
    return [] as number[];
  }
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) {
      return [];
    }
    return parsed.filter((value): value is number => Number.isInteger(value) && value > 0);
  } catch {
    return [];
  }
}

export default function AccountPage() {
  const [isHydrated, setIsHydrated] = useState(false);
  const [sessionToken, setSessionToken] = useState("");

  const [createEmail, setCreateEmail] = useState("");
  const [createName, setCreateName] = useState("");
  const [createRole, setCreateRole] = useState("user");

  const [tiers, setTiers] = useState<Tier[]>([]);
  const [tiersLoading, setTiersLoading] = useState(true);
  const [tiersError, setTiersError] = useState<string | null>(null);
  const [selectedTierCode, setSelectedTierCode] = useState("");
  const [overrideInputs, setOverrideInputs] = useState<Record<string, string>>({});

  const [apiKeyLabel, setApiKeyLabel] = useState("");
  const [apiKeyIds, setApiKeyIds] = useState<number[]>([]);
  const [latestApiKey, setLatestApiKey] = useState<string | null>(null);

  const [authError, setAuthError] = useState<string | null>(null);
  const [apiKeyError, setApiKeyError] = useState<string | null>(null);
  const [subscriptionError, setSubscriptionError] = useState<string | null>(null);
  const [subscriptionSuccess, setSubscriptionSuccess] = useState<string | null>(null);

  const [currentSubscription, setCurrentSubscription] = useState<SubscriptionData | null>(null);
  const [subscriptionLoading, setSubscriptionLoading] = useState(false);

  useEffect(() => {
    setIsHydrated(true);
    const storedSessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    setSessionToken(storedSessionToken);
  }, []);

  useEffect(() => {
    const loadTiers = async () => {
      setTiersLoading(true);
      setTiersError(null);
      try {
        const response = await fetch("/api/public/tiers", {
          method: "GET",
          headers: { "content-type": "application/json", accept: "application/json" },
          cache: "no-store",
        });
        const payload = (await response.json()) as TiersResponse | { error?: string };
        if (!response.ok) {
          throw new Error((payload as { error?: string }).error ?? "加载套餐失败");
        }
        const nextTiers = (payload as TiersResponse).tiers ?? [];
        setTiers(nextTiers);
        if (nextTiers.length > 0) {
          setSelectedTierCode(nextTiers[0].code);
        }
      } catch (error) {
        setTiersError(error instanceof Error ? error.message : "加载套餐失败");
      } finally {
        setTiersLoading(false);
      }
    };

    void loadTiers();
  }, []);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }
    if (!sessionToken) {
      setApiKeyIds([]);
      return;
    }
    const key = getApiKeyIdsStorageKey(sessionToken);
    setApiKeyIds(parseApiKeyIds(localStorage.getItem(key)));
  }, [isHydrated, sessionToken]);

  const selectedTier = useMemo(
    () => tiers.find((tier) => tier.code === selectedTierCode) ?? null,
    [tiers, selectedTierCode],
  );

  useEffect(() => {
    if (!selectedTier) {
      setOverrideInputs({});
      return;
    }
    const nextDefaults: Record<string, string> = {};
    for (const item of selectedTier.default_items) {
      nextDefaults[item.code] = String(item.included_units);
    }
    setOverrideInputs(nextDefaults);
  }, [selectedTier]);

  const persistApiKeyIds = (nextIds: number[]) => {
    if (!sessionToken) {
      return;
    }
    localStorage.setItem(getApiKeyIdsStorageKey(sessionToken), JSON.stringify(nextIds));
    setApiKeyIds(nextIds);
  };

  const loadCurrentSubscription = useCallback(async () => {
    if (!sessionToken) {
      setCurrentSubscription(null);
      return;
    }

    setSubscriptionLoading(true);
    setSubscriptionError(null);
    try {
      const response = await fetch("/api/subscription", {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        cache: "no-store",
      });
      const payload = (await response.json()) as SubscriptionResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "加载订阅失败");
      }
      setCurrentSubscription((payload as SubscriptionResponse).subscription);
    } catch (error) {
      setCurrentSubscription(null);
      setSubscriptionError(error instanceof Error ? error.message : "加载订阅失败");
    } finally {
      setSubscriptionLoading(false);
    }
  }, [sessionToken]);

  useEffect(() => {
    if (!isHydrated || !sessionToken) {
      setCurrentSubscription(null);
      return;
    }
    void loadCurrentSubscription();
  }, [isHydrated, sessionToken, loadCurrentSubscription]);

  const handleCreateUser = async (event: { preventDefault: () => void }) => {
    event.preventDefault();
    setAuthError(null);

    try {
      const response = await fetch("/api/users", {
        method: "POST",
        headers: { "content-type": "application/json", accept: "application/json" },
        body: JSON.stringify({
          email: createEmail,
          name: createName,
          role: createRole,
        }),
      });

      const payload = (await response.json()) as CreateUserResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "创建用户失败");
      }

      const nextSessionToken = (payload as CreateUserResponse).session_token;
      localStorage.setItem(SESSION_TOKEN_STORAGE_KEY, nextSessionToken);
      setSessionToken(nextSessionToken);
      setSubscriptionSuccess(null);
      setLatestApiKey(null);
      void loadCurrentSubscription();
    } catch (error) {
      setAuthError(error instanceof Error ? error.message : "创建用户失败");
    }
  };

  const handleLogout = () => {
    localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
    setSessionToken("");
    setCurrentSubscription(null);
    setSubscriptionSuccess(null);
    setLatestApiKey(null);
    setAuthError(null);
    setApiKeyError(null);
    setSubscriptionError(null);
  };

  const handleCreateApiKey = async (event: { preventDefault: () => void }) => {
    event.preventDefault();
    setApiKeyError(null);
    setLatestApiKey(null);

    if (!sessionToken) {
      setApiKeyError("请先登录");
      return;
    }

    try {
      const response = await fetch("/api/api-keys", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({ label: apiKeyLabel }),
      });

      const payload = (await response.json()) as CreateApiKeyResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "创建 API 密钥失败");
      }

      const created = payload as CreateApiKeyResponse;
      setLatestApiKey(created.api_key);
      const nextIds = Array.from(new Set([...apiKeyIds, created.id]));
      persistApiKeyIds(nextIds);
    } catch (error) {
      setApiKeyError(error instanceof Error ? error.message : "创建 API 密钥失败");
    }
  };

  const handleRevokeApiKey = async (keyId: number) => {
    setApiKeyError(null);
    if (!sessionToken) {
      setApiKeyError("请先登录");
      return;
    }

    try {
      const response = await fetch(`/api/api-keys/${keyId}`, {
        method: "DELETE",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
      });
      const payload = (await response.json()) as { revoked?: boolean; error?: string };
      if (!response.ok) {
        throw new Error(payload.error ?? "撤销 API 密钥失败");
      }

      persistApiKeyIds(apiKeyIds.filter((id) => id !== keyId));
    } catch (error) {
      setApiKeyError(error instanceof Error ? error.message : "撤销 API 密钥失败");
    }
  };

  const handleSaveSubscription = async (event: { preventDefault: () => void }) => {
    event.preventDefault();
    setSubscriptionError(null);
    setSubscriptionSuccess(null);

    if (!sessionToken) {
      setSubscriptionError("请先登录");
      return;
    }
    if (!selectedTier) {
      setSubscriptionError("请选择套餐");
      return;
    }

    const overrides: Array<{ service_item_code: string; included_units: number }> = [];
    for (const item of selectedTier.default_items) {
      const raw = overrideInputs[item.code] ?? String(item.included_units);
      const parsed = Number.parseInt(raw, 10);
      if (!Number.isInteger(parsed) || parsed < 0) {
        setSubscriptionError(`${item.code} 的包含额度无效`);
        return;
      }
      if (parsed !== item.included_units) {
        overrides.push({
          service_item_code: item.code,
          included_units: parsed,
        });
      }
    }

    try {
      const response = await fetch("/api/subscription", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({
          tier_code: selectedTier.code,
          overrides,
        }),
      });
      const payload = (await response.json()) as SubscriptionResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "保存订阅失败");
      }
      setCurrentSubscription((payload as SubscriptionResponse).subscription);
      setSubscriptionSuccess("订阅已保存。");
    } catch (error) {
      setSubscriptionError(error instanceof Error ? error.message : "保存订阅失败");
    }
  };

  return (
    <section className="space-y-6">
      <div className="clay-panel space-y-2 p-5">
        <h2 className="section-title">
          <span className="gradient-text">账户管理</span>
        </h2>
        <p className="section-subtitle">
          会话认证、API 密钥管理和订阅控制的一站式工作区。
        </p>
      </div>

      <div className="block-card">
        <h3 className="mb-3 text-lg font-semibold text-emerald-500 dark:text-emerald-400">登录（会话令牌）</h3>

                <div className="mb-4 block-card p-3 text-sm">
          当前会话令牌：{" "}
          <span className="font-mono break-all">{isHydrated && sessionToken ? sessionToken : "（无）"}</span>
        </div>

        <form className="mb-4 grid gap-3" onSubmit={handleCreateUser}>
          <p className="text-sm font-semibold text-[var(--stitch-text-muted)]">创建用户</p>
          <input
            className="field"
            type="email"
            placeholder="邮箱"
            value={createEmail}
            onChange={(event) => setCreateEmail(event.target.value)}
          />
          <input
            className="field"
            type="text"
            placeholder="姓名"
            value={createName}
            onChange={(event) => setCreateName(event.target.value)}
          />
          <select className="field" value={createRole} onChange={(event) => setCreateRole(event.target.value)}>
            <option value="user">用户</option>
            <option value="admin">管理员</option>
          </select>
          <button type="submit" className="btn-primary w-fit">
            创建用户并保存会话令牌
          </button>
        </form>

        <button type="button" onClick={handleLogout} className="btn-ghost">
          退出登录
        </button>

        {authError ? <p className="mt-3 text-sm text-red-500 dark:text-red-400">{authError}</p> : null}
      </div>

      <div className="block-card">
        <h3 className="mb-3 text-lg font-semibold text-emerald-500 dark:text-emerald-400">API 密钥</h3>
        <form className="mb-3 flex flex-wrap items-end gap-3" onSubmit={handleCreateApiKey}>
          <div className="flex-1 space-y-1">
            <label htmlFor="api-key-label" className="text-sm font-medium">
              标签（可选）
            </label>
            <input
              id="api-key-label"
              className="field"
              type="text"
              value={apiKeyLabel}
              onChange={(event) => setApiKeyLabel(event.target.value)}
            />
          </div>
          <button type="submit" className="btn-primary">
            创建 API 密钥
          </button>
        </form>

        {latestApiKey ? (
          <p className="notice mb-3">
            新 API 密钥（仅显示一次）： <span className="font-mono break-all">{latestApiKey}</span>
          </p>
        ) : null}

        {apiKeyIds.length === 0 ? (
          <p className="text-sm text-[var(--stitch-text-muted)]">当前会话没有本地跟踪的密钥 ID。</p>
        ) : (
          <ul className="space-y-2 text-sm">
            {apiKeyIds.map((id) => (
              <li key={id} className="block-card flex items-center justify-between">
                <span className="font-mono">密钥 ID： {id}</span>
                <button type="button" onClick={() => void handleRevokeApiKey(id)} className="btn-ghost px-2 py-1 text-xs">
                  撤销
                </button>
              </li>
            ))}
          </ul>
        )}

        {apiKeyError ? <p className="mt-3 text-sm text-red-500 dark:text-red-400">{apiKeyError}</p> : null}
      </div>

      <div className="block-card">
        <h3 className="mb-3 text-lg font-semibold text-emerald-500 dark:text-emerald-400">订阅管理</h3>

        {tiersLoading ? <p className="text-sm">加载套餐中...</p> : null}
        {tiersError ? <p className="text-sm text-red-500 dark:text-red-400">加载套餐失败： {tiersError}</p> : null}

        {!tiersLoading && !tiersError ? (
          <form className="space-y-4" onSubmit={handleSaveSubscription}>
            <div className="space-y-2">
              <label htmlFor="tier" className="text-sm font-medium">
                套餐
              </label>
              <select id="tier" className="field" value={selectedTierCode} onChange={(event) => setSelectedTierCode(event.target.value)}>
                {tiers.map((tier) => (
                  <option key={tier.code} value={tier.code}>
                    {tier.name} ({tier.code})
                  </option>
                ))}
              </select>
            </div>

            {selectedTier ? (
              <div className="space-y-2">
                <p className="text-sm font-semibold text-[var(--stitch-text-muted)]">单项包含额度（覆盖默认值）</p>
                <ul className="space-y-2">
                  {selectedTier.default_items.map((item) => (
                    <li key={item.code} className="block-card">
                      <div className="mb-1 text-sm font-semibold">
                        {item.name} ({item.code})
                      </div>
                      <div className="flex items-center gap-2">
                        <input
                          type="number"
                          min={0}
                          className="field w-40"
                          value={overrideInputs[item.code] ?? ""}
                          onChange={(event) =>
                            setOverrideInputs((previous) => ({
                              ...previous,
                              [item.code]: event.target.value,
                            }))
                          }
                        />
                        <span className="text-sm text-[var(--stitch-text-muted)]">{item.unit}</span>
                      </div>
                        <div className="mt-1 text-xs text-[var(--stitch-text-muted)]">默认： {item.included_units}</div>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}

            <div className="flex flex-wrap gap-3">
              <button type="submit" className="btn-primary">
                保存订阅
              </button>
              <button type="button" onClick={() => void loadCurrentSubscription()} className="btn-ghost">
                重新加载当前订阅
              </button>
            </div>
          </form>
        ) : null}

        {subscriptionError ? <p className="mt-3 text-sm text-red-500 dark:text-red-400">{subscriptionError}</p> : null}
        {subscriptionSuccess ? <p className="mt-3 text-sm text-emerald-500 dark:text-emerald-400">{subscriptionSuccess}</p> : null}

        <div className="block-card mt-4 text-sm">
          <div className="mb-2 flex items-center justify-between gap-3">
            <p className="font-semibold">当前订阅</p>
            {subscriptionLoading ? <span className="text-xs">加载中...</span> : null}
          </div>
          {!currentSubscription ? (
            <p className="text-[var(--stitch-text-muted)]">没有加载活跃的订阅。</p>
          ) : (
            <div className="space-y-2">
              <p>
                套餐： {currentSubscription.tier_name} ({currentSubscription.tier_code})
              </p>
              <ul className="space-y-1">
                {currentSubscription.quotas.map((quota) => (
                  <li key={quota.service_item_code}>
                    {quota.service_item_name} ({quota.service_item_code}): {quota.included_units} {quota.unit}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
