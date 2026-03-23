"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";

const SESSION_TOKEN_STORAGE_KEY = "session_token";
const DASHBOARD_CONFIG_KEY_STORAGE_KEY = "dashboard_config_user_key";

type ClientTemplateId = "claude-code" | "codex" | "openai" | "gemini";
type TemplateFormat = "json" | "yaml" | "shell";

type CreateApiKeyResponse = {
  id: number;
  label: string;
  api_key: string;
  created_at: string;
};

type TrendPoint = {
  bucket_start: string;
  value: number;
};

type TrendSeries = {
  aggregation_owner: "dashboard_app";
  aggregation_reason: "upstream_raw_logs_incomplete";
  interval: "day";
  points: TrendPoint[];
};

type PackageQuota = {
  service_item_code: string;
  service_item_name: string;
  unit: string;
  included_units: number;
};

type PackageSummary = {
  status: "active" | "unconfigured";
  tier_code: string | null;
  tier_name: string | null;
  quotas: PackageQuota[];
};

type BalanceSummary = {
  balance_micros: number;
  currency: string;
  updated_at: string | null;
};

type PurchaseOptions = {
  package_purchase: {
    durations: Array<{
      code: "one_week" | "one_month" | "three_months";
      label: string;
      days: number;
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

type DashboardHomeResponse = {
  request_trend: TrendSeries;
  token_trend: TrendSeries;
  package_summary: PackageSummary;
  balance_summary: BalanceSummary;
  purchase_options: PurchaseOptions;
};

type PurchaseMessageTone = "success" | "error" | "info";
type TicketMessageTone = "success" | "error";

type TemplateDefinition = {
  id: ClientTemplateId;
  label: string;
  helper: string;
  supportedFormats: TemplateFormat[];
};

const TEMPLATE_DEFINITIONS: TemplateDefinition[] = [
  {
    id: "claude-code",
    label: "Claude Code",
    helper: "Quick terminal export for Anthropic-compatible CLI setup.",
    supportedFormats: ["shell"],
  },
  {
    id: "codex",
    label: "Codex",
    helper: "OpenAI-compatible config for Codex-style local tooling.",
    supportedFormats: ["json", "yaml"],
  },
  {
    id: "openai",
    label: "OpenAI",
    helper: "OpenAI SDK/client settings pointing at your routed gateway.",
    supportedFormats: ["json", "yaml"],
  },
  {
    id: "gemini",
    label: "Gemini",
    helper: "Gemini client bridge using the same single routed user key.",
    supportedFormats: ["json", "yaml"],
  },
];

function escapeJsonString(value: string) {
  return value.replaceAll("\\", "\\\\").replaceAll('"', '\\"');
}

function escapeSingleQuotedShell(value: string) {
  return value.replaceAll("'", "'\\''");
}

function formatYamlScalar(value: string) {
  return `'${value.replaceAll("'", "''")}'`;
}

function trimTrailingSlash(value: string) {
  return value.replace(/\/$/, "");
}

function buildTemplateContent(templateId: ClientTemplateId, format: TemplateFormat, userKey: string, gatewayBaseUrl: string) {
  const escapedKey = escapeJsonString(userKey);
  const escapedBaseUrl = escapeJsonString(gatewayBaseUrl);
  const yamlKey = formatYamlScalar(userKey);
  const yamlBaseUrl = formatYamlScalar(gatewayBaseUrl);

  if (templateId === "claude-code") {
    return [
      `export ANTHROPIC_BASE_URL='${escapeSingleQuotedShell(gatewayBaseUrl)}'`,
      `export ANTHROPIC_AUTH_TOKEN='${escapeSingleQuotedShell(userKey)}'`,
      "claude",
    ].join("\n");
  }

  if (templateId === "codex") {
    if (format === "yaml") {
      return [
        "provider: openai",
        `base_url: ${yamlBaseUrl}`,
        `api_key: ${yamlKey}`,
        "model: gpt-4.1",
      ].join("\n");
    }

    return [
      "{",
      '  "provider": "openai",',
      `  "base_url": "${escapedBaseUrl}",`,
      `  "api_key": "${escapedKey}",`,
      '  "model": "gpt-4.1"',
      "}",
    ].join("\n");
  }

  if (templateId === "openai") {
    if (format === "yaml") {
      return [
        "openai:",
        `  api_key: ${yamlKey}`,
        `  base_url: ${yamlBaseUrl}`,
        "  model: gpt-4.1-mini",
      ].join("\n");
    }

    return [
      "{",
      '  "openai": {',
      `    "api_key": "${escapedKey}",`,
      `    "base_url": "${escapedBaseUrl}",`,
      '    "model": "gpt-4.1-mini"',
      "  }",
      "}",
    ].join("\n");
  }

  if (format === "yaml") {
    return [
      "gemini:",
      `  api_key: ${yamlKey}`,
      `  base_url: ${yamlBaseUrl}`,
      "  model: gemini-2.5-pro",
    ].join("\n");
  }

  return [
    "{",
    '  "gemini": {',
    `    "api_key": "${escapedKey}",`,
    `    "base_url": "${escapedBaseUrl}",`,
    '    "model": "gemini-2.5-pro"',
    "  }",
    "}",
  ].join("\n");
}

function formatMoney(micros: number, currency: string) {
  return `${(micros / 1_000_000).toFixed(2)} ${currency}`;
}

function formatShortDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return new Intl.DateTimeFormat("en", { month: "short", day: "numeric" }).format(date);
}

function buildPreviewPoints(points: TrendPoint[], fallbackStep: number) {
  if (points.length > 0) {
    return points;
  }

  return Array.from({ length: 7 }, (_, index) => ({
    bucket_start: new Date(Date.now() - (6 - index) * 24 * 60 * 60 * 1000).toISOString(),
    value: fallbackStep * (index + 1),
  }));
}

function TrendPreview({
  points,
  tone,
}: {
  points: TrendPoint[];
  tone: "emerald" | "cyan";
}) {
  const preview = useMemo(() => buildPreviewPoints(points, tone === "emerald" ? 12 : 3200), [points, tone]);
  const maxValue = Math.max(...preview.map((point) => point.value), 1);

  const coordinates = preview
    .map((point, index) => {
      const x = (index / Math.max(preview.length - 1, 1)) * 100;
      const y = 100 - (point.value / maxValue) * 100;
      return `${x},${y}`;
    })
    .join(" ");

  const areaCoordinates = `${coordinates} 100,100 0,100`;
  const stroke = tone === "emerald" ? "#10b981" : "#06b6d4";
  return (
    <div className="mt-4 rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
      <svg viewBox="0 0 100 100" className="h-36 w-full overflow-visible" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id={`trend-fill-${tone}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor={stroke} stopOpacity="0.28" />
            <stop offset="100%" stopColor={stroke} stopOpacity="0.02" />
          </linearGradient>
        </defs>
        <path d={`M ${areaCoordinates}`} fill={`url(#trend-fill-${tone})`} />
        <polyline fill="none" stroke={stroke} strokeWidth="3" strokeLinejoin="round" strokeLinecap="round" points={coordinates} />
        {preview.map((point, index) => {
          const pointKey = `${point.bucket_start}-${point.value}`;
          const x = (index / Math.max(preview.length - 1, 1)) * 100;
          const y = 100 - (point.value / maxValue) * 100;
          return <circle key={pointKey} cx={x} cy={y} r="2.5" fill={stroke} />;
        })}
      </svg>
      <div className="mt-3 grid grid-cols-3 gap-2 text-xs text-[var(--portal-muted)] sm:grid-cols-7">
        {preview.map((point) => (
          <div key={`${point.bucket_start}-${point.value}-label`} className="min-w-0 rounded-2xl bg-white/50 px-2 py-1 text-center dark:bg-slate-950/30">
            {formatShortDate(point.bucket_start)}
          </div>
        ))}
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const router = useRouter();
  const [isHydrated, setIsHydrated] = useState(false);
  const [sessionToken, setSessionToken] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dashboard, setDashboard] = useState<DashboardHomeResponse | null>(null);
  const [isConfigModalOpen, setIsConfigModalOpen] = useState(false);
  const [selectedTemplate, setSelectedTemplate] = useState<ClientTemplateId>("claude-code");
  const [selectedFormat, setSelectedFormat] = useState<TemplateFormat>("shell");
  const [userKey, setUserKey] = useState("");
  const [isGeneratingKey, setIsGeneratingKey] = useState(false);
  const [keyError, setKeyError] = useState<string | null>(null);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "error">("idle");
  const [selectedPackageTierCode, setSelectedPackageTierCode] = useState("");
  const [selectedDurationCode, setSelectedDurationCode] = useState<PurchaseOptions["package_purchase"]["durations"][number]["code"] | "">("");
  const [packageActionLoading, setPackageActionLoading] = useState(false);
  const [redeemCode, setRedeemCode] = useState("");
  const [prepaidActionLoading, setPrepaidActionLoading] = useState(false);
  const [purchaseMessage, setPurchaseMessage] = useState<{ tone: PurchaseMessageTone; text: string } | null>(null);
  const [ticketTitle, setTicketTitle] = useState("");
  const [ticketCategory, setTicketCategory] = useState("delivery_issue");
  const [ticketMessage, setTicketMessage] = useState("");
  const [ticketSubmitting, setTicketSubmitting] = useState(false);
  const [ticketSubmitMessage, setTicketSubmitMessage] = useState<{ tone: TicketMessageTone; text: string } | null>(null);
  const modalRef = useRef<HTMLDivElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const configTriggerRef = useRef<HTMLButtonElement | null>(null);
  const hadConfigModalOpenRef = useRef(false);

  const gatewayBaseUrl = useMemo(() => trimTrailingSlash(process.env.NEXT_PUBLIC_API_BASE_URL?.trim() ?? "http://localhost:8080"), []);

  const selectedTemplateDefinition = useMemo(
    () => TEMPLATE_DEFINITIONS.find((template) => template.id === selectedTemplate) ?? TEMPLATE_DEFINITIONS[0],
    [selectedTemplate],
  );

  const renderedConfig = useMemo(() => {
    return buildTemplateContent(selectedTemplate, selectedFormat, userKey.trim(), gatewayBaseUrl);
  }, [gatewayBaseUrl, selectedFormat, selectedTemplate, userKey]);

  const closeConfigModal = useCallback(() => {
    setIsConfigModalOpen(false);
    setCopyState("idle");
  }, []);

  useEffect(() => {
    setIsHydrated(true);
    const storedSessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    const storedUserKey = localStorage.getItem(DASHBOARD_CONFIG_KEY_STORAGE_KEY) ?? "";
    setSessionToken(storedSessionToken);
    setUserKey(storedUserKey);
  }, []);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    localStorage.setItem(DASHBOARD_CONFIG_KEY_STORAGE_KEY, userKey);
  }, [isHydrated, userKey]);

  const loadDashboard = useCallback(async (signal?: AbortSignal) => {
    if (!sessionToken) {
      setDashboard(null);
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);

    try {
      const response = await fetch("/api/dashboard/home", {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        cache: "no-store",
        signal,
      });

      const payload = (await response.json()) as DashboardHomeResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "Failed to load dashboard home");
      }

      setDashboard(payload as DashboardHomeResponse);
    } catch (fetchError) {
      if ((fetchError as Error).name === "AbortError") {
        return;
      }
      setDashboard(null);
      setError(fetchError instanceof Error ? fetchError.message : "Failed to load dashboard home");
    } finally {
      if (!signal?.aborted) {
        setLoading(false);
      }
    }
  }, [sessionToken]);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }

    if (!sessionToken) {
      setDashboard(null);
      setLoading(false);
      return;
    }

    const controller = new AbortController();

    void loadDashboard(controller.signal);

    return () => controller.abort();
  }, [isHydrated, loadDashboard, sessionToken]);

  useEffect(() => {
    const tiers = dashboard?.purchase_options.package_purchase.tiers ?? [];
    if (tiers.length === 0) {
      if (selectedPackageTierCode) {
        setSelectedPackageTierCode("");
      }
      return;
    }

    const hasSelectedTier = tiers.some((tier) => tier.code === selectedPackageTierCode);
    if (!hasSelectedTier) {
      setSelectedPackageTierCode(tiers[0].code);
    }
  }, [dashboard, selectedPackageTierCode]);

  useEffect(() => {
    const durations = dashboard?.purchase_options.package_purchase.durations ?? [];
    if (durations.length === 0) {
      if (selectedDurationCode) {
        setSelectedDurationCode("");
      }
      return;
    }

    const hasSelectedDuration = durations.some((duration) => duration.code === selectedDurationCode);
    if (!hasSelectedDuration) {
      setSelectedDurationCode(durations[0].code);
    }
  }, [dashboard, selectedDurationCode]);

  useEffect(() => {
    const nextFormat = selectedTemplateDefinition.supportedFormats.includes(selectedFormat)
      ? selectedFormat
      : selectedTemplateDefinition.supportedFormats[0];

    if (nextFormat !== selectedFormat) {
      setSelectedFormat(nextFormat);
    }
  }, [selectedFormat, selectedTemplateDefinition]);

  useEffect(() => {
    if (!isConfigModalOpen) {
      if (hadConfigModalOpenRef.current) {
        configTriggerRef.current?.focus();
        hadConfigModalOpenRef.current = false;
      }
      return;
    }

    hadConfigModalOpenRef.current = true;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        closeConfigModal();
        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const modal = modalRef.current;
      if (!modal) {
        return;
      }

      const focusable = modal.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) {
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const activeElement = document.activeElement;

      if (event.shiftKey && activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    window.addEventListener("keydown", handleKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [closeConfigModal, isConfigModalOpen]);

  const handleGenerateKey = useCallback(async () => {
    setKeyError(null);
    setCopyState("idle");

    if (!sessionToken) {
      setKeyError("Your session token is missing. Sign in again before creating a new user key.");
      return;
    }

    setIsGeneratingKey(true);

    try {
      const response = await fetch("/api/api-keys", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({ label: "dashboard-config-modal" }),
      });

      const payload = (await response.json()) as CreateApiKeyResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "Failed to create a user key");
      }

      const createdKey = (payload as CreateApiKeyResponse).api_key;
      setUserKey(createdKey);
    } catch (createError) {
      setKeyError(createError instanceof Error ? createError.message : "Failed to create a user key");
    } finally {
      setIsGeneratingKey(false);
    }
  }, [sessionToken]);

  const handleCopyConfig = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(renderedConfig);
      setCopyState("copied");
    } catch {
      setCopyState("error");
    }
  }, [renderedConfig]);

  const handlePackagePurchase = useCallback(async () => {
    setPurchaseMessage(null);

    if (!sessionToken) {
      setPurchaseMessage({ tone: "error", text: "Your session token is missing. Sign in again before starting a package entry." });
      return;
    }

    const selectedTier = dashboard?.purchase_options.package_purchase.tiers.find((tier) => tier.code === selectedPackageTierCode);
    const selectedDuration = dashboard?.purchase_options.package_purchase.durations.find((duration) => duration.code === selectedDurationCode);

    if (!selectedTier || !selectedDuration) {
      setPurchaseMessage({ tone: "error", text: "Choose both a package tier and one duration before submitting the package entry." });
      return;
    }

    setPackageActionLoading(true);

    try {
      const response = await fetch("/api/subscription", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({
          tier_code: selectedTier.code,
          duration_code: selectedDuration.code,
          overrides: [],
        }),
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        throw new Error(payload.error ?? "Package entry is unavailable right now.");
      }

      setPurchaseMessage({
        tone: "success",
        text: `Package entry submitted for ${selectedTier.name} (${selectedDuration.label}). Your dashboard package summary was refreshed, but this screen does not claim payment completion or final billing settlement.`,
      });
      await loadDashboard();
    } catch (packageError) {
      setPurchaseMessage({
        tone: "error",
        text:
          packageError instanceof Error
            ? `Package entry could not be completed: ${packageError.message} You can still review plans on /services or retry later.`
            : "Package entry could not be completed. You can still review plans on /services or retry later.",
      });
    } finally {
      setPackageActionLoading(false);
    }
  }, [dashboard, loadDashboard, selectedDurationCode, selectedPackageTierCode, sessionToken]);

  const handlePrepaidTopUp = useCallback(async () => {
    setPurchaseMessage(null);

    if (!sessionToken) {
      setPurchaseMessage({ tone: "error", text: "Your session token is missing. Sign in again before redeeming prepaid credit." });
      return;
    }

    const normalizedCode = redeemCode.trim();
    const redeemEndpoint = dashboard?.purchase_options.prepaid_topup.redeem_endpoint ?? "/api/wallet/redeem";

    if (!normalizedCode) {
      setPurchaseMessage({ tone: "error", text: "Enter a redeem code before submitting a prepaid top-up attempt." });
      return;
    }

    setPrepaidActionLoading(true);

    try {
      const response = await fetch(redeemEndpoint, {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({ card_code: normalizedCode }),
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        throw new Error(payload.error ?? "Prepaid top-up is unavailable right now.");
      }

      setRedeemCode("");
      setPurchaseMessage({
        tone: "success",
        text: "Prepaid redeem request submitted. Your balance card was refreshed, and any upstream processing delay is surfaced here instead of being treated as instant payment completion.",
      });
      await loadDashboard();
    } catch (redeemError) {
      setPurchaseMessage({
        tone: "error",
        text:
          redeemError instanceof Error
            ? `Prepaid top-up could not be completed: ${redeemError.message} No balance was changed locally.`
            : "Prepaid top-up could not be completed. No balance was changed locally.",
      });
    } finally {
      setPrepaidActionLoading(false);
    }
  }, [dashboard, loadDashboard, redeemCode, sessionToken]);

  const handleTicketSubmit = useCallback(async () => {
    setTicketSubmitMessage(null);

    if (!sessionToken) {
      setTicketSubmitMessage({ tone: "error", text: "Your session token is missing. Sign in again before creating a feedback ticket." });
      return;
    }

    const normalizedTitle = ticketTitle.trim();
    const normalizedMessage = ticketMessage.trim();

    if (!normalizedTitle || !ticketCategory.trim() || !normalizedMessage) {
      setTicketSubmitMessage({ tone: "error", text: "Title, category, and message are required before submitting your feedback ticket." });
      return;
    }

    setTicketSubmitting(true);

    try {
      const response = await fetch("/api/dashboard/tickets", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({
          title: normalizedTitle,
          category: ticketCategory,
          message: normalizedMessage,
        }),
      });

      const payload = (await response.json()) as { error?: string; ticket_id?: string };
      if (!response.ok) {
        throw new Error(payload.error ?? "Ticket submission is unavailable right now.");
      }

      setTicketTitle("");
      setTicketCategory("delivery_issue");
      setTicketMessage("");
      setTicketSubmitMessage({
        tone: "success",
        text: `Feedback ticket submitted successfully${payload.ticket_id ? ` (ID: ${payload.ticket_id})` : ""}.`,
      });
    } catch (submitError) {
      setTicketSubmitMessage({
        tone: "error",
        text: submitError instanceof Error ? `Ticket submission failed: ${submitError.message}` : "Ticket submission failed.",
      });
    } finally {
      setTicketSubmitting(false);
    }
  }, [sessionToken, ticketCategory, ticketMessage, ticketTitle]);

  const packageSummary = dashboard?.package_summary;
  const balanceSummary = dashboard?.balance_summary;
  const purchaseOptions = dashboard?.purchase_options;
  const requestPoints = dashboard?.request_trend.points ?? [];
  const tokenPoints = dashboard?.token_trend.points ?? [];
  const quotaPreview = packageSummary?.quotas.slice(0, 3) ?? [];
  const packageTiers = purchaseOptions?.package_purchase.tiers ?? [];
  const packageDurations = purchaseOptions?.package_purchase.durations ?? [];
  const redeemEndpoint = purchaseOptions?.prepaid_topup.redeem_endpoint ?? "/api/wallet/redeem";
  const purchaseMessageClassName =
    purchaseMessage?.tone === "error"
      ? "text-red-500 dark:text-red-400"
      : purchaseMessage?.tone === "success"
        ? "text-emerald-500 dark:text-emerald-400"
        : "text-[var(--portal-muted)]";
  const ticketMessageClassName =
    ticketSubmitMessage?.tone === "error"
      ? "text-red-500 dark:text-red-400"
      : ticketSubmitMessage?.tone === "success"
        ? "text-emerald-500 dark:text-emerald-400"
        : "text-[var(--portal-muted)]";

  if (!isHydrated || loading) {
    return (
      <section className="portal-shell py-8">
        <div className="clay-panel p-5">
          <p className="text-sm text-[var(--portal-muted)]">Loading your dashboard...</p>
        </div>
      </section>
    );
  }

  if (!sessionToken) {
    return (
      <section className="portal-shell space-y-6 py-8">
        <div className="clay-panel space-y-2 p-5">
          <h1 className="section-title">
            <span className="gradient-text">Dashboard</span>
          </h1>
          <p className="section-subtitle">Sign in to see your request traffic, package status, and action entry points.</p>
        </div>

        <div className="block-card space-y-4">
          <p className="notice">Your session token is missing. Please sign in again to load your private dashboard surfaces.</p>
          <div className="flex flex-wrap gap-3">
            <Link href="/login" className="btn-primary inline-flex items-center justify-center no-underline">
              Go to login
            </Link>
            <Link href="/services" className="btn-ghost inline-flex items-center justify-center no-underline">
              View packages
            </Link>
          </div>
        </div>
      </section>
    );
  }

  return (
    <section className="portal-shell space-y-6 py-8">
      <div className="portal-header clay-panel p-5">
        <div className="min-w-0 space-y-2">
          <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">Simplified home</p>
          <h1 className="section-title">
            <span className="gradient-text">Usage dashboard</span>
          </h1>
          <p className="section-subtitle max-w-2xl">
            A lightweight home for request flow, token usage, package status, balance, and the next actions your account needs.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button type="button" className="btn-ghost" onClick={() => window.location.reload()}>
            Refresh
          </button>
          <button
            type="button"
            className="btn-primary"
            onClick={() => {
              localStorage.removeItem(SESSION_TOKEN_STORAGE_KEY);
              setSessionToken("");
              router.replace("/login");
            }}
          >
            Sign out
          </button>
        </div>
      </div>

      {error ? <p className="notice">Dashboard data is temporarily unavailable: {error}</p> : null}

      <div className="grid gap-6 lg:grid-cols-[minmax(0,1.6fr)_minmax(320px,1fr)]">
        <div className="grid min-w-0 gap-6">
          <div className="grid gap-6 xl:grid-cols-2">
            <article className="block-card min-w-0">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">Request trend</p>
                  <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Traffic pulse</h2>
                  <p className="mt-2 text-sm text-[var(--portal-muted)]">Daily request rhythm for the last visible buckets. Empty-safe when upstream logs are incomplete.</p>
                </div>
                <div className="rounded-full border border-emerald-500/20 bg-emerald-500/10 px-3 py-1 text-xs font-semibold text-emerald-600 dark:text-emerald-300">
                  {requestPoints.length > 0 ? `${requestPoints.length} points` : "preview"}
                </div>
              </div>
              <TrendPreview points={requestPoints} tone="emerald" />
            </article>

            <article className="block-card min-w-0">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-sm font-semibold text-cyan-500 dark:text-cyan-400">Token trend</p>
                  <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Consumption curve</h2>
                  <p className="mt-2 text-sm text-[var(--portal-muted)]">A lightweight view of token movement, kept inline to preserve the current dashboard rhythm.</p>
                </div>
                <div className="rounded-full border border-cyan-500/20 bg-cyan-500/10 px-3 py-1 text-xs font-semibold text-cyan-600 dark:text-cyan-300">
                  {tokenPoints.length > 0 ? `${tokenPoints.length} points` : "preview"}
                </div>
              </div>
              <TrendPreview points={tokenPoints} tone="cyan" />
            </article>
          </div>

          <div className="grid gap-6 md:grid-cols-2">
            <article className="block-card min-w-0 space-y-4">
              <div className="flex items-start justify-between gap-3">
                <div>
                  <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">Package</p>
                  <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">
                    {packageSummary?.tier_name ?? "No package yet"}
                  </h2>
                  <p className="mt-2 text-sm text-[var(--portal-muted)]">
                    {packageSummary?.status === "active"
                      ? `Current plan code: ${packageSummary.tier_code ?? "--"}`
                      : "Start with a package or prepaid balance to unlock routed usage."}
                  </p>
                </div>
                <span className="rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] px-3 py-1 text-xs font-semibold text-[var(--portal-muted)]">
                  {packageSummary?.status ?? "unconfigured"}
                </span>
              </div>

              {quotaPreview.length === 0 ? (
                <p className="rounded-[1rem] border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
                  No active quota has been loaded yet.
                </p>
              ) : (
                <ul className="grid gap-3">
                  {quotaPreview.map((quota) => (
                    <li key={quota.service_item_code} className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold text-[var(--portal-ink)]">{quota.service_item_name}</p>
                          <p className="mt-1 text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">{quota.service_item_code}</p>
                        </div>
                        <p className="text-sm font-semibold text-[var(--portal-ink)]">
                          {quota.included_units} {quota.unit}
                        </p>
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </article>

            <article className="block-card min-w-0 space-y-4">
              <div>
                <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">Balance</p>
                <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">
                  {balanceSummary ? formatMoney(balanceSummary.balance_micros, balanceSummary.currency) : "0.00 CNY"}
                </h2>
                <p className="mt-2 text-sm text-[var(--portal-muted)]">Keep prepaid funds ready for burst usage and package extensions.</p>
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">Currency</p>
                  <p className="mt-2 text-lg font-semibold text-[var(--portal-ink)]">{balanceSummary?.currency ?? "CNY"}</p>
                </div>
                <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                  <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">Updated</p>
                  <p className="mt-2 text-lg font-semibold text-[var(--portal-ink)]">
                    {balanceSummary?.updated_at ? formatShortDate(balanceSummary.updated_at) : "Not synced"}
                  </p>
                </div>
              </div>
            </article>
          </div>
        </div>

        <div className="grid min-w-0 gap-6">
          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">Config & API key</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Client setup entry</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">
                Prepare your single user key for Claude Code, Codex, OpenAI, and Gemini templates from one entry point.
              </p>
            </div>
            <div className="rounded-[1rem] border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
              Generate or paste one routed user key, then switch between Claude Code, Codex, OpenAI, and Gemini config views without leaving the dashboard.
            </div>
            <div className="flex flex-wrap gap-3">
              <button
                type="button"
                className="btn-primary"
                ref={configTriggerRef}
                onClick={() => {
                  setIsConfigModalOpen(true);
                  setKeyError(null);
                  setCopyState("idle");
                }}
              >
                Open config setup
              </button>
              <Link href="/account" className="btn-ghost inline-flex items-center justify-center no-underline">
                Manage session & keys
              </Link>
            </div>
          </article>

          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">Purchase</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Top up or extend</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">
                One entry surface for package purchase durations and prepaid redeem-code top-up.
              </p>
            </div>

            <div className="grid gap-3">
              <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">Package purchase</p>
                    <p className="mt-2 text-sm text-[var(--portal-muted)]">
                      Pick a visible duration first: 1 week, 1 month, or 3 months. This is an entry surface, so it submits the current tier setup without claiming a completed payment session.
                    </p>
                  </div>
                  <Link href="/services" className="btn-ghost inline-flex items-center justify-center no-underline">
                    Compare packages
                  </Link>
                </div>

                <div className="mt-4 grid gap-3">
                  <div>
                    <label htmlFor="dashboard-package-tier" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                      Package tier
                    </label>
                    <select
                      id="dashboard-package-tier"
                      className="field mt-2"
                      value={selectedPackageTierCode}
                      onChange={(event) => setSelectedPackageTierCode(event.target.value)}
                      disabled={packageTiers.length === 0 || packageActionLoading}
                    >
                      {packageTiers.length === 0 ? <option value="">No public tiers loaded</option> : null}
                      {packageTiers.map((tier) => (
                        <option key={tier.code} value={tier.code}>
                          {tier.name} ({tier.code})
                        </option>
                      ))}
                    </select>
                  </div>

                  <div>
                    <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">Package durations</p>
                    <div className="mt-3 flex flex-wrap gap-2">
                      {packageDurations.map((duration) => {
                        const isSelected = duration.code === selectedDurationCode;
                        return (
                          <button
                            key={duration.code}
                            type="button"
                            className={`cursor-pointer rounded-full border px-3 py-1 text-xs font-semibold transition-all duration-200 ${
                              isSelected
                                ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-200"
                                : "border-[var(--portal-line)] bg-white/60 text-[var(--portal-ink)] dark:bg-slate-950/30"
                            }`}
                            onClick={() => setSelectedDurationCode(duration.code)}
                            disabled={packageActionLoading}
                            aria-pressed={isSelected}
                          >
                            {duration.label}
                          </button>
                        );
                      })}
                    </div>
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <button
                      type="button"
                      className="btn-primary w-fit"
                      onClick={() => void handlePackagePurchase()}
                      disabled={packageActionLoading}
                    >
                      {packageActionLoading ? "Submitting package entry..." : "Start package entry"}
                    </button>
                  </div>
                </div>
              </div>

              <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">Prepaid top-up</p>
                <p className="mt-2 text-sm text-[var(--portal-ink)]">
                  Redeem-code endpoint: <span className="font-mono">{redeemEndpoint}</span>
                </p>
                <p className="mt-2 text-sm text-[var(--portal-muted)]">
                  Currency hint: {purchaseOptions?.prepaid_topup.currency_hint ?? "CNY"}. If redeem is unavailable upstream, this card shows a non-destructive error and keeps your current balance unchanged.
                </p>

                <div className="mt-4 grid gap-3">
                  <div>
                    <label htmlFor="dashboard-redeem-code" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                      Redeem code
                    </label>
                    <input
                      id="dashboard-redeem-code"
                      className="field mt-2 font-mono"
                      type="text"
                      placeholder="CARD-XXXX-XXXX"
                      value={redeemCode}
                      onChange={(event) => setRedeemCode(event.target.value)}
                      disabled={prepaidActionLoading}
                    />
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <button type="button" className="btn-primary w-fit" onClick={() => void handlePrepaidTopUp()} disabled={prepaidActionLoading}>
                      {prepaidActionLoading ? "Submitting top-up..." : "Redeem prepaid code"}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            {purchaseMessage ? <p className={`text-sm ${purchaseMessageClassName}`}>{purchaseMessage.text}</p> : null}
          </article>

          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">Ticket feedback</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Support entry</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">Capture delivery issues, model feedback, or billing questions from a single lightweight starting point.</p>
            </div>

            <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
              <div className="grid gap-3">
                <div>
                  <label htmlFor="dashboard-ticket-title" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                    Title
                  </label>
                  <input
                    id="dashboard-ticket-title"
                    className="field mt-2"
                    type="text"
                    maxLength={120}
                    placeholder="Short summary of the issue"
                    value={ticketTitle}
                    onChange={(event) => {
                      setTicketTitle(event.target.value);
                      setTicketSubmitMessage(null);
                    }}
                    disabled={ticketSubmitting}
                  />
                </div>

                <div>
                  <label htmlFor="dashboard-ticket-category" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                    Category
                  </label>
                  <select
                    id="dashboard-ticket-category"
                    className="field mt-2"
                    value={ticketCategory}
                    onChange={(event) => {
                      setTicketCategory(event.target.value);
                      setTicketSubmitMessage(null);
                    }}
                    disabled={ticketSubmitting}
                  >
                    <option value="delivery_issue">Delivery issue</option>
                    <option value="model_feedback">Model feedback</option>
                    <option value="billing_question">Billing question</option>
                    <option value="other">Other</option>
                  </select>
                </div>

                <div>
                  <label htmlFor="dashboard-ticket-message" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
                    Message
                  </label>
                  <textarea
                    id="dashboard-ticket-message"
                    className="field mt-2 min-h-[108px] resize-y"
                    placeholder="Describe what happened and what you expected."
                    value={ticketMessage}
                    onChange={(event) => {
                      setTicketMessage(event.target.value);
                      setTicketSubmitMessage(null);
                    }}
                    disabled={ticketSubmitting}
                  />
                </div>
              </div>
            </div>

            <div className="flex flex-wrap gap-3">
              <button type="button" className="btn-primary w-fit" onClick={() => void handleTicketSubmit()} disabled={ticketSubmitting}>
                {ticketSubmitting ? "Submitting ticket..." : "Create feedback ticket"}
              </button>
            </div>

            {ticketSubmitMessage ? <p className={`text-sm ${ticketMessageClassName}`}>{ticketSubmitMessage.text}</p> : null}
          </article>

          <article className="block-card min-w-0 space-y-4">
            <div>
              <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">Details</p>
              <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">Open deeper records</h2>
              <p className="mt-2 text-sm text-[var(--portal-muted)]">Move into the dedicated details page for request history, token trend depth, and API frequency analysis.</p>
            </div>
            <Link href="/dashboard/details" className="btn-primary inline-flex w-fit items-center justify-center no-underline">
              Go to details page
            </Link>
          </article>
        </div>
      </div>

      {isConfigModalOpen ? (
        <section
          className="fixed inset-0 z-[70] flex items-center justify-center p-4 sm:p-6"
          role="dialog"
          aria-modal="true"
          aria-labelledby="dashboard-config-modal-title"
        >
          <button
            type="button"
            className="absolute inset-0 bg-slate-950/60 backdrop-blur-sm"
            aria-label="Close config setup modal"
            onClick={closeConfigModal}
          />

          <div
            ref={modalRef}
            className="relative z-[1] flex max-h-[90vh] w-full max-w-5xl flex-col overflow-hidden rounded-[1.4rem] border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] shadow-[var(--portal-shadow)]"
          >
            <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--portal-line)] px-5 py-4 sm:px-6">
              <div className="min-w-0 space-y-2">
                <p className="text-xs font-semibold uppercase tracking-[0.22em] text-[var(--portal-muted)]">Config modal</p>
                <h2 id="dashboard-config-modal-title" className="text-2xl font-bold text-[var(--portal-ink)]">
                  Single key, four client templates
                </h2>
                <p className="max-w-2xl text-sm text-[var(--portal-muted)]">
                  One routed user key powers every template below. Copy the rendered config exactly as shown and treat it as sensitive because the real key is embedded.
                </p>
              </div>
              <button
                type="button"
                ref={closeButtonRef}
                className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] text-xl font-semibold text-[var(--portal-ink)] transition-transform duration-200 hover:-translate-y-[1px]"
                aria-label="Close config setup modal"
                onClick={closeConfigModal}
              >
                ×
              </button>
            </div>

            <div className="grid min-h-0 gap-0 overflow-y-auto lg:grid-cols-[280px_minmax(0,1fr)]">
              <div className="border-b border-[var(--portal-line)] bg-[var(--portal-clay)] p-5 lg:border-b-0 lg:border-r">
                <div className="space-y-4">
                  <div className="space-y-2">
                    <label htmlFor="dashboard-user-key" className="text-sm font-semibold text-[var(--portal-ink)]">
                      Underlying user key
                    </label>
                    <textarea
                      id="dashboard-user-key"
                      className="field min-h-[112px] resize-y font-mono text-sm"
                      placeholder="Paste a routed user key or mint a fresh one below"
                      value={userKey}
                      onChange={(event) => {
                        setUserKey(event.target.value);
                        setKeyError(null);
                        setCopyState("idle");
                      }}
                    />
                    <p className="text-xs leading-5 text-[var(--portal-muted)]">
                      This is the only key source for every template in the modal. New keys from the API are only shown once, so copy and store them safely.
                    </p>
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <button type="button" className="btn-primary" onClick={() => void handleGenerateKey()} disabled={isGeneratingKey}>
                      {isGeneratingKey ? "Creating key..." : "Create fresh key"}
                    </button>
                    <button type="button" className="btn-ghost" onClick={() => setUserKey("")}>
                      Clear key
                    </button>
                  </div>

                  {keyError ? <p className="notice">{keyError}</p> : null}

                  <div className="rounded-[1rem] border border-amber-400/40 bg-amber-50/80 p-4 text-sm text-amber-900 dark:bg-amber-500/10 dark:text-amber-200">
                    Sensitive-key warning: the rendered snippets below contain your real user key, not a placeholder. Avoid screenshots, shared terminals, and pasted logs.
                  </div>

                  <div className="space-y-2">
                    <p className="text-sm font-semibold text-[var(--portal-ink)]">Template</p>
                    <div className="grid gap-2">
                      {TEMPLATE_DEFINITIONS.map((template) => {
                        const isActive = template.id === selectedTemplate;
                        return (
                          <button
                            key={template.id}
                            type="button"
                            className={`rounded-[1rem] border px-4 py-3 text-left transition-all duration-200 ${
                              isActive
                                ? "border-emerald-500/40 bg-emerald-500/10 shadow-[0_12px_24px_rgba(16,185,129,0.12)]"
                                : "border-[var(--portal-line)] bg-[var(--portal-clay-strong)] hover:-translate-y-[1px]"
                            }`}
                            onClick={() => {
                              setSelectedTemplate(template.id);
                              setCopyState("idle");
                            }}
                          >
                            <p className="text-sm font-semibold text-[var(--portal-ink)]">{template.label}</p>
                            <p className="mt-1 text-xs leading-5 text-[var(--portal-muted)]">{template.helper}</p>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                </div>
              </div>

              <div className="flex min-h-0 flex-col p-5 sm:p-6">
                <div className="flex flex-wrap items-start justify-between gap-3 border-b border-[var(--portal-line)] pb-4">
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{selectedTemplateDefinition.label}</p>
                    <h3 className="mt-1 text-xl font-bold text-[var(--portal-ink)]">Rendered client config</h3>
                    <p className="mt-2 max-w-2xl text-sm text-[var(--portal-muted)]">{selectedTemplateDefinition.helper}</p>
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    {selectedTemplateDefinition.supportedFormats.map((format) => (
                      <button
                        key={format}
                        type="button"
                        className={`rounded-full border px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] transition-colors ${
                          selectedFormat === format
                            ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
                            : "border-[var(--portal-line)] bg-[var(--portal-clay)] text-[var(--portal-muted)]"
                        }`}
                        onClick={() => {
                          setSelectedFormat(format);
                          setCopyState("idle");
                        }}
                      >
                        {format}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1fr)_220px]">
                  <div className="min-w-0 rounded-[1.15rem] border border-[var(--portal-line)] bg-slate-950 p-4 shadow-inner shadow-black/20">
                    <pre className="overflow-x-auto whitespace-pre-wrap break-all font-mono text-sm leading-6 text-emerald-100">
                      <code>{renderedConfig}</code>
                    </pre>
                  </div>

                  <div className="grid gap-3 self-start">
                    <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                      <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">Gateway base URL</p>
                      <p className="mt-2 break-all text-sm font-semibold text-[var(--portal-ink)]">{gatewayBaseUrl}</p>
                    </div>

                    <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
                      <p className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">Copy</p>
                      <button type="button" className="btn-primary mt-3 w-full" onClick={() => void handleCopyConfig()} disabled={!userKey.trim()}>
                        Copy rendered config
                      </button>
                      <p className="mt-3 text-xs leading-5 text-[var(--portal-muted)]">
                        {copyState === "copied"
                          ? "Copied the currently rendered config with your real key included."
                          : copyState === "error"
                            ? "Copy failed in this browser context. Select the config block manually instead."
                            : "Copy uses the active template and active format exactly as shown above."}
                      </p>
                    </div>

                    <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4 text-sm text-[var(--portal-muted)]">
                      {userKey.trim()
                        ? "Template content is live and interpolated from your current user key. Changing the key updates all template views immediately."
                        : "Add a user key first so the template output contains real credentials instead of an empty value."}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>
      ) : null}
    </section>
  );
}
