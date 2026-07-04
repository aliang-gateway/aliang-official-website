"use client";

import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asRecord, extractApiError, unwrapData } from "@/lib/api-response";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

// ── Types ──────────────────────────────────────────────────────────

type AdminGroup = {
  id: number;
  name: string;
  code?: string;
  platform?: string;
  subscription_type?: string;
  type?: string;
};

type SoftwareConfig = {
  id: number;
  software_code: string;
  software_name: string;
  group_id: number;
  description: string;
  is_enabled: boolean;
  tags?: string[];
  created_at: string;
  updated_at: string;
};

type ConfigTemplate = {
  id: number;
  software_config_id: number;
  name: string;
  format: string;
  content: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
};

type GlobalVar = {
  id: number;
  var_key: string;
  var_value: string;
  description: string;
  created_at: string;
  updated_at: string;
};

type GroupsResponse = {
  groups?: AdminGroup[];
};

type ConfigsResponse = {
  configs?: SoftwareConfig[];
};

type TemplatesResponse = {
  templates?: ConfigTemplate[];
};

type GlobalVarsResponse = {
  vars?: GlobalVar[];
};

// ── Helpers ────────────────────────────────────────────────────────

function formatDateTime(value?: string | null) {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function normalizeAdminGroups(groups: AdminGroup[]) {
  const seenIDs = new Set<number>();
  const normalized: AdminGroup[] = [];
  for (const group of groups) {
    const id = Number(group.id) || 0;
    if (id <= 0 || seenIDs.has(id)) continue;
    seenIDs.add(id);
    normalized.push({
      ...group,
      id,
      name: String(group.name ?? "").trim() || `Group #${id}`,
    });
  }
  return normalized;
}

function parseGroupsPayload(payload: unknown) {
  return unwrapData<GroupsResponse>(payload) ?? ((asRecord(payload) as GroupsResponse | null) ?? {});
}

// ── Active Tab ─────────────────────────────────────────────────────

type TabID = "software" | "global-vars";

// ── Component ──────────────────────────────────────────────────────

export default function AdminConfigCenterPage() {
  const [sessionToken, setSessionToken] = useState("");
  const [isHydrated, setIsHydrated] = useState(false);

  // Shared
  const [globalError, setGlobalError] = useState<string | null>(null);
  const [globalSuccess, setGlobalSuccess] = useState<string | null>(null);
  const [authBlocked, setAuthBlocked] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<TabID>("software");

  // Software configs
  const [configs, setConfigs] = useState<SoftwareConfig[]>([]);
  const [availableGroups, setAvailableGroups] = useState<AdminGroup[]>([]);
  const [isLoadingConfigs, setIsLoadingConfigs] = useState(false);
  const [isLoadingGroups, setIsLoadingGroups] = useState(false);

  // Software form
  const [mode, setMode] = useState<"create" | "edit">("create");
  const [editingCode, setEditingCode] = useState<string | null>(null);
  const [swCode, setSwCode] = useState("");
  const [swName, setSwName] = useState("");
  const [swGroupId, setSwGroupId] = useState<number>(0);
  const [swDescription, setSwDescription] = useState("");
  const [swIsEnabled, setSwIsEnabled] = useState(true);
  const [formError, setFormError] = useState<string | null>(null);
  const [isSubmittingForm, setIsSubmittingForm] = useState(false);
  const [isLoadingDetail, setIsLoadingDetail] = useState(false);
  const [rowLoadingCode, setRowLoadingCode] = useState<string | null>(null);
  const [showSwDialog, setShowSwDialog] = useState(false);

  // Tags
  const [tags, setTags] = useState<string[]>([]);
  const [newTag, setNewTag] = useState("");
  const [tagLoading, setTagLoading] = useState(false);

  // Templates
  const [templates, setTemplates] = useState<ConfigTemplate[]>([]);
  const [isLoadingTemplates, setIsLoadingTemplates] = useState(false);
  const [showTemplateForm, setShowTemplateForm] = useState(false);
  const [tplEditMode, setTplEditMode] = useState<"create" | "edit">("create");
  const [tplEditId, setTplEditId] = useState<number | null>(null);
  const [tplName, setTplName] = useState("");
  const [tplFormat, setTplFormat] = useState("json");
  const [tplContent, setTplContent] = useState("");
  const [tplIsDefault, setTplIsDefault] = useState(false);
  const [tplSubmitting, setTplSubmitting] = useState(false);
  const [tplError, setTplError] = useState<string | null>(null);

  // Global vars
  const [globalVars, setGlobalVars] = useState<GlobalVar[]>([]);
  const [isLoadingGlobalVars, setIsLoadingGlobalVars] = useState(false);
  const [gvKey, setGvKey] = useState("");
  const [gvValue, setGvValue] = useState("");
  const [gvDescription, setGvDescription] = useState("");
  const [gvSubmitting, setGvSubmitting] = useState(false);
  const [gvError, setGvError] = useState<string | null>(null);

  // ── Init ───────────────────────────────────────────────────────

  useEffect(() => {
    setIsHydrated(true);
    setSessionToken(localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "");
  }, []);

  const buildHeaders = useCallback(() => {
    const headers: Record<string, string> = {
      "content-type": "application/json",
      accept: "application/json",
    };
    if (sessionToken) {
      headers.Authorization = `Bearer ${sessionToken}`;
    }
    return headers;
  }, [sessionToken]);

  const handleAuthFailure = useCallback((status: number, message?: string) => {
    if (status === 401 || status === 403) {
      setAuthBlocked(message ?? "Unauthorized. Admin permission is required.");
      return true;
    }
    return false;
  }, []);

  // ── Load configs ────────────────────────────────────────────────

  const loadConfigs = useCallback(async () => {
    if (!sessionToken) {
      setConfigs([]);
      setGlobalError("Missing session token. Please login from /account.");
      setAuthBlocked("Blocked: no session token found.");
      return;
    }

    setIsLoadingConfigs(true);
    setGlobalError(null);

    try {
      const response = await fetch("/api/admin/config-center/software", {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(payload, "Failed to load software configs");
        if (handleAuthFailure(response.status, message)) {
          setConfigs([]);
          setGlobalError(message);
          return;
        }
        throw new Error(message);
      }

      setAuthBlocked(null);
      const parsed = unwrapData<ConfigsResponse>(payload) ?? ((asRecord(payload) as ConfigsResponse | null) ?? {});
      setConfigs(Array.isArray(parsed.configs) ? parsed.configs : []);
    } catch (error) {
      setGlobalError(error instanceof Error ? error.message : "Failed to load software configs");
    } finally {
      setIsLoadingConfigs(false);
    }
  }, [buildHeaders, handleAuthFailure, sessionToken]);

  const loadGroups = useCallback(async () => {
    if (!sessionToken) {
      setAvailableGroups([]);
      return;
    }

    setIsLoadingGroups(true);

    try {
      const response = await fetch("/api/admin/groups/available", {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(payload, "Failed to load available groups");
        if (handleAuthFailure(response.status, message)) {
          setAvailableGroups([]);
          return;
        }
        throw new Error(message);
      }

      const parsed = parseGroupsPayload(payload);
      setAvailableGroups(Array.isArray(parsed.groups) ? normalizeAdminGroups(parsed.groups) : []);
    } catch (error) {
      setGlobalError(error instanceof Error ? error.message : "Failed to load available groups");
    } finally {
      setIsLoadingGroups(false);
    }
  }, [buildHeaders, handleAuthFailure, sessionToken]);

  useEffect(() => {
    if (!isHydrated) return;
    void Promise.all([loadConfigs(), loadGroups()]);
  }, [isHydrated, loadConfigs, loadGroups]);

  // ── Load templates for a config ────────────────────────────────

  const loadTemplates = useCallback(
    async (code: string) => {
      setIsLoadingTemplates(true);
      try {
        const response = await fetch(
          `/api/admin/config-center/software/${encodeURIComponent(code)}/templates`,
          { method: "GET", headers: buildHeaders(), cache: "no-store" },
        );
        const payload = (await response.json()) as unknown;
        if (!response.ok) {
          throw new Error(extractApiError(payload, "Failed to load templates"));
        }
        const parsed = unwrapData<TemplatesResponse>(payload) ?? ((asRecord(payload) as TemplatesResponse | null) ?? {});
        setTemplates(Array.isArray(parsed.templates) ? parsed.templates : []);
      } catch (error) {
        setFormError(error instanceof Error ? error.message : "Failed to load templates");
      } finally {
        setIsLoadingTemplates(false);
      }
    },
    [buildHeaders],
  );

  // ── Load global vars ────────────────────────────────────────────

  const loadGlobalVars = useCallback(async () => {
    if (!sessionToken) return;

    setIsLoadingGlobalVars(true);
    try {
      const response = await fetch("/api/admin/config-center/global-vars", {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to load global vars"));
      }
      const parsed = unwrapData<GlobalVarsResponse>(payload) ?? ((asRecord(payload) as GlobalVarsResponse | null) ?? {});
      setGlobalVars(Array.isArray(parsed.vars) ? parsed.vars : []);
    } catch (error) {
      setGlobalError(error instanceof Error ? error.message : "Failed to load global vars");
    } finally {
      setIsLoadingGlobalVars(false);
    }
  }, [buildHeaders, sessionToken]);

  useEffect(() => {
    if (!isHydrated || activeTab !== "global-vars") return;
    void loadGlobalVars();
  }, [isHydrated, activeTab, loadGlobalVars]);

  // ── Group lookup ────────────────────────────────────────────────

  const groupByID = useMemo(() => {
    const index = new Map<number, AdminGroup>();
    for (const group of availableGroups) {
      index.set(group.id, group);
    }
    return index;
  }, [availableGroups]);

  // ── Reset form ──────────────────────────────────────────────────

  const resetForm = useCallback(() => {
    setMode("create");
    setEditingCode(null);
    setSwCode("");
    setSwName("");
    setSwGroupId(0);
    setSwDescription("");
    setSwIsEnabled(true);
    setFormError(null);
    setIsLoadingDetail(false);
    setTags([]);
    setNewTag("");
    setTemplates([]);
    setShowTemplateForm(false);
    setTplEditMode("create");
    setTplEditId(null);
    setTplName("");
    setTplFormat("json");
    setTplContent("");
    setTplIsDefault(false);
    setTplError(null);
  }, []);

  // ── Edit config ─────────────────────────────────────────────────

  const handleEdit = async (code: string) => {
    setRowLoadingCode(code);
    setGlobalSuccess(null);
    setFormError(null);
    setIsLoadingDetail(true);

    try {
      const response = await fetch(`/api/admin/config-center/software/${encodeURIComponent(code)}`, {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(payload, "Failed to load software config");
        if (handleAuthFailure(response.status, message)) {
          setFormError(message);
          return;
        }
        throw new Error(message);
      }

      const cfg = asRecord(payload) as SoftwareConfig | null;
      if (!cfg) throw new Error("Failed to load software config");

      setAuthBlocked(null);
      setMode("edit");
      setEditingCode(cfg.software_code);
      setSwCode(cfg.software_code);
      setSwName(cfg.software_name);
      setSwGroupId(cfg.group_id);
      setSwDescription(cfg.description);
      setSwIsEnabled(cfg.is_enabled);
      setTags(Array.isArray(cfg.tags) ? cfg.tags : []);

      void loadTemplates(code);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to load software config");
    } finally {
      setIsLoadingDetail(false);
      setRowLoadingCode(null);
    }
  };

  // ── Create / Update config ──────────────────────────────────────

  const handleCreateOrUpdate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);
    setGlobalSuccess(null);

    if (!sessionToken) {
      setFormError("Missing session token. Please login first.");
      return;
    }

    const trimmedCode = swCode.trim().toLowerCase().replace(/[^a-z0-9_-]/g, "-");
    const trimmedName = swName.trim();
    if (!trimmedCode && mode === "create") {
      setFormError("Software code is required.");
      return;
    }
    if (!trimmedName) {
      setFormError("Software name is required.");
      return;
    }
    if (!swGroupId) {
      setFormError("Please select a group.");
      return;
    }

    setIsSubmittingForm(true);

    const endpoint = editingCode
      ? `/api/admin/config-center/software/${encodeURIComponent(editingCode)}`
      : "/api/admin/config-center/software";
    const method = editingCode ? "PUT" : "POST";
    const body = editingCode
      ? { software_name: trimmedName, group_id: swGroupId, description: swDescription, is_enabled: swIsEnabled }
      : { software_code: trimmedCode, software_name: trimmedName, group_id: swGroupId, description: swDescription, is_enabled: swIsEnabled };

    try {
      const response = await fetch(endpoint, { method, headers: buildHeaders(), body: JSON.stringify(body) });
      const responsePayload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(responsePayload, "Failed to save software config");
        if (handleAuthFailure(response.status, message)) {
          setFormError(message);
          return;
        }
        throw new Error(message);
      }

      setGlobalSuccess(editingCode ? "Software config updated." : "Software config created.");
      if (!editingCode) {
        resetForm();
      }
      setShowSwDialog(false);
      await loadConfigs();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to save software config");
    } finally {
      setIsSubmittingForm(false);
    }
  };

  // ── Delete config ───────────────────────────────────────────────

  const handleDelete = async (code: string) => {
    const confirmed = window.confirm(`Delete software config "${code}"? This will also delete its tags and templates.`);
    if (!confirmed) return;

    setRowLoadingCode(code);
    setGlobalSuccess(null);
    setGlobalError(null);

    try {
      const response = await fetch(`/api/admin/config-center/software/${encodeURIComponent(code)}`, {
        method: "DELETE",
        headers: buildHeaders(),
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(payload, "Failed to delete software config");
        if (handleAuthFailure(response.status, message)) {
          setGlobalError(message);
          return;
        }
        throw new Error(message);
      }
      setGlobalSuccess("Software config deleted.");
      if (editingCode === code) resetForm();
      await loadConfigs();
    } catch (error) {
      setGlobalError(error instanceof Error ? error.message : "Failed to delete software config");
    } finally {
      setRowLoadingCode(null);
    }
  };

  // ── Tag add / remove ────────────────────────────────────────────

  const handleAddTag = async () => {
    const trimmed = newTag.trim().toLowerCase();
    if (!trimmed || !editingCode) return;
    if (tags.includes(trimmed)) {
      setNewTag("");
      return;
    }

    setTagLoading(true);
    try {
      const response = await fetch(
        `/api/admin/config-center/software/${encodeURIComponent(editingCode)}/tags`,
        { method: "POST", headers: buildHeaders(), body: JSON.stringify({ tag: trimmed }) },
      );
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to add tag"));
      }
      setTags((prev) => [...prev, trimmed]);
      setNewTag("");
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to add tag");
    } finally {
      setTagLoading(false);
    }
  };

  const handleRemoveTag = async (tag: string) => {
    if (!editingCode) return;
    setTagLoading(true);
    try {
      const response = await fetch(
        `/api/admin/config-center/software/${encodeURIComponent(editingCode)}/tags/${encodeURIComponent(tag)}`,
        { method: "DELETE", headers: buildHeaders() },
      );
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to remove tag"));
      }
      setTags((prev) => prev.filter((t) => t !== tag));
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to remove tag");
    } finally {
      setTagLoading(false);
    }
  };

  // ── Template CRUD ───────────────────────────────────────────────

  const resetTemplateForm = () => {
    setShowTemplateForm(false);
    setTplEditMode("create");
    setTplEditId(null);
    setTplName("");
    setTplFormat("json");
    setTplContent("");
    setTplIsDefault(false);
    setTplError(null);
  };

  const handleCreateOrUpdateTemplate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setTplError(null);
    if (!editingCode) return;

    if (!tplName.trim() || !tplContent.trim()) {
      setTplError("Template name and content are required.");
      return;
    }

    setTplSubmitting(true);
    try {
      const body = { name: tplName.trim(), format: tplFormat, content: tplContent, is_default: tplIsDefault };

      let response: Response;
      if (tplEditMode === "edit" && tplEditId) {
        response = await fetch(`/api/admin/config-center/templates/${tplEditId}`, {
          method: "PUT",
          headers: buildHeaders(),
          body: JSON.stringify(body),
        });
      } else {
        response = await fetch(
          `/api/admin/config-center/software/${encodeURIComponent(editingCode)}/templates`,
          { method: "POST", headers: buildHeaders(), body: JSON.stringify(body) },
        );
      }

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to save template"));
      }
      resetTemplateForm();
      void loadTemplates(editingCode);
    } catch (error) {
      setTplError(error instanceof Error ? error.message : "Failed to save template");
    } finally {
      setTplSubmitting(false);
    }
  };

  const handleEditTemplate = (tpl: ConfigTemplate) => {
    setTplEditMode("edit");
    setTplEditId(tpl.id);
    setTplName(tpl.name);
    setTplFormat(tpl.format);
    setTplContent(tpl.content);
    setTplIsDefault(tpl.is_default);
    setShowTemplateForm(true);
    setTplError(null);
  };

  const handleDeleteTemplate = async (tplId: number) => {
    if (!editingCode) return;
    const confirmed = window.confirm("Delete this template?");
    if (!confirmed) return;

    try {
      const response = await fetch(`/api/admin/config-center/templates/${tplId}`, {
        method: "DELETE",
        headers: buildHeaders(),
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to delete template"));
      }
      void loadTemplates(editingCode);
    } catch (error) {
      setTplError(error instanceof Error ? error.message : "Failed to delete template");
    }
  };

  // ── Global vars CRUD ────────────────────────────────────────────

  const handleSetGlobalVar = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setGvError(null);
    if (!gvKey.trim()) {
      setGvError("Variable key is required.");
      return;
    }

    setGvSubmitting(true);
    try {
      const response = await fetch("/api/admin/config-center/global-vars", {
        method: "POST",
        headers: buildHeaders(),
        body: JSON.stringify({ var_key: gvKey.trim(), var_value: gvValue, description: gvDescription }),
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to save global var"));
      }
      setGvKey("");
      setGvValue("");
      setGvDescription("");
      await loadGlobalVars();
    } catch (error) {
      setGvError(error instanceof Error ? error.message : "Failed to save global var");
    } finally {
      setGvSubmitting(false);
    }
  };

  const handleDeleteGlobalVar = async (key: string) => {
    const confirmed = window.confirm(`Delete global variable "${key}"?`);
    if (!confirmed) return;

    try {
      const response = await fetch(`/api/admin/config-center/global-vars/${encodeURIComponent(key)}`, {
        method: "DELETE",
        headers: buildHeaders(),
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to delete global var"));
      }
      await loadGlobalVars();
    } catch (error) {
      setGlobalError(error instanceof Error ? error.message : "Failed to delete global var");
    }
  };

  // ── Render ──────────────────────────────────────────────────────

  const isBlocked = Boolean(authBlocked);

  return (
    <section className="space-y-6">
      {/* Header */}
      <div className="clay-panel space-y-3 p-5">
        <div className="space-y-2">
          <h1 className="section-title">
            <span className="gradient-text">Config Center</span>
          </h1>
          <p className="section-subtitle">
            Manage software default configs, templates, tags, and global variables for client auto-configuration.
          </p>
        </div>
      </div>

      {/* Status bar */}
      <div className="block-card space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Session & Data</h2>
          <button
            className="btn-ghost"
            type="button"
            onClick={() => {
              void Promise.all([loadConfigs(), loadGroups()]);
            }}
            disabled={isLoadingConfigs || isLoadingGroups}
          >
            Refresh data
          </button>
        </div>
        <p className="text-sm text-[var(--portal-muted)]">
          Session token: {isHydrated && sessionToken ? "Loaded from localStorage" : "Not found"}
        </p>
        {authBlocked ? (
          <div className="rounded-xl border border-red-400/40 bg-red-500/10 p-3 text-sm text-red-700 dark:border-red-400/60 dark:bg-red-500/20 dark:text-red-300" role="alert">
            Blocked workflow: {authBlocked}
          </div>
        ) : null}
        {globalSuccess ? (
          <div className="rounded-xl border border-emerald-400/40 bg-emerald-500/10 p-3 text-sm text-emerald-700 dark:border-emerald-400/60 dark:bg-emerald-500/20 dark:text-emerald-300" aria-live="polite">
            {globalSuccess}
          </div>
        ) : null}
        {globalError ? (
          <div className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300" role="alert">
            {globalError}
          </div>
        ) : null}
      </div>

      {/* Tab nav */}
      <div className="flex gap-2">
        <button
          type="button"
          className={`nav-pill ${activeTab === "software" ? "!bg-[var(--portal-clay-strong)] !font-semibold" : ""}`}
          onClick={() => setActiveTab("software")}
        >
          <MaterialIcon name="settings_applications" size={16} className="mr-1" />
          Software Configs
        </button>
        <button
          type="button"
          className={`nav-pill ${activeTab === "global-vars" ? "!bg-[var(--portal-clay-strong)] !font-semibold" : ""}`}
          onClick={() => setActiveTab("global-vars")}
        >
          <MaterialIcon name="variable" size={16} className="mr-1" />
          Global Variables
        </button>
      </div>

      {/* ─── Software Configs Tab ────────────────────────────────── */}
      {activeTab === "software" ? (
        <div className="block-card space-y-4">
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Software Configs</h2>
            <div className="flex items-center gap-3">
              <span className="text-xs text-[var(--portal-muted)]">
                {isLoadingConfigs ? "Loading..." : `${configs.length} config(s)`}
              </span>
              <button
                type="button"
                className="btn-primary px-3 py-1.5 text-xs"
                disabled={isBlocked || isSubmittingForm}
                onClick={() => { resetForm(); setShowSwDialog(true); }}
              >
                + New Config
              </button>
            </div>
          </div>

          {isLoadingConfigs ? (
            <p className="text-sm text-[var(--portal-muted)]">Loading software configs...</p>
          ) : configs.length === 0 ? (
            <div className="rounded-xl border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
              No software configs yet. Create one to define default configurations.
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="min-w-full border-separate border-spacing-y-2 text-sm">
                <thead>
                  <tr className="text-left text-[var(--portal-muted)]">
                    <th className="px-2 py-1">Code</th>
                    <th className="px-2 py-1">Name</th>
                    <th className="px-2 py-1">Group</th>
                    <th className="px-2 py-1">Enabled</th>
                    <th className="px-2 py-1">Tags</th>
                    <th className="px-2 py-1">Updated</th>
                    <th className="px-2 py-1">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {configs.map((cfg) => {
                    const isRowBusy = rowLoadingCode === cfg.software_code;
                    return (
                      <tr key={cfg.software_code} className="rounded-lg bg-[var(--portal-clay)] align-top">
                        <td className="px-2 py-2 font-mono text-xs text-[var(--portal-muted)]">{cfg.software_code}</td>
                        <td className="px-2 py-2 font-medium text-[var(--portal-ink)]">{cfg.software_name}</td>
                        <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">
                          {groupByID.get(cfg.group_id)?.name ?? `#${cfg.group_id}`}
                        </td>
                        <td className="px-2 py-2">
                          <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${cfg.is_enabled ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300" : "bg-slate-500/10 text-slate-500 dark:text-slate-400"}`}>
                            {cfg.is_enabled ? "On" : "Off"}
                          </span>
                        </td>
                        <td className="px-2 py-2">
                          <div className="flex flex-wrap gap-1">
                            {(cfg.tags ?? []).length === 0 ? (
                              <span className="text-xs text-[var(--portal-muted)]">-</span>
                            ) : (
                              (cfg.tags ?? []).map((tag) => (
                                <span key={tag} className="inline-flex rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] px-2 py-0.5 text-xs text-[var(--portal-ink)]">
                                  {tag}
                                </span>
                              ))
                            )}
                          </div>
                        </td>
                        <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">
                          {formatDateTime(cfg.updated_at)}
                        </td>
                        <td className="px-2 py-2">
                          <div className="flex flex-wrap items-center gap-2">
                            <button
                              type="button"
                              className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                              disabled={isBlocked || isRowBusy}
                              onClick={() => { void handleEdit(cfg.software_code).then(() => setShowSwDialog(true)); }}
                            >
                              Edit
                            </button>
                            <button
                              type="button"
                              className="cursor-pointer rounded-xl border border-red-400/40 bg-red-500/10 px-3 py-1.5 text-xs font-semibold text-red-700 dark:border-red-400/60 dark:bg-red-500/20 dark:text-red-300"
                              disabled={isBlocked || isRowBusy || isSubmittingForm}
                              onClick={() => void handleDelete(cfg.software_code)}
                            >
                              Delete
                            </button>
                            {isRowBusy ? <span className="text-xs text-[var(--portal-muted)]">Working...</span> : null}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      ) : null}

      {/* ─── Software Config Dialog ──────────────────────────────── */}
      {showSwDialog ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="relative max-h-[90vh] w-full max-w-5xl overflow-y-auto rounded-2xl border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] p-6 shadow-2xl">
            {/* Dialog header */}
            <div className="flex items-center justify-between gap-3 mb-4">
              <h2 className="text-lg font-semibold text-[var(--portal-ink)]">
                {mode === "create" ? "Create Software Config" : `Edit (${editingCode})`}
              </h2>
              <button
                type="button"
                className="cursor-pointer text-xl leading-none text-[var(--portal-muted)] hover:text-[var(--portal-ink)]"
                onClick={() => setShowSwDialog(false)}
              >
                &times;
              </button>
            </div>

            {formError ? (
              <div className="mb-4 rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300" role="alert">
                {formError}
              </div>
            ) : null}

            {/* Two-column layout */}
            <div className="grid gap-6 lg:grid-cols-2">
              {/* Left: Config info */}
              <div className="space-y-4">
                <form className="grid gap-4" onSubmit={handleCreateOrUpdate}>
                  {isLoadingDetail ? (
                    <p className="text-sm text-[var(--portal-muted)]">Loading detail...</p>
                  ) : (
                    <>
                      <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                        <span>Software Code</span>
                        <input
                          className="field font-mono"
                          type="text"
                          value={swCode}
                          onChange={(e) => setSwCode(e.target.value.toLowerCase().replace(/[^a-z0-9_-]/g, "-"))}
                          disabled={mode === "edit" || isBlocked || isSubmittingForm}
                          required={mode === "create"}
                        />
                      </label>

                      <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                        <span>Software Name</span>
                        <input
                          className="field"
                          type="text"
                          value={swName}
                          onChange={(e) => setSwName(e.target.value)}
                          disabled={isBlocked || isSubmittingForm}
                          required
                        />
                      </label>

                      <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                        <span>Bound Group</span>
                        {isLoadingGroups ? (
                          <p className="text-xs text-[var(--portal-muted)]">Loading groups...</p>
                        ) : (
                          <select
                            className="field"
                            value={swGroupId || ""}
                            onChange={(e) => setSwGroupId(Number(e.target.value))}
                            disabled={isBlocked || isSubmittingForm}
                            required
                          >
                            <option value="">Select a group...</option>
                            {availableGroups.map((group) => (
                              <option key={group.id} value={group.id}>
                                {group.name} #{group.id}
                              </option>
                            ))}
                          </select>
                        )}
                      </label>

                      <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                        <span>Description</span>
                        <textarea
                          className="field min-h-[60px] resize-y"
                          rows={2}
                          value={swDescription}
                          onChange={(e) => setSwDescription(e.target.value)}
                          disabled={isBlocked || isSubmittingForm}
                        />
                      </label>

                      <label className="flex items-center gap-3 text-sm text-[var(--portal-muted)]">
                        <input
                          className="size-4 accent-emerald-500"
                          type="checkbox"
                          checked={swIsEnabled}
                          onChange={(e) => setSwIsEnabled(e.target.checked)}
                          disabled={isBlocked || isSubmittingForm}
                        />
                        <span>Enabled</span>
                      </label>
                    </>
                  )}

                  <div className="flex flex-wrap items-center gap-2">
                    <button className="btn-primary" type="submit" disabled={isBlocked || isSubmittingForm || isLoadingGroups}>
                      {isSubmittingForm ? "Saving..." : mode === "create" ? "Create config" : "Save changes"}
                    </button>
                    <button
                      className="btn-ghost"
                      type="button"
                      disabled={isSubmittingForm}
                      onClick={() => { resetForm(); setShowSwDialog(false); }}
                    >
                      Cancel
                    </button>
                  </div>
                </form>

                {/* Tags */}
                {mode === "edit" && editingCode ? (
                  <fieldset className="grid gap-3 rounded-xl border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
                    <legend className="px-1 text-sm font-semibold text-[var(--portal-ink)]">Tags</legend>
                    <p className="text-xs text-[var(--portal-muted)]">
                      Tags are used by clients to auto-discover the matching software config. e.g. &quot;opencode&quot;, &quot;claude-code&quot;
                    </p>
                    <div className="flex flex-wrap gap-2">
                      {tags.map((tag) => (
                        <span
                          key={tag}
                          className="inline-flex items-center gap-1 rounded-full border border-emerald-400/40 bg-emerald-500/10 px-2 py-1 text-xs font-semibold text-emerald-700 dark:border-emerald-400/60 dark:bg-emerald-500/20 dark:text-emerald-300"
                        >
                          {tag}
                          <button
                            type="button"
                            className="ml-1 text-emerald-700/60 hover:text-red-500 dark:text-emerald-300/60"
                            disabled={tagLoading}
                            onClick={() => void handleRemoveTag(tag)}
                          >
                            x
                          </button>
                        </span>
                      ))}
                      {tags.length === 0 ? (
                        <span className="text-xs text-[var(--portal-muted)]">No tags added yet.</span>
                      ) : null}
                    </div>
                    <div className="flex gap-2">
                      <input
                        className="field flex-1"
                        type="text"
                        placeholder="New tag (e.g. opencode)"
                        value={newTag}
                        onChange={(e) => setNewTag(e.target.value)}
                        disabled={tagLoading || isBlocked}
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            e.preventDefault();
                            void handleAddTag();
                          }
                        }}
                      />
                      <button
                        type="button"
                        className="btn-ghost px-3 text-xs"
                        disabled={tagLoading || !newTag.trim() || isBlocked}
                        onClick={() => void handleAddTag()}
                      >
                        Add
                      </button>
                    </div>
                  </fieldset>
                ) : null}
              </div>

              {/* Right: Templates */}
              <div className="space-y-4">
                <div className="flex items-center justify-between gap-2">
                  <h3 className="text-sm font-semibold text-[var(--portal-ink)]">Templates</h3>
                  {mode === "edit" && editingCode ? (
                    <button
                      type="button"
                      className="btn-ghost px-3 text-xs"
                      disabled={isBlocked}
                      onClick={() => { resetTemplateForm(); setShowTemplateForm(true); }}
                    >
                      + New Template
                    </button>
                  ) : null}
                </div>
                <p className="text-xs text-[var(--portal-muted)]">
                  Templates are configs sent to clients. Use {"{{apikey}}"}, {"{{modelname}}"} etc. as placeholders.
                  {mode === "create" ? " Save the config first, then add templates." : null}
                </p>

                {mode === "edit" && editingCode ? (
                  <>
                    {isLoadingTemplates ? (
                      <p className="text-sm text-[var(--portal-muted)]">Loading templates...</p>
                    ) : templates.length === 0 ? (
                      <p className="text-sm text-[var(--portal-muted)]">No templates yet.</p>
                    ) : (
                      <div className="grid gap-2">
                        {templates.map((tpl) => (
                          <div
                            key={tpl.id}
                            className="flex items-center justify-between gap-3 rounded-xl border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] px-3 py-2"
                          >
                            <div className="grid gap-0.5">
                              <span className="text-sm font-medium text-[var(--portal-ink)]">
                                {tpl.name}
                                {tpl.is_default ? (
                                  <span className="ml-2 inline-flex rounded-full bg-blue-500/10 px-2 py-0.5 text-[10px] font-semibold text-blue-700 dark:text-blue-300">
                                    default
                                  </span>
                                ) : null}
                              </span>
                              <span className="text-xs text-[var(--portal-muted)]">
                                {tpl.format} &middot; {tpl.content.length} chars &middot; {formatDateTime(tpl.updated_at)}
                              </span>
                            </div>
                            <div className="flex gap-2">
                              <button
                                type="button"
                                className="btn-ghost cursor-pointer px-2 text-xs"
                                disabled={isBlocked}
                                onClick={() => handleEditTemplate(tpl)}
                              >
                                Edit
                              </button>
                              <button
                                type="button"
                                className="cursor-pointer px-2 text-xs text-red-500 hover:text-red-700"
                                disabled={isBlocked}
                                onClick={() => void handleDeleteTemplate(tpl.id)}
                              >
                                Delete
                              </button>
                            </div>
                          </div>
                        ))}
                      </div>
                    )}

                    {/* Template form */}
                    {showTemplateForm ? (
                      <form className="grid gap-3 rounded-xl border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3" onSubmit={handleCreateOrUpdateTemplate}>
                        <h4 className="text-sm font-semibold text-[var(--portal-ink)]">
                          {tplEditMode === "create" ? "New Template" : `Edit Template #${tplEditId}`}
                        </h4>
                        {tplError ? (
                          <div className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-2 text-xs text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300" role="alert">
                            {tplError}
                          </div>
                        ) : null}
                        <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                          <span>Name</span>
                          <input className="field" type="text" value={tplName} onChange={(e) => setTplName(e.target.value)} disabled={isBlocked || tplSubmitting} required />
                        </label>
                        <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                          <span>Format</span>
                          <select className="field" value={tplFormat} onChange={(e) => setTplFormat(e.target.value)} disabled={isBlocked || tplSubmitting}>
                            <option value="json">JSON</option>
                            <option value="yaml">YAML</option>
                            <option value="toml">TOML</option>
                            <option value="cli">CLI</option>
                            <option value="text">Text</option>
                          </select>
                        </label>
                        <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                          <span>Content</span>
                          <textarea className="field min-h-[120px] resize-y font-mono text-xs" rows={8} value={tplContent} onChange={(e) => setTplContent(e.target.value)} disabled={isBlocked || tplSubmitting} required />
                        </label>
                        <label className="flex items-center gap-3 text-sm text-[var(--portal-muted)]">
                          <input className="size-4 accent-emerald-500" type="checkbox" checked={tplIsDefault} onChange={(e) => setTplIsDefault(e.target.checked)} disabled={isBlocked || tplSubmitting} />
                          <span>Default template</span>
                        </label>
                        <div className="flex gap-2">
                          <button className="btn-primary px-3 py-1.5 text-xs" type="submit" disabled={isBlocked || tplSubmitting}>
                            {tplSubmitting ? "Saving..." : "Save template"}
                          </button>
                          <button className="btn-ghost px-3 py-1.5 text-xs" type="button" disabled={tplSubmitting} onClick={resetTemplateForm}>
                            Cancel
                          </button>
                        </div>
                      </form>
                    ) : null}
                  </>
                ) : (
                  <div className="rounded-xl border border-dashed border-[var(--portal-line)] p-6 text-center text-sm text-[var(--portal-muted)]">
                    Save the config first, then you can add templates and tags here.
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      ) : null}

      {/* ─── Global Variables Tab ────────────────────────────────── */}
      {activeTab === "global-vars" ? (
        <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
          {/* List */}
          <div className="block-card space-y-4">
            <div className="flex items-center justify-between gap-3">
              <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Global Variables</h2>
              <span className="text-xs text-[var(--portal-muted)]">
                {isLoadingGlobalVars ? "Loading..." : `${globalVars.length} variable(s)`}
              </span>
            </div>

            <p className="text-xs text-[var(--portal-muted)]">
              Global variables are used as placeholders in templates. Use {"{{var_key}}"} in template content to reference them.
            </p>

            {isLoadingGlobalVars ? (
              <p className="text-sm text-[var(--portal-muted)]">Loading...</p>
            ) : globalVars.length === 0 ? (
              <div className="rounded-xl border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
                No global variables defined yet.
              </div>
            ) : (
              <div className="overflow-x-auto">
                <table className="min-w-full border-separate border-spacing-y-2 text-sm">
                  <thead>
                    <tr className="text-left text-[var(--portal-muted)]">
                      <th className="px-2 py-1">Key</th>
                      <th className="px-2 py-1">Value</th>
                      <th className="px-2 py-1">Description</th>
                      <th className="px-2 py-1">Updated</th>
                      <th className="px-2 py-1">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {globalVars.map((v) => (
                      <tr key={v.var_key} className="rounded-lg bg-[var(--portal-clay)] align-top">
                        <td className="px-2 py-2 font-mono text-xs text-[var(--portal-ink)]">{"{{" + v.var_key + "}}"}</td>
                        <td className="max-w-[200px] truncate px-2 py-2 text-xs text-[var(--portal-ink)]">{v.var_value || "-"}</td>
                        <td className="max-w-[200px] truncate px-2 py-2 text-xs text-[var(--portal-muted)]">{v.description || "-"}</td>
                        <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">{formatDateTime(v.updated_at)}</td>
                        <td className="px-2 py-2">
                          <button
                            type="button"
                            className="cursor-pointer px-2 text-xs text-red-500 hover:text-red-700"
                            disabled={isBlocked}
                            onClick={() => void handleDeleteGlobalVar(v.var_key)}
                          >
                            Delete
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Form */}
          <div className="block-card space-y-4">
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Add / Update Variable</h2>
            {gvError ? (
              <div className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300" role="alert">
                {gvError}
              </div>
            ) : null}
            <form className="grid gap-4" onSubmit={handleSetGlobalVar}>
              <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                <span>Variable Key</span>
                <input
                  className="field font-mono"
                  type="text"
                  value={gvKey}
                  onChange={(e) => setGvKey(e.target.value)}
                  disabled={isBlocked || gvSubmitting}
                  required
                  placeholder="e.g. modelname, base_url"
                />
              </label>
              <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                <span>Value</span>
                <input
                  className="field"
                  type="text"
                  value={gvValue}
                  onChange={(e) => setGvValue(e.target.value)}
                  disabled={isBlocked || gvSubmitting}
                  placeholder="e.g. gpt-4, https://api.example.com"
                />
              </label>
              <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                <span>Description</span>
                <input
                  className="field"
                  type="text"
                  value={gvDescription}
                  onChange={(e) => setGvDescription(e.target.value)}
                  disabled={isBlocked || gvSubmitting}
                />
              </label>
              <button className="btn-primary" type="submit" disabled={isBlocked || gvSubmitting}>
                {gvSubmitting ? "Saving..." : "Save variable"}
              </button>
            </form>
          </div>
        </div>
      ) : null}
    </section>
  );
}
