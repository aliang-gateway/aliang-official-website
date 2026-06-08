const CURRENCY_ENV_KEYS = ["NEXT_PUBLIC_STRIPE_CURRENCY", "STRIPE_CURRENCY"] as const;

export type StripeCurrency = "usd" | "cny";

function readCurrencyEnv(): string {
  for (const key of CURRENCY_ENV_KEYS) {
    const value = process.env[key]?.trim().toLowerCase();
    if (value) {
      return value;
    }
  }
  return "cny";
}

/** Normalized Stripe billing currency from env (defaults to CNY, matching backend). */
export function getStripeCurrency(): StripeCurrency {
  const raw = readCurrencyEnv();
  return raw === "usd" ? "usd" : "cny";
}

/** Price prefix: $ for USD, ¥ for CNY. */
export function getCurrencySymbol(): string {
  return getStripeCurrency() === "usd" ? "$" : "¥";
}

export function getIntlCurrencyCode(): "USD" | "CNY" {
  return getStripeCurrency() === "usd" ? "USD" : "CNY";
}

export function formatPriceWithSymbol(amount: string): string {
  return `${getCurrencySymbol()}${amount}`;
}

export function formatMetricCurrency(
  value: number | null,
  options?: { minimumFractionDigits?: number; maximumFractionDigits?: number },
): string {
  if (value === null) {
    return "--";
  }

  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: getIntlCurrencyCode(),
    minimumFractionDigits: options?.minimumFractionDigits ?? 2,
    maximumFractionDigits: options?.maximumFractionDigits ?? 2,
  }).format(value);
}
