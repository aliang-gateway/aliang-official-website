"use client";

import { QRCodeSVG } from "qrcode.react";
import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { asRecord, asString, extractApiError } from "@/lib/api-response";
import { useTranslations } from "next-intl";

export function ScanLoginPanel({ nextPath }: { nextPath: string }) {
  const router = useRouter();
  const t = useTranslations("login");
  const [qrPayload, setQrPayload] = useState<string | null>(null);
  const [message, setMessage] = useState<string>(t("scanWaiting"));

  const deviceCodeRef = useRef<string>("");
  const intervalRef = useRef<number>(2);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const stoppedRef = useRef(false);

  function clearTimer() {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }
  function schedule(fn: () => void, ms: number) {
    clearTimer();
    timerRef.current = setTimeout(fn, ms);
  }

  async function startSession() {
    setMessage(t("scanWaiting"));
    setQrPayload(null);
    try {
      const res = await fetch("/api/auth/scan/init", {
        method: "POST",
        headers: { accept: "application/json" },
      });
      const payload = (await res.json()) as unknown;
      if (!res.ok) {
        setMessage(extractApiError(payload, t("scanWaiting")));
        if (!stoppedRef.current) schedule(startSession, 2000);
        return;
      }
      const rec = asRecord(payload);
      const dc = asString(rec?.device_code);
      const qp = asString(rec?.qr_payload) || asString(rec?.scan_code);
      const iv = asString(rec?.interval);
      if (!dc || !qp) {
        setMessage(t("scanInitFailed"));
        if (!stoppedRef.current) schedule(startSession, 2000);
        return;
      }
      deviceCodeRef.current = dc;
      if (iv) intervalRef.current = Math.max(1, Number(iv));
      setQrPayload(qp);
      if (!stoppedRef.current) schedule(poll, intervalRef.current * 1000);
    } catch {
      if (!stoppedRef.current) schedule(startSession, 2000);
    }
  }

  async function poll() {
    const dc = deviceCodeRef.current;
    if (!dc) return;
    try {
      const res = await fetch(`/api/auth/scan/status?device_code=${encodeURIComponent(dc)}`, {
        headers: { accept: "application/json" },
      });
      if (!res.ok) {
        if (!stoppedRef.current) schedule(poll, intervalRef.current * 1000);
        return;
      }
      const rec = asRecord(await res.json());
      const status = asString(rec?.status);
      const token = asString(rec?.session_token);
      const role = asString(asRecord(rec?.user)?.role);

      if (status === "authorized" && token) {
        if (stoppedRef.current) return;
        localStorage.setItem("session_token", token);
        setMessage(t("scanSuccess"));
        const safe = nextPath.startsWith("/") && !nextPath.startsWith("//") ? nextPath : "";
        router.replace(
          safe ||
            (role === "distributor"
              ? "/distributor"
              : role === "admin"
                ? "/admin/users"
                : "/dashboard"),
        );
        return;
      }
      if (status === "scanned") {
        setMessage(t("scanScanned"));
      } else if (status === "denied") {
        setMessage(t("scanDenied"));
        schedule(startSession, 2000);
        return;
      } else if (status === "expired") {
        setMessage(t("scanExpired"));
        schedule(startSession, 500);
        return;
      } else {
        setMessage(t("scanWaiting"));
      }
      if (!stoppedRef.current) schedule(poll, intervalRef.current * 1000);
    } catch {
      if (!stoppedRef.current) schedule(poll, intervalRef.current * 1000);
    }
  }

  useEffect(() => {
    stoppedRef.current = false;
    void startSession();
    return () => {
      stoppedRef.current = true;
      clearTimer();
    };
    // 仅在挂载时启动扫码生命周期；清理通过 stoppedRef + clearTimer 完成。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="flex flex-col items-center gap-4">
      <div className="rounded-lg border border-[var(--stitch-border)] bg-white p-3">
        {qrPayload ? (
          <QRCodeSVG value={qrPayload} size={200} />
        ) : (
          <div className="flex h-[200px] w-[200px] items-center justify-center text-sm text-[var(--stitch-text-muted)]">
            …
          </div>
        )}
      </div>
      <p className="text-sm text-[var(--stitch-text-muted)]">{message}</p>
    </div>
  );
}
