import { getApiBaseUrl } from "@/lib/server/api-base-url";

type UnknownRecord = Record<string, unknown>;

type UpstreamJsonResult = {
  ok: boolean;
  status: number;
  data: unknown | null;
};

type SubscriptionQuota = {
  service_item_code: string;
  service_item_name: string;
  unit: string;
  included_units: number;
};

type PackageSummary = {
  status: "active" | "unconfigured";
  tier_code: string | null;
  tier_name: string | null;
  quotas: SubscriptionQuota[];
};

type BalanceSummary = {
  balance_micros: number;
  currency: string;
  updated_at: string | null;
};

type PurchaseOptions = {
  package_purchase: {
    durations: Array<{
      code: "configured";
      label: string;
      days: number | null;
    }>;
    tiers: Array<{
      code: string;
      name: string;
    }>;
  };
  prepaid_topup: {
    entry_mode: "redeem_code";
    redeem_endpoint: "/api/wallet/redeem";
    currency_hint: string;
  };
};

type TrendSeries = {
  aggregation_owner: "dashboard_app";
  aggregation_reason: "upstream_raw_logs_incomplete";
  interval: "day";
  points: Array<{
    bucket_start: string;
    value: number;
  }>;
};

type RequestRecord = {
  request_id: string;
  occurred_at: string;
  endpoint: string;
  status: "success" | "error";
  request_count: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  latency_ms: number;
};

type RequestRecordList = {
  aggregation_owner: "dashboard_app";
  aggregation_reason: "upstream_raw_logs_incomplete";
  records: RequestRecord[];
};

type TicketSubmissionResult = {
  ok: boolean;
  submission_owner: "dashboard_app";
  result: {
    ticket_id: string;
    submitted_at: string;
    status: "submitted";
  };
};

function asRecord(value: unknown): UnknownRecord | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as UnknownRecord;
}

function asString(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function asNumber(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function asTrendSeries(): TrendSeries {
  return {
    aggregation_owner: "dashboard_app",
    aggregation_reason: "upstream_raw_logs_incomplete",
    interval: "day",
    points: [],
  };
}

function asRequestRecordList(): RequestRecordList {
  return {
    aggregation_owner: "dashboard_app",
    aggregation_reason: "upstream_raw_logs_incomplete",
    records: [],
  };
}

function normalizeSubscription(data: unknown): PackageSummary {
  const root = asRecord(data);
  const subscription = asRecord(root?.subscription);
  if (!subscription) {
    return {
      status: "unconfigured",
      tier_code: null,
      tier_name: null,
      quotas: [],
    };
  }

  const quotaRaw = Array.isArray(subscription.quotas) ? subscription.quotas : [];
  const quotas: SubscriptionQuota[] = quotaRaw
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map((item) => ({
      service_item_code: asString(item.service_item_code),
      service_item_name: asString(item.service_item_name),
      unit: asString(item.unit),
      included_units: asNumber(item.included_units),
    }))
    .filter((item) => item.service_item_code && item.service_item_name);

  return {
    status: "active",
    tier_code: asString(subscription.tier_code) || null,
    tier_name: asString(subscription.tier_name) || null,
    quotas,
  };
}

function normalizeBalance(data: unknown): BalanceSummary {
  const root = asRecord(data);
  const wallet = asRecord(root?.wallet);

  if (!wallet) {
    return {
      balance_micros: 0,
      currency: "CNY",
      updated_at: null,
    };
  }

  return {
    balance_micros: asNumber(wallet.balance_micros),
    currency: asString(wallet.currency, "CNY"),
    updated_at: asString(wallet.updated_at) || null,
  };
}

function normalizePurchaseOptions(data: unknown, currencyHint: string): PurchaseOptions {
  const root = asRecord(data);
  const tiersRaw = Array.isArray(root?.tiers) ? root.tiers : [];
  const tiers = tiersRaw
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map((item) => ({
      code: asString(item.code),
      name: asString(item.name),
    }))
    .filter((item) => item.code && item.name);

  return {
    package_purchase: {
      durations: [],
      tiers,
    },
    prepaid_topup: {
      entry_mode: "redeem_code",
      redeem_endpoint: "/api/wallet/redeem",
      currency_hint: currencyHint,
    },
  };
}

async function fetchUpstreamJson(params: {
  apiBaseUrl: string;
  path: string;
  method: "GET" | "POST";
  authorization: string;
}): Promise<UpstreamJsonResult> {
  try {
    const upstream = await fetch(`${params.apiBaseUrl}${params.path}`, {
      method: params.method,
      headers: {
        "content-type": "application/json",
        accept: "application/json",
        Authorization: params.authorization,
      },
      cache: "no-store",
    });

    try {
      const data = await upstream.json();
      return {
        ok: upstream.ok,
        status: upstream.status,
        data,
      };
    } catch {
      return {
        ok: upstream.ok,
        status: upstream.status,
        data: null,
      };
    }
  } catch {
    return {
      ok: false,
      status: 0,
      data: null,
    };
  }
}

function makeTicketSubmissionResult(nowISO: string): TicketSubmissionResult {
  return {
    ok: true,
    submission_owner: "dashboard_app",
    result: {
      ticket_id: `dashboard-ticket-${Date.now()}`,
      submitted_at: nowISO,
      status: "submitted",
    },
  };
}

export {
  asRequestRecordList,
  asTrendSeries,
  fetchUpstreamJson,
  getApiBaseUrl,
  makeTicketSubmissionResult,
  normalizeBalance,
  normalizePurchaseOptions,
  normalizeSubscription,
};

export type {
  BalanceSummary,
  PackageSummary,
  PurchaseOptions,
  RequestRecordList,
  TicketSubmissionResult,
  TrendSeries,
};
