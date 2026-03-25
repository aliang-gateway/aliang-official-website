import { asRecord, asString } from "@/lib/api-response";

type UnknownRecord = Record<string, unknown>;

export type DashboardStatsAdapter = {
  today_requests: number;
  today_cost: number;
  today_tokens: number;
  total_tokens: number;
};

export type DashboardTrendRowAdapter = {
  date: string;
  requests: number;
  input_tokens: number;
  output_tokens: number;
  cache_creation_tokens: number;
  cache_read_tokens: number;
  total_tokens: number;
  cost: number;
  actual_cost: number;
};

export type DashboardTrendEnvelopeAdapter = {
  trend: DashboardTrendRowAdapter[];
  granularity: string;
  start_date: string;
  end_date: string;
};

export type DashboardModelRowAdapter = {
  model: string;
  requests: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost: number;
  actual_cost: number;
};

export type DashboardModelsEnvelopeAdapter = {
  models: DashboardModelRowAdapter[];
  start_date: string;
  end_date: string;
};

export type DashboardUsageRowAdapter = {
  id: number;
  request_id: string;
  model: string;
  inbound_endpoint: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  total_cost: number;
  actual_cost: number;
  duration_ms: number;
  created_at: string;
  request_type: string;
  stream: boolean;
};

export type DashboardUsagePaginationAdapter = {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
  has_next: boolean;
  has_prev: boolean;
};

export type DashboardUsageEnvelopeAdapter = {
  data: DashboardUsageRowAdapter[];
  pagination: DashboardUsagePaginationAdapter;
};

export type DashboardProfileAdapter = {
  balance: number;
};

export type DashboardSimpleTrendPointAdapter = {
  bucket_start: string;
  value: number;
};

export type DashboardTrendMetric = "auto" | "requests" | "total_tokens" | "cost" | "actual_cost";

