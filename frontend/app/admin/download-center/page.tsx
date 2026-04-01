"use client";

import { useCallback, useEffect, useState } from "react";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { extractApiError, unwrapData } from "@/lib/api-response";

type Download = {
  id: number;
  software_name: string;
  platform: string;
  file_type: string;
  download_url: string;
  version: string;
  force_update: boolean;
  changelog: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
};

type DownloadsResponse = {
  downloads?: Download[];
  error?: string;
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

const platformOptions = ["linux", "darwin", "windows"];
const fileTypeOptions = ["dmg", "zip", "exe", "deb", "pkg", "bin", "AppImage", "msi"];

const versionRegex = /^v\d\.\d\.\d$/;

export default function DownloadCenterPage() {
  const [downloads, setDownloads] = useState<Download[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const [showDialog, setShowDialog] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [isDetailLoading, setIsDetailLoading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  const [softwareName, setSoftwareName] = useState("");
  const [platform, setPlatform] = useState("linux");
  const [fileType, setFileType] = useState("deb");
  const [downloadUrl, setDownloadUrl] = useState("");
  const [version, setVersion] = useState("v1.0.0");
  const [forceUpdate, setForceUpdate] = useState(false);
  const [isDefault, setIsDefault] = useState(false);
  const [changelog, setChangelog] = useState("");

  const [globalSuccess, setGlobalSuccess] = useState("");

  const resetForm = useCallback(() => {
    setSoftwareName("");
    setPlatform("linux");
    setFileType("deb");
    setDownloadUrl("");
    setVersion("v1.0.0");
    setForceUpdate(false);
    setIsDefault(false);
    setChangelog("");
    setEditingId(null);
    setFormError(null);
  }, []);

  const loadDownloads = useCallback(async () => {
    setIsLoading(true);
    setLoadError(null);

    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    try {
      const response = await fetch("/api/admin/download-center", {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + sessionToken,
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to load downloads"));
      }

      const data = unwrapData<DownloadsResponse>(payload) ?? (payload as DownloadsResponse);
      setDownloads(data.downloads ?? []);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Failed to load downloads");
      setDownloads([]);
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadDownloads();
  }, [loadDownloads]);

  const handleEdit = async (id: number) => {
    setIsDetailLoading(true);
    setFormError(null);

    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    try {
      const response = await fetch("/api/admin/download-center/" + id, {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + sessionToken,
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to load download"));
      }

      const d = (unwrapData<Download>(payload) ?? payload) as Download;
      setEditingId(d.id);
      setSoftwareName(d.software_name);
      setPlatform(d.platform);
      setFileType(d.file_type);
      setDownloadUrl(d.download_url);
      setVersion(d.version);
      setForceUpdate(d.force_update);
      setIsDefault(d.is_default);
      setChangelog(d.changelog ?? "");
      setShowDialog(true);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to load download");
    } finally {
      setIsDetailLoading(false);
    }
  };

  const handleCreateOrUpdate = async () => {
    setFormError(null);

    const trimmedVersion = version.trim();
    if (!versionRegex.test(trimmedVersion)) {
      setFormError("Version must be in vX.X.X format with single digits (e.g. v1.0.0, v2.3.9)");
      return;
    }
    if (!softwareName.trim() || !downloadUrl.trim()) {
      setFormError("Software name and download URL are required");
      return;
    }

    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    const body = JSON.stringify({
      software_name: softwareName.trim(),
      platform,
      file_type: fileType,
      download_url: downloadUrl.trim(),
      version: trimmedVersion,
      force_update: forceUpdate,
      is_default: isDefault,
      changelog: changelog.trim(),
    });

    try {
      const url = editingId ? "/api/admin/download-center/" + editingId : "/api/admin/download-center";
      const method = editingId ? "PUT" : "POST";

      const response = await fetch(url, {
        method,
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + sessionToken,
        },
        body,
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to save download"));
      }

      setGlobalSuccess(editingId ? "Download updated." : "Download created.");
      resetForm();
      setShowDialog(false);
      await loadDownloads();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to save download");
    }
  };

  const handleDelete = async (id: number) => {
    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    try {
      const response = await fetch("/api/admin/download-center/" + id, {
        method: "DELETE",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: "Bearer " + sessionToken,
        },
        cache: "no-store",
      });

      if (!response.ok) {
        const payload = (await response.json()) as unknown;
        throw new Error(extractApiError(payload, "Failed to delete download"));
      }

      setGlobalSuccess("Download deleted.");
      await loadDownloads();
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Failed to delete download");
    }
  };

  const platformLabel = (p: string) => {
    if (p === "darwin") return "macOS";
    if (p === "linux") return "Linux";
    if (p === "windows") return "Windows";
    return p;
  };

  return (
    <div className="space-y-6">
      {/* List Header */}
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-[var(--portal-ink)]">Downloads</h2>
          <p className="text-sm text-[var(--portal-muted)]">Manage software downloads &amp; version control</p>
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
          New Download
        </button>
      </div>

      {/* Success / Error */}
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

      {/* Loading */}
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
      ) : downloads.length === 0 ? (
        <div className="py-16 text-center rounded-xl border" style={{ borderColor: "var(--portal-line)", backgroundColor: "var(--portal-clay)" }}>
          <MaterialIcon name="cloud_download" size={48} className="text-[var(--portal-muted)]" />
          <p className="mt-2 text-sm text-[var(--portal-muted)]">No downloads yet. Click &quot;New Download&quot; to add one.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-xl border" style={{ borderColor: "var(--portal-line)" }}>
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b text-left text-xs font-semibold uppercase" style={{ borderColor: "var(--portal-line)", backgroundColor: "var(--portal-clay)", color: "var(--portal-muted)" }}>
                <th className="px-4 py-3">Software</th>
                <th className="px-4 py-3">Platform</th>
                <th className="px-4 py-3">Type</th>
                <th className="px-4 py-3">Version</th>
                <th className="px-4 py-3 text-center">Default</th>
                <th className="px-4 py-3 text-center">Force</th>
                <th className="px-4 py-3 text-right">Actions</th>
              </tr>
            </thead>
            <tbody>
              {downloads.map((dl) => (
                <tr key={dl.id} className="border-b transition-colors hover:bg-[var(--portal-clay)]" style={{ borderColor: "var(--portal-line)" }}>
                  <td className="px-4 py-3 font-medium text-[var(--portal-ink)]">{dl.software_name}</td>
                  <td className="px-4 py-3 text-[var(--portal-ink)]">{platformLabel(dl.platform)}</td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center rounded px-2 py-0.5 text-xs font-medium" style={{ backgroundColor: "var(--portal-accent)", color: "white" }}>
                      {dl.file_type}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-[var(--portal-ink)]">{dl.version}</td>
                  <td className="px-4 py-3 text-center">
                    {dl.is_default ? (
                      <MaterialIcon name="star" size={18} className="text-amber-500" />
                    ) : (
                      <span className="text-[var(--portal-muted)]">&mdash;</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-center">
                    {dl.force_update ? (
                      <span className="inline-flex items-center rounded bg-red-500/10 px-2 py-0.5 text-xs font-medium text-red-700">Force</span>
                    ) : (
                      <span className="text-[var(--portal-muted)]">&mdash;</span>
                    )}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div className="flex items-center justify-end gap-1">
                      <button
                        type="button"
                        onClick={() => { void handleEdit(dl.id); }}
                        className="rounded-lg p-2 text-[var(--portal-muted)] transition-colors hover:bg-[var(--portal-clay)] hover:text-[var(--portal-accent)]"
                        title="Edit"
                      >
                        <MaterialIcon name="edit" size={16} />
                      </button>
                      <button
                        type="button"
                        onClick={() => { void handleDelete(dl.id); }}
                        className="rounded-lg p-2 text-[var(--portal-muted)] transition-colors hover:bg-red-500/10 hover:text-red-600"
                        title="Delete"
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

      {/* Dialog */}
      {showDialog ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="relative max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] p-6 shadow-2xl">
            {/* Header */}
            <div className="flex items-center justify-between mb-6">
              <h3 className="text-lg font-bold text-[var(--portal-ink)]">
                {editingId ? "Edit Download" : "New Download"}
              </h3>
              <button
                type="button"
                onClick={() => { resetForm(); setShowDialog(false); }}
                className="rounded-lg p-2 text-[var(--portal-muted)] transition-colors hover:bg-[var(--portal-clay)]"
              >
                &times;
              </button>
            </div>

            {/* Detail loading */}
            {isDetailLoading ? (
              <div className="py-12 text-center text-[var(--portal-muted)]">Loading...</div>
            ) : (
              <>
                {formError && (
                  <div className="mb-4 rounded-xl border border-red-400/40 bg-red-500/10 p-3 text-sm text-red-700" role="alert">
                    {formError}
                  </div>
                )}

                <div className="space-y-4">
                  {/* Software Name */}
                  <div>
                    <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">Software Name</label>
                    <input
                      type="text"
                      value={softwareName}
                      onChange={(e) => setSoftwareName(e.target.value)}
                      placeholder="e.g. ALiang Tool"
                      className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                    />
                  </div>

                  {/* Platform + File Type */}
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">Platform</label>
                      <select
                        value={platform}
                        onChange={(e) => setPlatform(e.target.value)}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      >
                        {platformOptions.map((p) => (
                          <option key={p} value={p}>{platformLabel(p)}</option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">File Type</label>
                      <select
                        value={fileType}
                        onChange={(e) => setFileType(e.target.value)}
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      >
                        {fileTypeOptions.map((ft) => (
                          <option key={ft} value={ft}>{ft}</option>
                        ))}
                      </select>
                    </div>
                  </div>

                  {/* Version + Force Update + Default */}
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">Version (vX.X.X)</label>
                      <input
                        type="text"
                        value={version}
                        onChange={(e) => setVersion(e.target.value)}
                        placeholder="v1.0.0"
                        className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 font-mono text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                      />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">Force Update</label>
                      <div className="flex items-center gap-3 pt-2">
                        <button
                          type="button"
                          role="switch"
                          aria-checked={forceUpdate}
                          onClick={() => setForceUpdate(!forceUpdate)}
                          className={"relative inline-flex h-6 w-11 items-center rounded-full transition-colors " + (forceUpdate ? "bg-[var(--portal-accent)]" : "bg-[var(--portal-line)]")}
                        >
                          <span className={"inline-block size-4 rounded-full bg-white transition-transform " + (forceUpdate ? "translate-x-6" : "translate-x-1")} />
                        </button>
                        <span className="text-sm text-[var(--portal-muted)]">{forceUpdate ? "Yes" : "No"}</span>
                      </div>
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">Default</label>
                      <div className="flex items-center gap-3 pt-2">
                        <button
                          type="button"
                          role="switch"
                          aria-checked={isDefault}
                          onClick={() => setIsDefault(!isDefault)}
                          className={"relative inline-flex h-6 w-11 items-center rounded-full transition-colors " + (isDefault ? "bg-amber-500" : "bg-[var(--portal-line)]")}
                        >
                          <span className={"inline-block size-4 rounded-full bg-white transition-transform " + (isDefault ? "translate-x-6" : "translate-x-1")} />
                        </button>
                        <span className="text-sm text-[var(--portal-muted)]">{isDefault ? "Yes" : "No"}</span>
                      </div>
                    </div>
                  </div>

                  {/* Download URL */}
                  <div>
                    <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">Download URL</label>
                    <input
                      type="text"
                      value={downloadUrl}
                      onChange={(e) => setDownloadUrl(e.target.value)}
                      placeholder="https://example.com/downloads/software-v1.0.0.dmg"
                      className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)]"
                    />
                  </div>

                  {/* Changelog */}
                  <div>
                    <label className="mb-1 block text-xs font-semibold text-[var(--portal-muted)]">Changelog</label>
                    <textarea
                      value={changelog}
                      onChange={(e) => setChangelog(e.target.value)}
                      placeholder="What's new in this version..."
                      rows={3}
                      className="w-full rounded-lg border border-[var(--portal-line)] bg-[var(--portal-clay)] px-4 py-2.5 text-sm text-[var(--portal-ink)] outline-none focus:border-[var(--portal-accent)] resize-y"
                    />
                  </div>
                </div>

                {/* Actions */}
                <div className="mt-6 flex items-center justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => { resetForm(); setShowDialog(false); }}
                    className="rounded-lg border border-[var(--portal-line)] px-6 py-2.5 text-sm font-semibold text-[var(--portal-muted)] transition-colors hover:bg-[var(--portal-clay)]"
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    onClick={() => { void handleCreateOrUpdate(); }}
                    className="rounded-lg bg-[var(--portal-accent)] px-6 py-2.5 text-sm font-bold text-white transition-colors hover:opacity-90"
                  >
                    {editingId ? "Update" : "Create"}
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
