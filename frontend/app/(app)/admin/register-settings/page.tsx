"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";

import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asRecord } from "@/lib/api-response";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type RegisterSettings = {
  email_verification_enabled: boolean;
  smtp_configured: boolean;
};

// Admin 开关页：注册邮箱验证（portal 自有，免重启生效）。开启前校验 SMTP
// 已配置（后端同样拒绝，双保险）。
export default function AdminRegisterSettingsPage() {
  const t = useTranslations("adminRegisterSettings");
  const router = useRouter();
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [settings, setSettings] = useState<RegisterSettings | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const sessionToken = () => localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";

  const load = useCallback(
    async (token: string) => {
      setError(null);
      try {
        const response = await fetch("/api/admin/register-settings", {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${token}`,
          },
          cache: "no-store",
        });
        if (response.status === 401 || response.status === 403) {
          router.replace("/admin");
          return;
        }
        const payload = (await response.json()) as unknown;
        const root = asRecord(payload);
        setSettings({
          email_verification_enabled: Boolean(root?.email_verification_enabled),
          smtp_configured: Boolean(root?.smtp_configured),
        });
      } catch (loadError) {
        setError(loadError instanceof Error ? loadError.message : "failed");
      }
    },
    [router],
  );

  useEffect(() => {
    const token = sessionToken();
    if (!token) {
      router.replace("/login");
      return;
    }
    void load(token).finally(() => setIsCheckingAuth(false));
  }, [load, router]);

  const toggle = async () => {
    if (!settings) return;
    const token = sessionToken();
    setBusy(true);
    setError(null);
    try {
      const response = await fetch("/api/admin/register-settings", {
        method: "PUT",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({ email_verification_enabled: !settings.email_verification_enabled }),
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const root = asRecord(payload);
        throw new Error(root?.error ? String(root.error) : "failed");
      }
      const root = asRecord(payload);
      setSettings({
        email_verification_enabled: Boolean(root?.email_verification_enabled),
        smtp_configured: Boolean(root?.smtp_configured),
      });
    } catch (toggleError) {
      setError(toggleError instanceof Error ? toggleError.message : "failed");
    } finally {
      setBusy(false);
    }
  };

  if (isCheckingAuth) {
    return null;
  }

  const enabled = settings?.email_verification_enabled ?? false;
  const smtpOk = settings?.smtp_configured ?? false;

  return (
    <section className="portal-shell space-y-6 py-10">
      <h1 className="text-2xl font-extrabold text-[var(--ink)]">{t("title")}</h1>

      {error ? <p className="notice">{error}</p> : null}

      <div className="clay-panel space-y-4 p-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="min-w-0">
            <p className="flex items-center gap-2 text-sm font-bold text-[var(--ink)]">
              <MaterialIcon name="mark_email_read" size={18} />
              {t("emailVerificationToggle")}
            </p>
            <p className="mt-1 text-xs leading-relaxed text-[var(--ink-muted)]">{t("emailVerificationDesc")}</p>
          </div>
          <button
            type="button"
            onClick={toggle}
            disabled={busy || !smtpOk}
            className={`inline-flex min-w-[120px] items-center justify-center gap-2 rounded-full px-5 py-2 text-sm font-bold transition-opacity disabled:cursor-not-allowed disabled:opacity-40 ${
              enabled
                ? "bg-emerald-500 text-white hover:bg-emerald-600"
                : "border border-[var(--line)] text-[var(--ink)] hover:border-[var(--accent)]/40"
            }`}
          >
            <MaterialIcon name={busy ? "hourglass_top" : enabled ? "toggle_on" : "toggle_off"} size={18} />
            {enabled ? t("stateOn") : t("stateOff")}
          </button>
        </div>

        {!smtpOk ? (
          <p className="rounded-[0.75rem] border border-amber-500/40 bg-amber-500/10 px-4 py-3 text-xs leading-relaxed text-[var(--ink)]">
            {t("smtpMissingHint")}
          </p>
        ) : null}

        <p className="text-xs text-[var(--ink-muted)]">
          {t("runtimeHint")}
        </p>
      </div>
    </section>
  );
}
