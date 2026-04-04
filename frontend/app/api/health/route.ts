import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// --- Module-level singleton cache ---
let cachedHealthScore: number | null = null;
let lastError: string | null = null;
let lastUpdatedAt: number | 0 = 0;
let intervalHandle: ReturnType<typeof setInterval> | null = null;
let fetchPromise: Promise<void> | null = null;

const POLL_INTERVAL_MS = 2000; // 2s
const BACKEND_PATH = "/api/v1/admin/ops/dashboard/snapshot-v2";

async function fetchHealthScore() {
  try {
    const apiBaseUrl = getApiBaseUrl();
    const res = await fetch(`${apiBaseUrl}${BACKEND_PATH}`, {
      cache: "no-store",
      signal: AbortSignal.timeout(5000),
    });

    if (!res.ok) {
      lastError = `backend returned ${res.status}`;
      return;
    }

    const json = await res.json();
    const score = json?.data?.overview?.health_score;

    if (typeof score === "number") {
      cachedHealthScore = score;
      lastError = null;
      lastUpdatedAt = Date.now();
    } else {
      lastError = "health_score not found in response";
    }
  } catch (err) {
    lastError = err instanceof Error ? err.message : "fetch failed";
  }
}

function startPolling() {
  if (intervalHandle) return;

  // Fetch immediately on first call
  fetchPromise = fetchHealthScore();

  intervalHandle = setInterval(() => {
    fetchPromise = fetchHealthScore();
  }, POLL_INTERVAL_MS);
}

// Auto-start on module load
startPolling();

export async function GET() {
  // Wait for any in-flight fetch to finish so callers get fresh data when possible
  if (fetchPromise) {
    await fetchPromise;
  }

  return NextResponse.json({
    health_score: cachedHealthScore,
    updated_at: lastUpdatedAt ? new Date(lastUpdatedAt).toISOString() : null,
    error: lastError,
  });
}
