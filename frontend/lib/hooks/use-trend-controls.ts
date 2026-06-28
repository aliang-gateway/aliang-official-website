"use client";

import { useCallback, useEffect, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { buildTrendDateRange, isTrendGranularity, isTrendRange, normalizeTrendGranularity } from "@/lib/dashboard-format";
import type { TrendGranularity, TrendRange } from "@/lib/dashboard-types";

const DASHBOARD_TREND_TIMEZONE = "Asia/Shanghai";

export type TrendControls = {
  selectedRange: TrendRange;
  selectedGranularity: TrendGranularity;
  appliedGranularity: TrendGranularity;
  trendDateRange: { start_date: string; end_date: string };
  queryString: string;
  updateSearchParams: (range: TrendRange, granularity: TrendGranularity, historyMode: "push" | "replace") => void;
};

export function useTrendControls(): TrendControls {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();

  const selectedRange = useMemo<TrendRange>(() => {
    const requestedRange = searchParams.get("range");
    return requestedRange && isTrendRange(requestedRange) ? requestedRange : "7d";
  }, [searchParams]);

  const selectedGranularity = useMemo<TrendGranularity>(() => {
    const requestedGranularity = searchParams.get("granularity");
    return requestedGranularity && isTrendGranularity(requestedGranularity) ? requestedGranularity : "day";
  }, [searchParams]);

  const appliedGranularity = useMemo(
    () => normalizeTrendGranularity(selectedRange, selectedGranularity),
    [selectedGranularity, selectedRange],
  );

  const updateSearchParams = useCallback(
    (range: TrendRange, granularity: TrendGranularity, historyMode: "push" | "replace") => {
      const nextParams = new URLSearchParams(searchParams.toString());
      nextParams.set("range", range);
      nextParams.set("granularity", normalizeTrendGranularity(range, granularity));

      const nextQuery = nextParams.toString();
      const nextHref = nextQuery ? `${pathname}?${nextQuery}` : pathname;

      if (historyMode === "replace") {
        router.replace(nextHref);
        return;
      }

      router.push(nextHref);
    },
    [pathname, router, searchParams],
  );

  const trendDateRange = useMemo(
    () => buildTrendDateRange(selectedRange, DASHBOARD_TREND_TIMEZONE),
    [selectedRange],
  );

  const queryString = useMemo(() => {
    const params = new URLSearchParams({
      start_date: trendDateRange.start_date,
      end_date: trendDateRange.end_date,
      granularity: appliedGranularity,
      timezone: DASHBOARD_TREND_TIMEZONE,
    });

    return params.toString();
  }, [appliedGranularity, trendDateRange.end_date, trendDateRange.start_date]);

  useEffect(() => {
    const currentRange = searchParams.get("range");
    const currentGranularity = searchParams.get("granularity");

    if (currentRange === selectedRange && currentGranularity === appliedGranularity) {
      return;
    }

    updateSearchParams(selectedRange, appliedGranularity, "replace");
  }, [appliedGranularity, searchParams, selectedRange, updateSearchParams]);

  return {
    selectedRange,
    selectedGranularity,
    appliedGranularity,
    trendDateRange,
    queryString,
    updateSearchParams,
  };
}
