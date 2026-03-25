type UnknownRecord = Record<string, unknown>;

export function asRecord(value: unknown): UnknownRecord | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }

  return value as UnknownRecord;
}

export function asString(value: unknown, fallback = "") {
  return typeof value === "string" ? value : fallback;
}

export function unwrapData<T>(payload: unknown): T | null {
  const root = asRecord(payload);
  const data = asRecord(root?.data);
  if (!data) {
    return null;
  }

  return data as T;
}

export function extractApiError(payload: unknown, fallback: string) {
  const root = asRecord(payload);
  const data = asRecord(root?.data);

  return (
    asString(root?.message) ||
    asString(root?.code) ||
    asString(data?.message) ||
    asString(root?.error) ||
    fallback
  );
}
