"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";

import {
  aggregateCacheRate,
  formatCacheHitRate,
  parseDashboardTrendEnvelope,
} from "@/lib/dashboard-analytics-adapter";

// 首页缓存命中率卡：按当前所选时间范围拉取带缓存 token 的趋势行并聚合。
// 没有数据（范围内无请求）时整卡隐藏，不占版面。
export function CacheRateCard({
  sessionToken,
  queryString,
}: {
  sessionToken: string;
  queryString: string;
}) {
  const t = useTranslations("dashboard");
  const [rate, setRate] = useState<number | null>(null);

  useEffect(() => {
    if (!sessionToken) {
      return;
    }
    let cancelled = false;
    const controller = new AbortController();

    const load = async () => {
      try {
        const response = await fetch(`/api/dashboard/details?${queryString}`, {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) {
          return;
        }
        const payload = (await response.json()) as unknown;
        const envelope = parseDashboardTrendEnvelope(payload);
        if (!cancelled) {
          setRate(aggregateCacheRate(envelope.trend));
        }
      } catch (fetchError) {
        if ((fetchError as Error).name === "AbortError") {
          return;
        }
        // 静默：缓存率是锦上添花指标，取不到就不展示整卡。
      }
    };

    void load();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [sessionToken, queryString]);

  if (rate === null) {
    return null;
  }

  return (
    <div className="clay-panel flex flex-wrap items-center justify-between gap-4 px-6 py-4">
      <div>
        <p className="text-[11px] font-extrabold uppercase tracking-[0.2em] text-[var(--accent)]">{t("cacheHitRateLabel")}</p>
        <p className="mt-1 text-2xl font-extrabold tracking-tight text-[var(--ink)]">{formatCacheHitRate(rate)}</p>
      </div>
      <p className="max-w-md text-xs leading-relaxed text-[var(--ink-muted)]">{t("cacheRateHint")}</p>
    </div>
  );
}
