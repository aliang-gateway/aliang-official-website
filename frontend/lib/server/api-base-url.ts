import "server-only";

import { getRuntimeSettings, readNonEmptyEnv } from "@/lib/server/runtime-settings";

const API_BASE_URL_ENV_KEYS = ["API_BASE_URL", "NEXT_PUBLIC_API_BASE_URL"] as const;

export function getApiBaseUrl() {
  const fromEnv = readNonEmptyEnv(API_BASE_URL_ENV_KEYS);
  if (fromEnv) {
    return fromEnv.replace(/\/$/, "");
  }

  const fromConfig = getRuntimeSettings().apiBaseUrl?.trim();
  if (fromConfig) {
    return fromConfig.replace(/\/$/, "");
  }

  throw new Error(
    "API base URL is not configured (set API_BASE_URL / NEXT_PUBLIC_API_BASE_URL env, or api_base_url in config.json / config.yaml)",
  );
}
