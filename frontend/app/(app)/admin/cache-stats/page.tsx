"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";

import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asNumber } from "@/lib/api-keys";
import { asRecord } from "@/lib/api-response";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type GroupCacheRate = {
  group_id: number;
  group_name: string;
  rate_multiplier: number;
  requests: number;
  input_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
  cache_hit_rate: number;
};

type CacheStats = {
  days: number;
  start_date: string;
  end_date: string;
  requests: number;
  input_tokens: number;
  cache_read_tokens: number;
  cache_creation_tokens: number;
  cache_hit_rate: number;
  groups: GroupCacheRate[];
};

function formatInt(value: number): string {
  return new Intl.NumberFormat().format(value);
}

function formatRate(rate: number): string {
  return `${(rate * 100).toFixed(1)}%`;
}

function formatMultiplier(rate: number): string {
  if (!Number.isFinite(rate) || rate <= 0) return "×1";
  return `×${Number.isInteger(rate) ? rate : Number(rate.toFixed(2))}`;
}

export default function AdminCacheStatsPage() {
  const t = useTranslations("adminCacheStats");
  const router = useRouter();
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [days, setDays] = useState(3);
  const [stats, setStats] = useState<CacheStats | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async (sessionToken: string, selectedDays: number) => {
    setLoading(true);
    setError(null);
    try {
      const response = await fetch(`/api/admin/cache-stats?days=${selectedDays}`, {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        cache: "no-store",
      });
      if (response.status === 401 || response.status === 403) {
        router.replace("/admin");
        return;
      }
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(asRecord(payload)?.error ? String(asRecord(payload)?.error) : "failed");
      }
      const root = asRecord(payload);
      const groups = Array.isArray(root?.groups) ? root.groups : [];
      setStats({
        days: asNumber(root?.days, selectedDays),
        start_date: String(root?.start_date ?? ""),
        end_date: String(root?.end_date ?? ""),
        requests: asNumber(root?.requests),
        input_tokens: asNumber(root?.input_tokens),
        cache_read_tokens: asNumber(root?.cache_read_tokens),
        cache_creation_tokens: asNumber(root?.cache_creation_tokens),
        cache_hit_rate: asNumber(root?.cache_hit_rate),
        groups: groups
          .map((item) => asRecord(item))
          .filter((item): item is Record<string, unknown> => Boolean(item))
          .map((item) => ({
            group_id: asNumber(item.group_id),
            group_name: String(item.group_name ?? ""),
            rate_multiplier: asNumber(item.rate_multiplier, 1),
            requests: asNumber(item.requests),
            input_tokens: asNumber(item.input_tokens),
            cache_read_tokens: asNumber(item.cache_read_tokens),
            cache_creation_tokens: asNumber(item.cache_creation_tokens),
            cache_hit_rate: asNumber(item.cache_hit_rate),
          })),
      });
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : "failed");
    } finally {
      setLoading(false);
    }
  }, [router]);

  useEffect(() => {
    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    if (!sessionToken) {
      router.replace("/login");
      return;
    }
    setIsCheckingAuth(false);
  }, [router]);

  useEffect(() => {
    if (isCheckingAuth) return;
    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    void load(sessionToken, days);
  }, [isCheckingAuth, days, load]);

  if (isCheckingAuth) {
    return null;
  }

  return (
    <section className="portal-shell space-y-6 py-10">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-extrabold text-[var(--ink)]">{t("title")}</h1>
          {stats ? (
            <p className="mt-1 text-xs text-[var(--ink-muted)]">
              {t("dateRange", { start: stats.start_date, end: stats.end_date })}
            </p>
          ) : null}
        </div>
        <div className="flex items-center gap-2">
          {[3, 7].map((option) => (
            <button
              key={option}
              type="button"
              onClick={() => setDays(option)}
              className={`rounded-full border px-4 py-1.5 text-xs font-bold transition-colors ${
                days === option
                  ? "border-[var(--accent)] bg-[var(--accent-wash)] text-[var(--accent-ink)]"
                  : "border-[var(--line)] text-[var(--ink-muted)] hover:border-[var(--accent)]/40"
              }`}
            >
              {t("lastNDays", { count: option })}
            </button>
          ))}
          <button
            type="button"
            onClick={() => {
              const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
              void load(sessionToken, days);
            }}
            disabled={loading}
            className="inline-flex items-center gap-1 rounded-full border border-[var(--line)] px-3 py-1.5 text-xs font-bold text-[var(--ink)] disabled:opacity-40"
          >
            <MaterialIcon name={loading ? "hourglass_top" : "refresh"} size={14} />
            {t("refresh")}
          </button>
        </div>
      </div>

      {error ? <p className="notice">{t("errorPrefix")}{error}</p> : null}

      {stats ? (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            <div className="block-card">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("cacheHitRate")}</p>
              <p className="mt-2 text-3xl font-extrabold text-[var(--ink)]">{formatRate(stats.cache_hit_rate)}</p>
            </div>
            <div className="block-card">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("requests")}</p>
              <p className="mt-2 text-3xl font-extrabold text-[var(--ink)]">{formatInt(stats.requests)}</p>
            </div>
            <div className="block-card">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("cacheReadTokens")}</p>
              <p className="mt-2 text-3xl font-extrabold text-[var(--ink)]">{formatInt(stats.cache_read_tokens)}</p>
            </div>
            <div className="block-card">
              <p className="text-[10px] font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("inputTokens")}</p>
              <p className="mt-2 text-3xl font-extrabold text-[var(--ink)]">{formatInt(stats.input_tokens)}</p>
            </div>
          </div>

          <div className="overflow-x-auto rounded-[1rem] border border-[var(--line)]">
            <table className="w-full min-w-[720px] text-left">
              <thead>
                <tr className="border-b border-[var(--line)] bg-[var(--paper)]/60">
                  <th className="px-4 py-3 text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("group")}</th>
                  <th className="px-4 py-3 text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("multiplier")}</th>
                  <th className="px-4 py-3 text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("requests")}</th>
                  <th className="px-4 py-3 text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("cacheHitRate")}</th>
                  <th className="px-4 py-3 text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("cacheReadTokens")}</th>
                  <th className="px-4 py-3 text-xs font-bold uppercase tracking-[0.16em] text-[var(--ink-muted)]">{t("inputTokens")}</th>
                </tr>
              </thead>
              <tbody>
                {stats.groups.map((group) => (
                  <tr key={group.group_id} className="border-b border-[var(--line)] last:border-b-0">
                    <td className="px-4 py-3 text-sm font-semibold text-[var(--ink)]">{group.group_name || `#${group.group_id}`}</td>
                    <td className="px-4 py-3 text-sm text-[var(--ink-muted)]">{formatMultiplier(group.rate_multiplier)}</td>
                    <td className="px-4 py-3 text-sm text-[var(--ink)]">{formatInt(group.requests)}</td>
                    <td className="px-4 py-3 text-sm font-bold text-[var(--accent-ink)]">{formatRate(group.cache_hit_rate)}</td>
                    <td className="px-4 py-3 text-sm text-[var(--ink)]">{formatInt(group.cache_read_tokens)}</td>
                    <td className="px-4 py-3 text-sm text-[var(--ink)]">{formatInt(group.input_tokens)}</td>
                  </tr>
                ))}
                {stats.groups.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-sm text-[var(--ink-muted)]">{t("noData")}</td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </>
      ) : null}

      {loading && !stats ? <p className="text-sm text-[var(--ink-muted)]">{t("loading")}</p> : null}
    </section>
  );
}