function asNumber(value: unknown, fallback = 0) {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function asBoolean(value: unknown, fallback = false) {
  return typeof value === "boolean" ? value : fallback;
}

function pickFirstFiniteNumber(...values: unknown[]) {
  for (const value of values) {
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
  }
  return Number.NaN;
}

function extractEnvelopeData(payload: unknown) {
  const root = asRecord(payload);
  if (!root || !("data" in root)) {
    return null;
  }
  return root.data;
}

function mapTrendMetricValue(item: UnknownRecord, metric: DashboardTrendMetric) {
  if (metric === "requests") {
    return pickFirstFiniteNumber(item.requests, item.request_count, item.count, item.value, item.total_tokens);
  }
  if (metric === "total_tokens") {
    return pickFirstFiniteNumber(item.total_tokens, item.value, item.tokens, item.requests, item.request_count);
  }
  if (metric === "cost") {
    return pickFirstFiniteNumber(item.cost, item.actual_cost, item.value);
  }
  if (metric === "actual_cost") {
    return pickFirstFiniteNumber(item.actual_cost, item.cost, item.value);
  }

  return pickFirstFiniteNumber(item.value, item.count, item.requests, item.request_count, item.total_tokens, item.tokens);
}

function parseLegacyTrendPoints(payload: unknown, metric: DashboardTrendMetric): DashboardSimpleTrendPointAdapter[] {
  const record = asRecord(payload);
  const pointsRaw = Array.isArray(record?.points) ? record.points : Array.isArray(payload) ? payload : [];

  return pointsRaw
    .map((point) => asRecord(point))
    .filter((point): point is UnknownRecord => Boolean(point))
    .map((point) => ({
      bucket_start:
        asString(point.bucket_start) ||
        asString(point.bucket) ||
        asString(point.date) ||
        asString(point.timestamp) ||
        asString(point.time),
      value: mapTrendMetricValue(point, metric),
    }))
    .filter((point) => point.bucket_start && Number.isFinite(point.value));
}

function parsePaginationRoot(payload: unknown) {
  const root = asRecord(payload);
  const pagination = asRecord(root?.pagination);
  const meta = asRecord(root?.meta);
  const metaPagination = asRecord(meta?.pagination);

  return {
    root,
    pagination,
    meta,
    metaPagination,
  };
}

export function parseDashboardStatsEnvelope(payload: unknown): DashboardStatsAdapter {
  const envelope = asRecord(extractEnvelopeData(payload));
  const fallbackRoot = asRecord(payload);
  const source = envelope ?? fallbackRoot;

  return {
    today_requests: asNumber(source?.today_requests),
    today_cost: pickFirstFiniteNumber(source?.today_actual_cost, source?.today_cost, 0),
    today_tokens: asNumber(source?.today_tokens),
    total_tokens: asNumber(source?.total_tokens),
  };
}

export function parseDashboardTrendEnvelope(payload: unknown): DashboardTrendEnvelopeAdapter {
  const envelope = asRecord(extractEnvelopeData(payload));
  const fallbackRoot = asRecord(payload);
  const source = envelope ?? fallbackRoot;
  const trendRaw = Array.isArray(source?.trend)
    ? source.trend
    : Array.isArray(payload)
      ? payload
      : [];

  const trend = trendRaw
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map<DashboardTrendRowAdapter>((item) => ({
      date: asString(item.date) || asString(item.bucket_start) || asString(item.bucket) || asString(item.timestamp),
      requests: asNumber(item.requests),
      input_tokens: asNumber(item.input_tokens),
      output_tokens: asNumber(item.output_tokens),
      cache_creation_tokens: asNumber(item.cache_creation_tokens),
      cache_read_tokens: asNumber(item.cache_read_tokens),
      total_tokens: pickFirstFiniteNumber(item.total_tokens, asNumber(item.input_tokens) + asNumber(item.output_tokens), 0),
      cost: asNumber(item.cost),
      actual_cost: pickFirstFiniteNumber(item.actual_cost, item.cost, 0),
    }))
    .filter((item) => item.date);

  return {
    trend,
    granularity: asString(source?.granularity, "day"),
    start_date: asString(source?.start_date),
    end_date: asString(source?.end_date),
  };
}

export function parseDashboardModelsEnvelope(payload: unknown): DashboardModelsEnvelopeAdapter {
  const envelope = asRecord(extractEnvelopeData(payload));
  const fallbackRoot = asRecord(payload);
  const source = envelope ?? fallbackRoot;
  const modelsRaw = Array.isArray(source?.models)
    ? source.models
    : Array.isArray(payload)
      ? payload
      : [];

  const models = modelsRaw
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map<DashboardModelRowAdapter>((item) => ({
      model: asString(item.model) || asString(item.name),
      requests: asNumber(item.requests),
      input_tokens: asNumber(item.input_tokens),
      output_tokens: asNumber(item.output_tokens),
      total_tokens: pickFirstFiniteNumber(item.total_tokens, asNumber(item.input_tokens) + asNumber(item.output_tokens), 0),
      cost: asNumber(item.cost),
      actual_cost: pickFirstFiniteNumber(item.actual_cost, item.cost, 0),
    }))
    .filter((item) => item.model);

  return {
    models,
    start_date: asString(source?.start_date),
    end_date: asString(source?.end_date),
  };
}

export function parseDashboardUsageEnvelope(payload: unknown): DashboardUsageEnvelopeAdapter {
  const envelopeData = extractEnvelopeData(payload);
  const envelopeRecord = asRecord(envelopeData);
  const root = asRecord(payload);

  const listSource = Array.isArray(envelopeData)
    ? envelopeData
    : Array.isArray(envelopeRecord?.data)
      ? envelopeRecord.data
      : Array.isArray(envelopeRecord?.items)
        ? envelopeRecord.items
        : Array.isArray(envelopeRecord?.records)
          ? envelopeRecord.records
          : Array.isArray(payload)
            ? payload
            : [];

  const data = listSource
    .map((item) => asRecord(item))
    .filter((item): item is UnknownRecord => Boolean(item))
    .map<DashboardUsageRowAdapter>((item) => {
      const totalTokens = pickFirstFiniteNumber(
        item.total_tokens,
        item.tokens,
        asNumber(item.input_tokens) + asNumber(item.output_tokens),
      );

      return {
        id: asNumber(item.id),
        request_id: asString(item.request_id) || asString(item.id) || "--",
        model: asString(item.model),
        inbound_endpoint: asString(item.inbound_endpoint) || asString(item.endpoint) || asString(item.path),
        input_tokens: asNumber(item.input_tokens),
        output_tokens: asNumber(item.output_tokens),
        total_tokens: Number.isFinite(totalTokens) ? totalTokens : 0,
        total_cost: asNumber(item.total_cost),
        actual_cost: pickFirstFiniteNumber(item.actual_cost, item.total_cost, 0),
        duration_ms: pickFirstFiniteNumber(item.duration_ms, item.latency_ms, item.latency, 0),
        created_at: asString(item.created_at) || asString(item.occurred_at),
        request_type: asString(item.request_type),
        stream: asBoolean(item.stream),
      };
    });

  const { pagination, meta, metaPagination } = parsePaginationRoot(payload);
  const page = Math.max(
    1,
    Math.trunc(
      pickFirstFiniteNumber(root?.page, pagination?.page, meta?.page, metaPagination?.page, 1),
    ),
  );
  const per_page = Math.max(
    1,
    Math.trunc(
      pickFirstFiniteNumber(root?.per_page, root?.page_size, pagination?.per_page, pagination?.page_size, meta?.per_page, meta?.page_size, metaPagination?.per_page, metaPagination?.page_size, 20),
    ),
  );
  const total = Math.max(
    0,
    Math.trunc(
      pickFirstFiniteNumber(root?.total, root?.total_count, pagination?.total, pagination?.total_count, meta?.total, meta?.total_count, metaPagination?.total, metaPagination?.total_count, data.length),
    ),
  );
  const total_pages = Math.max(
    1,
    Math.trunc(
      pickFirstFiniteNumber(
        root?.total_pages,
        pagination?.total_pages,
        meta?.total_pages,
        metaPagination?.total_pages,
        Math.ceil(total / per_page),
      ),
    ),
  );

  return {
    data,
    pagination: {
      page,
      per_page,
      total,
      total_pages,
      has_next: page < total_pages,
      has_prev: page > 1,
    },
  };
}

export function parseDashboardProfileEnvelope(payload: unknown): DashboardProfileAdapter {
  const envelope = asRecord(extractEnvelopeData(payload));
  const root = asRecord(payload);

  const envelopeBalance = envelope?.balance;
  const rootBalance = root?.balance;
  const balance =
    pickFirstFiniteNumber(
      envelopeBalance,
      rootBalance,
      asNumber(asRecord(envelopeBalance)?.amount),
      asNumber(asRecord(rootBalance)?.amount),
      0,
    ) || 0;

  return {
    balance,
  };
}

export function parseDashboardSimpleTrendPoints(payload: unknown, metric: DashboardTrendMetric = "auto") {
  const trendEnvelope = parseDashboardTrendEnvelope(payload);

  if (trendEnvelope.trend.length > 0) {
    return trendEnvelope.trend
      .map<DashboardSimpleTrendPointAdapter>((item) => ({
        bucket_start: item.date,
        value:
          metric === "requests"
            ? item.requests
            : metric === "total_tokens"
              ? pickFirstFiniteNumber(item.total_tokens, item.requests, 0)
              : metric === "cost"
                ? pickFirstFiniteNumber(item.cost, item.actual_cost, 0)
                : metric === "actual_cost"
                  ? pickFirstFiniteNumber(item.actual_cost, item.cost, 0)
                  : pickFirstFiniteNumber(item.requests, item.total_tokens, item.cost, 0),
      }))
      .filter((item) => item.bucket_start && Number.isFinite(item.value));
  }

  return parseLegacyTrendPoints(payload, metric);
}
