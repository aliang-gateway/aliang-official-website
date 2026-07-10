"use client";

import { useCallback, useEffect, useState } from "react";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { extractApiError, unwrapData } from "@/lib/api-response";

type ServiceDirection = {
  id: number;
  status: "research" | "done";
  phase_zh: string;
  phase_en: string;
  title_zh: string;
  title_en: string;
  desc_zh: string;
  desc_en: string;
  sort_order: number;
  is_published: boolean;
  created_at: string;
  updated_at: string;
};

type ServicesResponse = { services?: ServiceDirection[]; error?: string };

const SESSION_TOKEN_STORAGE_KEY = "session_token";

const statusOptions: { value: "research" | "done"; label: string }[] = [
  { value: "research", label: "研究中 (Research)" },
  { value: "done", label: "已完成 (Shipped)" },
];

export default function AdminServicesPage() {
  const [items, setItems] = useState<ServiceDirection[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [showDialog, setShowDialog] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const [status, setStatus] = useState<"research" | "done">("research");
  const [phaseZh, setPhaseZh] = useState("");
  const [phaseEn, setPhaseEn] = useState("");
  const [titleZh, setTitleZh] = useState("");
  const [titleEn, setTitleEn] = useState("");
  const [descZh, setDescZh] = useState("");
  const [descEn, setDescEn] = useState("");
  const [sortOrder, setSortOrder] = useState(1);
  const [isPublished, setIsPublished] = useState(true);

  const [globalSuccess, setGlobalSuccess] = useState("");

  const resetForm = useCallback(() => {
    setStatus("research");
    setPhaseZh("");
    setPhaseEn("");
    setTitleZh("");
    setTitleEn("");
    setDescZh("");
    setDescEn("");
    setSortOrder(1);
    setIsPublished(true);
    setEditingId(null);
    setFormError(null);
  }, []);

  const loadItems = useCallback(async () => {
    setIsLoading(true);
    setLoadError(null);
    const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    try {
      const res = await fetch("/api/admin/services", {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + token,
        },
        cache: "no-store",
      });
      const payload = (await res.json()) as unknown;
      if (!res.ok) throw new Error(extractApiError(payload, "Failed to load services"));
      const data = unwrapData<ServicesResponse>(payload) ?? (payload as ServicesResponse);
      setItems(data.services ?? []);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Failed to load services");
      setItems([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const handleEdit = async (id: number) => {
    setIsDetailLoading(true);
    setFormError(null);
    const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    try {
      const res = await fetch(`/api/admin/services/${id}`, {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + token,
        },
        cache: "no-store",
      });
      const payload = (await res.json()) as unknown;
      if (!res.ok) throw new Error(extractApiError(payload, "Failed to load service"));
      const it = (unwrapData<ServiceDirection>(payload) ?? payload) as ServiceDirection;
      setEditingId(it.id);
      setStatus(it.status);
      setPhaseZh(it.phase_zh);
      setPhaseEn(it.phase_en);
      setTitleZh(it.title_zh);
      setTitleEn(it.title_en);
      setDescZh(it.desc_zh);
      setDescEn(it.desc_en);
      setSortOrder(it.sort_order);
      setIsPublished(it.is_published);
      setShowDialog(true);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to load service");
    } finally {
      setIsDetailLoading(false);
    }
  };

  const handleCreateOrUpdate = async () => {
    setFormError(null);
    if (!titleZh.trim() || !titleEn.trim()) {
      setFormError("中英文标题均为必填");
      return;
    }
    if (!phaseZh.trim() || !phaseEn.trim()) {
      setFormError("中英文阶段标签均为必填");
      return;
    }
    if (!descZh.trim() || !descEn.trim()) {
      setFormError("中英文描述均为必填");
      return;
    }
    const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    const body = JSON.stringify({
      status,
      phase_zh: phaseZh.trim(),
      phase_en: phaseEn.trim(),
      title_zh: titleZh.trim(),
      title_en: titleEn.trim(),
      desc_zh: descZh.trim(),
      desc_en: descEn.trim(),
      sort_order: sortOrder,
      is_published: isPublished,
    });
    try {
      const url = editingId ? `/api/admin/services/${editingId}` : "/api/admin/services";
      const method = editingId ? "PUT" : "POST";
      const res = await fetch(url, {
        method,
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + token,
        },
        body,
        cache: "no-store",
      });
      const payload = (await res.json()) as unknown;
      if (!res.ok) throw new Error(extractApiError(payload, "Failed to save service"));
      setGlobalSuccess(editingId ? "服务项已更新。" : "服务项已创建。");
      resetForm();
      setShowDialog(false);
      await loadItems();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to save service");
    }
  };

  const handleDelete = async (id: number) => {
    const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    try {
      const res = await fetch(`/api/admin/services/${id}`, {
        method: "DELETE",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + token,
        },
        cache: "no-store",
      });
      if (!res.ok) {
        const payload = (await res.json()) as unknown;
        throw new Error(extractApiError(payload, "Failed to delete service"));
      }
      setGlobalSuccess("服务项已删除。");
      await loadItems();
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Failed to delete service");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-[var(--portal-ink)]">服务时间线</h2>
          <p className="text-sm text-[var(--portal-muted)]">管理 /services 页的研究项与已完成项（中英双语），数字越小越靠前。</p>
        </div>
        <button
          type="button"
          onClick={() => {
            resetForm();
            setShowDialog(true);
          }}
          className="flex h-10 items-center gap-2 rounded-lg bg-[var(--portal-accent)] px-4 font-bold text-sm text-white transition-colors hover:opacity-90"
        >
          <MaterialIcon name="add" size={18} />
          新建服务项
        </button>
      </div>

      {globalSuccess && (
        <div className="rounded-xl border border-green-400/40 bg-green-500/10 p-4 text-sm text-green-700">
          {globalSuccess}
        </div>
      )}
      {loadError && (
        <div className="rounded-xl border border-red-400/40 bg-red-500/10 p-4 text-sm text-red-700" role="alert">
          {loadError}
        </div>
      )}

      {isLoading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((s) => (
            <div
              key={s}
              className="h-14 rounded-lg border animate-pulse"
              style={{ backgroundColor: "var(--portal-clay)", borderColor: "var(--portal-line)" }}
            />
          ))}
        </div>
      ) : items.length === 0 ? (
        <div
          className="py-16 text-center rounded-xl border"
          style={{ borderColor: "var(--portal-line)", backgroundColor: "var(--portal-clay)" }}
        >
          <MaterialIcon name="timeline" size={48} className="text-[var(--portal-muted)]" />
          <p className="mt-2 text-sm text-[var(--portal-muted)]">暂无服务项，点击&quot;新建服务项&quot;添加。</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border" style={{ borderColor: "var(--portal-line)" }}>
          <table className="w-full text-sm">
            <thead>
              <tr
                className="border-b text-left text-xs font-semibold uppercase"
                style={{
                  borderColor: "var(--portal-line)",
                  backgroundColor: "var(--portal-clay)",
                  color: "var(--portal-muted)",
                }}
              >
                <th className="px-4 py-3">排序</th>
                <th className="px-4 py-3">状态</th>
                <th className="px-4 py-3">标题 (中)</th>
                <th className="px-4 py-3">标题 (英)</th>
                <th className="px-4 py-3 text-center">已发布</th>
                <th className="px-4 py-3 text-right">操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((it) => (
                <tr
                  key={it.id}
                  className="border-b transition-colors hover:bg-[var(--portal-clay)]"
                  style={{ borderColor: "var(--portal-line)" }}
                >
                  <td className="px-4 py-3 font-mono text-xs text-[var(--portal-ink)]">{it.sort_order}</td>
                  <td className="px-4 py-3">
                    <span
                      className={
                        "inline-flex items-center rounded px-2 py-0.5 text-xs font-medium " +
                        (it.status === "research"
                          ? "bg-amber-500/10 text-amber-700"
                          : "bg-green-500/10 text-green-700")
                      }
                    >
                      {it.status === "research" ? "研究中" : "已完成"}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-medium text-[var(--portal-ink)]">{it.title_zh}</td>
                  <td className="px-4 py-3 text-[var(--portal-muted)]">{it.title_en}</td>
                  <td className="px-4 py-3 text-center">
                    {it.is_published ? (
                      <MaterialIcon name="check_circle" size={18} className="text-green-600" />
                    ) : (
                      <span className="text-[var(--portal-muted)]">&mdash;</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        onClick={() => {
                          void handleEdit(it.id);
                        }}
                        className="rounded-lg p-2 text-[var(--portal-muted)] transition-colors hover:bg-[var(--portal-clay)] hover:text-[var(--portal-accent)]"
                        title="编辑"
                      >
                        <MaterialIcon name="edit" size={16} />
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          void handleDelete(it.id);
                        }}
                        className="rounded-lg p-2 text-[var(--portal-muted)] transition-colors hover:bg-red-500/10 hover:text-red-600"
                        title="删除"
                      >
                        <MaterialIcon name="delete" size={16} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {showDialog ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="relative max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] p-6 shadow-2xl">
            <div className="mb-6 flex items-center justify-between">
              <h3 className="text-lg font-bold text-[var(--portal-ink)]">
                {editingId ? "编辑服务项" : "新建服务项"}
              </h3>
              <button
                type="button"
                onClick={() => {
                  resetForm();
                  setShowDialog(false);
                }}
                className="rounded-lg p-2 text-[var(--portal-muted)] transition-colors hover:bg-[var(--portal-clay)]"
              >
                &times;
              </button>
            </div>

            {isDetailLoading ? (
              <div className="py-12 text-center text-[var(--portal-muted)]">加载中...</div>
            ) : (
              <>
                {formError && (
                  <div
                    className="mb-4 rounded-xl border border-red-400/40 bg-red-500/10 p-3 text-sm text-red-700"
                    role="alert"
                  >
                    {formError}
                  </div>
                )}

                <div className="space-y-4">
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">状态</label>
                      <select
                        value={status}
                        onChange={(e) => setStatus(e.target.value as "research" | "done")}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      >
                        {statusOptions.map((o) => (
                          <option key={o.value} value={o.value}>
                            {o.label}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">
                        排序 (越小越靠前)
                      </label>
                      <input
                        type="number"
                        value={sortOrder}
                        onChange={(e) => setSortOrder(Number(e.target.value))}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">已发布</label>
                      <div className="flex items-center gap-3 pt-2">
                        <button
                          type="button"
                          role="switch"
                          aria-checked={isPublished}
                          onClick={() => setIsPublished(!isPublished)}
                          className={
                            "relative inline-flex h-6 w-11 items-center rounded-full transition-colors " +
                            (isPublished ? "bg-[var(--portal-accent)]" : "bg-[var(--portal-line)]")
                          }
                        >
                          <span
                            className={
                              "inline-block size-4 rounded-full bg-white transition-transform " +
                              (isPublished ? "translate-x-6" : "translate-x-1")
                            }
                          />
                        </button>
                        <span className="text-sm text-[var(--portal-muted)]">{isPublished ? "是" : "否"}</span>
                      </div>
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">
                        阶段标签 (中) · 如 &quot;最新 01 / 研究中&quot;
                      </label>
                      <input
                        type="text"
                        value={phaseZh}
                        onChange={(e) => setPhaseZh(e.target.value)}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">
                        阶段标签 (英) · e.g. &quot;Latest 01 / In research&quot;
                      </label>
                      <input
                        type="text"
                        value={phaseEn}
                        onChange={(e) => setPhaseEn(e.target.value)}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">标题 (中)</label>
                      <input
                        type="text"
                        value={titleZh}
                        onChange={(e) => setTitleZh(e.target.value)}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">标题 (英)</label>
                      <input
                        type="text"
                        value={titleEn}
                        onChange={(e) => setTitleEn(e.target.value)}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                  </div>

                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">描述 (中)</label>
                      <textarea
                        value={descZh}
                        onChange={(e) => setDescZh(e.target.value)}
                        rows={4}
                        className="w-full resize-y rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">描述 (英)</label>
                      <textarea
                        value={descEn}
                        onChange={(e) => setDescEn(e.target.value)}
                        rows={4}
                        className="w-full resize-y rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                  </div>
                </div>

                <div className="mt-6 flex items-center justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => {
                      resetForm();
                      setShowDialog(false);
                    }}
                    className="rounded-lg border border-[var(--portal-line)] px-6 py-2.5 text-sm font-semibold text-[var(--portal-muted)] transition-colors hover:bg-[var(--portal-clay)]"
                  >
                    取消
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      void handleCreateOrUpdate();
                    }}
                    className="rounded-lg bg-[var(--portal-accent)] px-6 py-2.5 text-sm font-bold text-white transition-colors hover:opacity-90"
                  >
                    {editingId ? "保存" : "创建"}
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}
