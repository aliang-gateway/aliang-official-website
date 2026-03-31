import "server-only";

const API_BASE_URL_ENV_KEYS = ["API_BASE_URL", "NEXT_PUBLIC_API_BASE_URL"] as const;

export function getApiBaseUrl() {
  for (const key of API_BASE_URL_ENV_KEYS) {
    const baseUrl = process.env[key]?.trim();
    if (baseUrl) {
      return baseUrl.replace(/\/$/, "");
    }
  }

  throw new Error("API_BASE_URL or NEXT_PUBLIC_API_BASE_URL is not set");
}
