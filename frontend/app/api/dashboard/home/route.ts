import { NextResponse } from "next/server";

import {
  asTrendSeries,
  fetchUpstreamJson,
  getApiBaseUrl,
  normalizeBalance,
  normalizePurchaseOptions,
  normalizeSubscription,
} from "../_shared";

type DashboardHomeResponse = {
  request_trend: ReturnType<typeof asTrendSeries>;
  token_trend: ReturnType<typeof asTrendSeries>;
  package_summary: ReturnType<typeof normalizeSubscription>;
  balance_summary: ReturnType<typeof normalizeBalance>;
  purchase_options: ReturnType<typeof normalizePurchaseOptions>;
};

export async function GET(request: Request) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const authorization = request.headers.get("Authorization") ?? "";

  const [subscriptionResult, walletResult, tiersResult] = await Promise.all([
    fetchUpstreamJson({
      apiBaseUrl,
      path: "/subscription",
      method: "GET",
      authorization,
    }),
    fetchUpstreamJson({
      apiBaseUrl,
      path: "/wallet",
      method: "GET",
      authorization,
    }),
    fetchUpstreamJson({
      apiBaseUrl,
      path: "/public/tiers",
      method: "GET",
      authorization: "",
    }),
  ]);

  const balanceSummary = normalizeBalance(walletResult.data);
  const response: DashboardHomeResponse = {
    request_trend: asTrendSeries(),
    token_trend: asTrendSeries(),
    package_summary: normalizeSubscription(subscriptionResult.data),
    balance_summary: balanceSummary,
    purchase_options: normalizePurchaseOptions(tiersResult.data, balanceSummary.currency),
  };

  return NextResponse.json(response, { status: 200 });
}
