"use client";

import { useCallback, useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { asRecord, asString } from "@/lib/api-response";
import {
  normalizeModelShareData,
  parseDashboardHomePayload,
  parseDashboardMetricSummary,
  parseTokenTrendResponse,
} from "@/lib/dashboard-home";
import type {
  DashboardHomeResponse,
  DashboardMetricSummary,
  ModelShareDatum,
  TokenTrendResponse,
} from "@/lib/dashboard-types";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type ModelShare = { start_date: string; end_date: string; items: ModelShareDatum[] };

export type DashboardData = {
  isHydrated: boolean;
  sessionToken: string;
  loading: boolean;
  error: string | null;
  clearError: () => void;
  dashboard: DashboardHomeResponse | null;
  tokenTrend: TokenTrendResponse | null;
  modelShare: ModelShare | null;
  metricSummary: DashboardMetricSummary | null;
  loadDashboard: (signal?: AbortSignal) => Promise<void>;
  reload: () => void;
  signOut: () => void;
};

export function useDashboardData(queryString: string): DashboardData {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const [isHydrated, setIsHydrated] = useState(false);
  const [sessionToken, setSessionToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dashboard, setDashboard] = useState<DashboardHomeResponse | null>(null);
  const [tokenTrend, setTokenTrend] = useState<TokenTrendResponse | null>(null);
  const [modelShare, setModelShare] = useState<ModelShare | null>(null);
  const [metricSummary, setMetricSummary] = useState<DashboardMetricSummary | null>(null);

  useEffect(() => {
    setIsHydrated(true);
    const storedSessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    setSessionToken(storedSessionToken);
  }, []);

  useEffect(() => {
    if (!isHydrated || sessionToken) {
      return;
    }

    const query = searchParams.toString();
    const next = query ? `${pathname}?${query}` : pathname;
    router.replace(`/login?next=${encodeURIComponent(next)}`);
  }, [isHydrated, pathname, router, searchParams, sessionToken]);

  const loadDashboard = useCallback(
    async (signal?: AbortSignal) => {
      if (!sessionToken) {
        setDashboard(null);
        setTokenTrend(null);
        setModelShare(null);
        setMetricSummary(null);
        setLoading(false);
        return;
      }

      setLoading(true);
      setError(null);

      try {
        const commonRequestInit: RequestInit = {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
          signal,
        };

        const [homeResponse, subscriptionResponse, accountResponse, trendResponse, modelsResponse, groupsResponse] = await Promise.all([
          fetch("/api/dashboard/home", commonRequestInit),
          fetch("/api/subscriptions/summary", commonRequestInit),
          fetch("/api/dashboard/account", commonRequestInit),
          fetch(`/api/dashboard/trend?${queryString}`, commonRequestInit),
          fetch(`/api/dashboard/models?${queryString}`, commonRequestInit),
          fetch("/api/groups/available", commonRequestInit),
        ]);

        if (homeResponse.status === 401 || homeResponse.status === 403) {
          localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
          router.replace("/login");
          return;
        }

        const homePayload = (await homeResponse.json()) as unknown;
        if (!homeResponse.ok) {
          const errorPayload = asRecord(homePayload);
          throw new Error(asString(errorPayload?.error, "Failed to load dashboard home"));
        }

        const trendPayload = (await trendResponse.json()) as unknown;
        if (!trendResponse.ok) {
          const errorPayload = asRecord(trendPayload);
          throw new Error(asString(errorPayload?.error, "Failed to load dashboard token trend"));
        }

        const modelsPayload = (await modelsResponse.json()) as unknown;
        if (!modelsResponse.ok) {
          const errorPayload = asRecord(modelsPayload);
          throw new Error(asString(errorPayload?.error, "Failed to load dashboard model share"));
        }

        let subscriptionPayload: unknown = null;
        if (subscriptionResponse.ok) {
          subscriptionPayload = (await subscriptionResponse.json()) as unknown;
        }

        let accountPayload: unknown = null;
        if (accountResponse.ok) {
          accountPayload = (await accountResponse.json()) as unknown;
        }

        let groupsPayload: unknown = null;
        if (groupsResponse.ok) {
          groupsPayload = (await groupsResponse.json()) as unknown;
        }

        setDashboard(parseDashboardHomePayload(homePayload, subscriptionPayload, accountPayload, groupsPayload));
        setTokenTrend(parseTokenTrendResponse(trendPayload));
        setModelShare(normalizeModelShareData(modelsPayload));
        setMetricSummary(parseDashboardMetricSummary(homePayload, accountPayload));
      } catch (fetchError) {
        if ((fetchError as Error).name === "AbortError") {
          return;
        }
        setDashboard(null);
        setTokenTrend(null);
        setModelShare(null);
        setMetricSummary(null);
        setError(fetchError instanceof Error ? fetchError.message : "Failed to load dashboard home");
      } finally {
        if (!signal?.aborted) {
          setLoading(false);
        }
      }
    },
    [queryString, router, sessionToken],
  );

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    if (!sessionToken) {
      setDashboard(null);
      setTokenTrend(null);
      setModelShare(null);
      setMetricSummary(null);
      setLoading(false);
      return;
    }

    const controller = new AbortController();

    void loadDashboard(controller.signal);

    return () => controller.abort();
  }, [isHydrated, loadDashboard, sessionToken]);

  const clearError = useCallback(() => setError(null), []);

  const reload = useCallback(() => {
    void loadDashboard();
  }, [loadDashboard]);

  const signOut = useCallback(() => {
    localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
    setSessionToken("");
    router.replace("/login");
  }, [router]);

  return {
    isHydrated,
    sessionToken,
    loading,
    error,
    clearError,
    dashboard,
    tokenTrend,
    modelShare,
    metricSummary,
    loadDashboard,
    reload,
    signOut,
  };
}
