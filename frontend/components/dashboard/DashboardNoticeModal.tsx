"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslations } from "next-intl";

import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { Modal } from "@/components/ui/Modal";
import { asRecord } from "@/lib/api-response";

type ActiveNotice = {
  id: string;
  title: string;
  body: string;
};

// Dashboard 公告弹窗（portal 自有）：登录用户进入控制台时拉取当前公告，未确认
// 则弹窗。点「知道了」记录确认（按账号，服务端存储）；仅关闭不算确认——下次
// 进来照弹。拉取失败静默（公告是附加信息，不能挡住控制台）。
export function DashboardNoticeModal({ sessionToken }: { sessionToken: string }) {
  const t = useTranslations("dashboard");
  const [notice, setNotice] = useState<ActiveNotice | null>(null);
  const [open, setOpen] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [confirmError, setConfirmError] = useState(false);
  // 公告没有页面内的触发元素，给 Modal 一个空 ref 占位（关闭时聚焦回退为 no-op）。
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  // 稳定引用：Modal 的 effect 依赖 onClose，inline 箭头函数会让每次 render 重挂
  // 监听并把焦点跳回关闭按钮（评审确认）。
  const handleClose = useCallback(() => setOpen(false), []);

  useEffect(() => {
    if (!sessionToken) {
      return;
    }
    let cancelled = false;
    const controller = new AbortController();
    const load = async () => {
      try {
        const response = await fetch("/api/dashboard/notice", {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) {
          return;
        }
        const payload = (await response.json()) as unknown;
        const root = asRecord(payload);
        if (root?.acknowledged !== false) {
          return;
        }
        const rawNotice = asRecord(root?.notice);
        if (!rawNotice) {
          return;
        }
        const id = String(rawNotice.id ?? "");
        const title = String(rawNotice.title ?? "").trim();
        const body = String(rawNotice.body ?? "").trim();
        if (!id || !title || !body || cancelled) {
          return;
        }
        setNotice({ id, title, body });
        setOpen(true);
      } catch (fetchError) {
        if ((fetchError as Error).name === "AbortError") {
          return;
        }
        // 静默：拉不到公告就不弹。
      }
    };

    void load();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [sessionToken]);

  const confirm = async () => {
    if (!notice || confirming) {
      return;
    }
    setConfirming(true);
    setConfirmError(false);
    try {
      const response = await fetch("/api/dashboard/notice", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        cache: "no-store",
      });
      if (!response.ok) {
        // 确认失败不关弹窗，用户可重试；下下次进入仍会弹（符合未确认语义）。
        setConfirmError(true);
        return;
      }
      setOpen(false);
      setNotice(null);
    } catch {
      setConfirmError(true);
    } finally {
      setConfirming(false);
    }
  };

  if (!notice) {
    return null;
  }

  return (
    <Modal
      isOpen={open}
      onClose={handleClose}
      closeLabel={t("noticeDismiss")}
      triggerRef={triggerRef}
      panelClassName="max-w-lg"
    >
      <div className="space-y-4 p-6 pr-12">
        <div className="flex items-center gap-2 text-[var(--accent)]">
          <span className="inline-flex h-9 w-9 items-center justify-center rounded-full bg-[var(--accent-wash)]">
            <MaterialIcon name="campaign" size={20} />
          </span>
          <h2 className="text-lg font-extrabold leading-snug text-[var(--ink)]">{notice.title}</h2>
        </div>
        <p className="whitespace-pre-line break-words text-sm leading-relaxed text-[var(--ink-soft)]">{notice.body}</p>
        {confirmError ? (
          <p className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-2.5 text-xs text-amber-700 dark:text-amber-300" role="alert">
            {t("noticeConfirmFailed")}
          </p>
        ) : null}
        <div className="flex justify-end">
          <button
            type="button"
            className="btn-primary inline-flex items-center gap-2"
            onClick={confirm}
            disabled={confirming}
          >
            {confirming ? t("noticeConfirming") : t("noticeConfirm")}
          </button>
        </div>
      </div>
    </Modal>
  );
}
