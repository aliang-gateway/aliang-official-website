"use client";

import { useCallback, useEffect, useState } from "react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";

import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asRecord } from "@/lib/api-response";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

// 后端存的是 RFC3339 UTC；就地转本地时区展示，解析失败原样回退。
function formatUpdatedAt(raw: string): string {
  if (!raw) {
    return "-";
  }
  const parsed = new Date(raw);
  return Number.isNaN(parsed.getTime()) ? raw : parsed.toLocaleString();
}

type NoticeForm = {
  title: string;
  body: string;
  enabled: boolean;
};

// Admin 公告编辑页：单条当前公告（portal 自有，免重启生效）。改文案会换公告
// id → 所有用户（含已确认过的）下次进控制台重新弹；仅切 enabled 不重弹。
export default function AdminDashboardNoticePage() {
  const t = useTranslations("adminDashboardNotice");
  const router = useRouter();
  const [isCheckingAuth, setIsCheckingAuth] = useState(true);
  const [form, setForm] = useState<NoticeForm>({ title: "", body: "", enabled: false });
  const [hasStoredNotice, setHasStoredNotice] = useState(false);
  const [updatedAt, setUpdatedAt] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState(false);

  const sessionToken = () => localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";

  const load = useCallback(
    async (token: string) => {
      setError(null);
      try {
        const response = await fetch("/api/admin/dashboard-notice", {
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
        const notice = asRecord(root?.notice);
        setHasStoredNotice(notice !== null);
        setForm({
          title: notice ? String(notice.title ?? "") : "",
          body: notice ? String(notice.body ?? "") : "",
          enabled: notice ? Boolean(notice.enabled) : false,
        });
        setUpdatedAt(notice ? String(notice.updated_at ?? "") : "");
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

  const save = async () => {
    const token = sessionToken();
    setBusy(true);
    setError(null);
    setSaved(false);
    try {
      const response = await fetch("/api/admin/dashboard-notice", {
        method: "PUT",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          title: form.title.trim(),
          body: form.body.trim(),
          enabled: form.enabled,
        }),
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const root = asRecord(payload);
        throw new Error(root?.error ? String(root.error) : "failed");
      }
      const root = asRecord(payload);
      const notice = asRecord(root?.notice);
      if (notice) {
        setForm({
          title: String(notice.title ?? ""),
          body: String(notice.body ?? ""),
          enabled: Boolean(notice.enabled),
        });
        setUpdatedAt(String(notice.updated_at ?? ""));
      }
      setHasStoredNotice(true);
      setSaved(true);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "failed");
    } finally {
      setBusy(false);
    }
  };

  if (isCheckingAuth) {
    return null;
  }

  const canEnable = form.title.trim() !== "" && form.body.trim() !== "";

  return (
    <section className="portal-shell space-y-6 py-10">
      <h1 className="text-2xl font-extrabold text-[var(--ink)]">{t("title")}</h1>

      {error ? <p className="notice">{error}</p> : null}
      {saved ? (
        <div
          className="rounded-xl border border-emerald-400/40 dark:border-emerald-400/60 bg-emerald-500/10 dark:bg-emerald-500/20 p-3 text-sm text-emerald-700 dark:text-emerald-300"
          role="status"
        >
          {t("saved")}
        </div>
      ) : null}

      <div className="clay-panel space-y-5 p-6">
        <p className="text-xs leading-relaxed text-[var(--ink-muted)]">{t("intro")}</p>

        <div className="field">
          <label htmlFor="dashboard-notice-title">{t("titleLabel")}</label>
          <input
            id="dashboard-notice-title"
            className="field-input"
            placeholder={t("titlePlaceholder")}
            type="text"
            value={form.title}
            onChange={(event) => {
              setSaved(false);
              setForm((current) => ({ ...current, title: event.target.value }));
            }}
          />
        </div>

        <div className="field">
          <label htmlFor="dashboard-notice-body">{t("bodyLabel")}</label>
          <textarea
            id="dashboard-notice-body"
            className="field-input min-h-[8rem]"
            placeholder={t("bodyPlaceholder")}
            value={form.body}
            onChange={(event) => {
              setSaved(false);
              setForm((current) => ({ ...current, body: event.target.value }));
            }}
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-4">
          <button
            type="button"
            onClick={() => {
              setSaved(false);
              setForm((current) => ({ ...current, enabled: !current.enabled }));
            }}
            disabled={!canEnable && !form.enabled}
            className={`inline-flex min-w-[120px] items-center justify-center gap-2 rounded-full px-5 py-2 text-sm font-bold transition-opacity disabled:cursor-not-allowed disabled:opacity-40 ${
              form.enabled
                ? "bg-emerald-500 text-white hover:bg-emerald-600"
                : "border border-[var(--line)] text-[var(--ink)] hover:border-[var(--accent)]/40"
            }`}
          >
            <MaterialIcon name={form.enabled ? "toggle_on" : "toggle_off"} size={18} />
            {form.enabled ? t("stateOn") : t("stateOff")}
          </button>

          <button
            type="button"
            className="btn-primary inline-flex items-center gap-2"
            onClick={save}
            disabled={busy || (!canEnable && form.enabled)}
          >
            <MaterialIcon name={busy ? "hourglass_top" : "save"} size={18} />
            {busy ? t("saving") : t("save")}
          </button>
        </div>

        {!canEnable ? <p className="text-xs text-amber-600">{t("emptyHint")}</p> : null}

        {hasStoredNotice ? (
          <p className="text-xs text-[var(--ink-muted)]">{t("lastUpdated", { time: formatUpdatedAt(updatedAt) })}</p>
        ) : (
          <p className="text-xs text-[var(--ink-muted)]">{t("noneYet")}</p>
        )}
      </div>
    </section>
  );
}
