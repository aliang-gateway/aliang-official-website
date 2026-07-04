"use client";

import { type ChangeEvent, type FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import { asRecord, extractApiError, unwrapData } from "@/lib/api-response";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type AdminGroup = {
  id: number;
  name: string;
  code?: string;
  platform?: string;
  subscription_type?: string;
  type?: string;
  billing_type?: string;
};

type AdminPackage = {
  code: string;
  name: string;
  level?: "admin" | "distributor";
  group_ids: number[];
  price_micros: number;
  value_type: string;
  value_amount: number;
  description: string;
  features: string[];
  is_enabled: boolean;
  is_visible: boolean;
  is_published: boolean;
  created_at: string;
  updated_at: string;
};

type PackagesResponse = {
  packages?: AdminPackage[];
};

type GroupsResponse = {
  groups?: AdminGroup[];
};

type PackageFormState = {
  code: string;
  name: string;
  level: "admin" | "distributor";
  groupIds: number[];
  priceMicros: number;
  valueType: string;
  valueAmount: number;
  description: string;
  features: string[];
  isVisible: boolean;
  isPublished: boolean;
};

const defaultFormState: PackageFormState = {
  code: "",
  name: "",
  level: "admin",
  groupIds: [],
  priceMicros: 0,
  valueType: "",
  valueAmount: 0,
  description: "",
  features: [],
  isVisible: true,
  isPublished: true,
};

function formatDateTime(value?: string | null) {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat("en-US", {
    year: "numeric",
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function normalizeCode(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

function parsePackagesPayload(payload: unknown) {
  return unwrapData<PackagesResponse>(payload) ?? ((asRecord(payload) as PackagesResponse | null) ?? {});
}

function parseGroupsPayload(payload: unknown) {
  return unwrapData<GroupsResponse>(payload) ?? ((asRecord(payload) as GroupsResponse | null) ?? {});
}

function normalizeAdminGroups(groups: AdminGroup[]) {
  const seenIDs = new Set<number>();
  const normalized: AdminGroup[] = [];

  for (const group of groups) {
    const id = Number(group.id) || 0;
    if (id <= 0 || seenIDs.has(id)) {
      continue;
    }

    seenIDs.add(id);
    normalized.push({
      ...group,
      id,
      name: String(group.name ?? "").trim() || `Group #${id}`,
      code: String(group.code ?? "").trim() || undefined,
      platform: String(group.platform ?? "").trim() || undefined,
      subscription_type: String(group.subscription_type ?? "").trim() || undefined,
      type: String(group.type ?? "").trim() || undefined,
      billing_type: String(group.billing_type ?? "").trim() || undefined,
    });
  }

  return normalized;
}

function isSubscriptionGroup(group: AdminGroup) {
  const billingType = String(group.billing_type ?? "").trim().toLowerCase();
  if (billingType === "subscription") {
    return true;
  }
  if (billingType === "balance") {
    return false;
  }
  const rawType = String(group.subscription_type ?? group.type ?? "").trim().toLowerCase();
  return rawType !== "" && rawType !== "standard" && rawType !== "balance";
}

function groupBillingLabel(group: AdminGroup) {
  return isSubscriptionGroup(group) ? "Subscription group" : "Balance group";
}

function addFeature(features: string[]): string[] {
  return [...features, ""];
}

function updateFeature(features: string[], index: number, value: string): string[] {
  return features.map((f: string, i: number) => (i === index ? value : f));
}

function removeFeature(features: string[], index: number): string[] {
  return features.filter((_: string, i: number) => i !== index);
}

export default function AdminPackagesPage() {
  const [sessionToken, setSessionToken] = useState("");
  const [isHydrated, setIsHydrated] = useState(false);

  const [packages, setPackages] = useState<AdminPackage[]>([]);
  const [availableGroups, setAvailableGroups] = useState<AdminGroup[]>([]);
  const [isLoadingPackages, setIsLoadingPackages] = useState(false);
  const [isLoadingGroups, setIsLoadingGroups] = useState(false);
  const [globalError, setGlobalError] = useState<string | null>(null);
  const [globalSuccess, setGlobalSuccess] = useState<string | null>(null);
  const [authBlocked, setAuthBlocked] = useState<string | null>(null);

  const [mode, setMode] = useState<"create" | "edit">("create");
  const [editingCode, setEditingCode] = useState<string | null>(null);
  const [formState, setFormState] = useState<PackageFormState>(defaultFormState);
  const [formError, setFormError] = useState<string | null>(null);
  const [isSubmittingForm, setIsSubmittingForm] = useState(false);
  const [isLoadingDetail, setIsLoadingDetail] = useState(false);
  const [rowLoadingCode, setRowLoadingCode] = useState<string | null>(null);
  const [showDialog, setShowDialog] = useState(false);

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

  const resetForm = useCallback(() => {
    setMode("create");
    setEditingCode(null);
    setFormState(defaultFormState);
    setFormError(null);
    setIsLoadingDetail(false);
  }, []);

  const handleAuthFailure = useCallback((status: number, message?: string) => {
    if (status === 401 || status === 403) {
      setAuthBlocked(message ?? "Unauthorized. Admin permission is required.");
      return true;
    }
    return false;
  }, []);

  const loadPackages = useCallback(async () => {
    if (!sessionToken) {
      setPackages([]);
      setGlobalError("Missing session token. Please login from /account.");
      setAuthBlocked("Blocked: no session token found.");
      return;
    }

    setIsLoadingPackages(true);
    setGlobalError(null);

    try {
      const response = await fetch("/api/admin/packages", {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(payload, "Failed to load packages");
        if (handleAuthFailure(response.status, message)) {
          setPackages([]);
          setGlobalError(message);
          return;
        }
        throw new Error(message);
      }

      setAuthBlocked(null);
      const parsed = parsePackagesPayload(payload);
      setPackages(Array.isArray(parsed.packages) ? parsed.packages : []);
    } catch (error) {
      setGlobalError(error instanceof Error ? error.message : "Failed to load packages");
    } finally {
      setIsLoadingPackages(false);
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
    if (!isHydrated) {
      return;
    }

    void Promise.all([loadPackages(), loadGroups()]);
  }, [isHydrated, loadGroups, loadPackages]);

  const handleFormChange = useCallback(
    (key: keyof PackageFormState, value: string | string[] | number[] | boolean) => {
      setFormError(null);
      setGlobalSuccess(null);
      setFormState((prev) => {
        if (key === "code") {
          return { ...prev, code: normalizeCode(String(value)) };
        }
        if (key === "groupIds") {
          return { ...prev, groupIds: Array.isArray(value) ? (value as number[]) : prev.groupIds };
        }
        if (key === "isVisible" || key === "isPublished") {
          return { ...prev, [key]: value as boolean };
        }
        if (key === "level") {
          return { ...prev, level: value === "distributor" ? value : "admin" };
        }
        if (key === "priceMicros" || key === "valueAmount") {
          return { ...prev, [key]: Math.max(0, parseInt(String(value), 10) || 0) };
        }
        if (key === "features") {
          return { ...prev, features: Array.isArray(value) ? (value as string[]) : prev.features };
        }
        return { ...prev, [key]: String(value) };
      });
    },
    [],
  );

  const toggleGroupID = (groupID: number) => {
    setFormError(null);
    setGlobalSuccess(null);
    setFormState((previous: PackageFormState) => {
      const isSelected = previous.groupIds.includes(groupID);
      return {
        ...previous,
        groupIds: isSelected
          ? previous.groupIds.filter((id: number) => id !== groupID)
          : [...previous.groupIds, groupID],
      };
    });
  };

  const availableGroupByID = useMemo(() => {
    const index = new Map<number, AdminGroup>();
    for (const group of availableGroups) {
      index.set(group.id, group);
    }
    return index;
  }, [availableGroups]);

  const selectedGroups = useMemo(
    () =>
      formState.groupIds.map((groupID: number) => {
        const group = availableGroupByID.get(groupID);
        return group ?? { id: groupID, name: `Group #${groupID}` };
      }),
    [availableGroupByID, formState.groupIds],
  );

  const handleEdit = async (packageCode: string) => {
    setRowLoadingCode(packageCode);
    setGlobalSuccess(null);
    setFormError(null);
    setIsLoadingDetail(true);

    try {
      const response = await fetch(`/api/admin/packages/${encodeURIComponent(packageCode)}`, {
        method: "GET",
        headers: buildHeaders(),
        cache: "no-store",
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(payload, "Failed to load package detail");
        if (handleAuthFailure(response.status, message)) {
          setFormError(message);
          return;
        }
        throw new Error(message);
      }

      const pkg = unwrapData<AdminPackage>(payload) ?? ((asRecord(payload) as AdminPackage | null) ?? null);
      if (!pkg) {
        throw new Error("Failed to load package detail");
      }

      setAuthBlocked(null);
      setMode("edit");
      setEditingCode(pkg.code);
      setFormState({
        code: pkg.code,
        name: pkg.name,
        level: pkg.level === "distributor" ? pkg.level : "admin",
        groupIds: Array.isArray(pkg.group_ids) ? pkg.group_ids.map((id: number) => Number(id)).filter((id: number) => id > 0) : [],
        priceMicros: Number(pkg.price_micros) || 0,
        valueType: String(pkg.value_type ?? ""),
        valueAmount: Number(pkg.value_amount) || 0,
        description: String(pkg.description ?? ""),
        features: Array.isArray(pkg.features) ? pkg.features : [],
        isVisible: pkg.is_visible !== false,
        isPublished: (pkg.is_published ?? pkg.is_enabled) !== false,
      });
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to load package detail");
    } finally {
      setIsLoadingDetail(false);
      setRowLoadingCode(null);
    }
  };

  const handleCreateOrUpdate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);
    setGlobalSuccess(null);

    const normalizedCode = normalizeCode(formState.code);
    const trimmedName = formState.name.trim();
    const uniqueGroupIDs = Array.from(new Set(formState.groupIds)).filter((id: number) => id > 0);

    if (!sessionToken) {
      setFormError("Missing session token. Please login first.");
      return;
    }
    if (mode === "create" && !normalizedCode) {
      setFormError("Package code is required.");
      return;
    }
    if (!trimmedName) {
      setFormError("Package name is required.");
      return;
    }
    if (uniqueGroupIDs.length === 0) {
      setFormError("Select at least one group to bind to this package.");
      return;
    }

    setIsSubmittingForm(true);

    const endpoint = editingCode
      ? `/api/admin/packages/${encodeURIComponent(editingCode)}`
      : "/api/admin/packages";
    const method = editingCode ? "PUT" : "POST";
    const payload = editingCode
      ? {
          name: trimmedName,
          level: formState.level,
          group_ids: uniqueGroupIDs,
          price_micros: formState.priceMicros,
          value_type: formState.valueType,
          value_amount: formState.valueAmount,
          description: formState.description,
          features_json: JSON.stringify(formState.features.filter((f: string) => f.trim() !== "")),
          is_visible: formState.isVisible,
          is_published: formState.isPublished,
          is_enabled: formState.isPublished,
        }
      : {
          code: normalizedCode,
          name: trimmedName,
          level: formState.level,
          group_ids: uniqueGroupIDs,
          price_micros: formState.priceMicros,
          value_type: formState.valueType,
          value_amount: formState.valueAmount,
          description: formState.description,
          features_json: JSON.stringify(formState.features.filter((f: string) => f.trim() !== "")),
          is_visible: formState.isVisible,
          is_published: formState.isPublished,
          is_enabled: formState.isPublished,
        };

    try {
      const response = await fetch(endpoint, {
        method,
        headers: buildHeaders(),
        body: JSON.stringify(payload),
      });

      const responsePayload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(responsePayload, "Failed to save package");
        if (handleAuthFailure(response.status, message)) {
          setFormError(message);
          return;
        }
        throw new Error(message);
      }

      setGlobalSuccess(editingCode ? "Package updated." : "Package created.");
      resetForm();
      setShowDialog(false);
      await loadPackages();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to save package");
    } finally {
      setIsSubmittingForm(false);
    }
  };

  const handleDelete = async (packageCode: string) => {
    const confirmed = window.confirm(`Delete package "${packageCode}" from this platform?`);
    if (!confirmed) {
      return;
    }

    setRowLoadingCode(packageCode);
    setGlobalError(null);
    setGlobalSuccess(null);
    setFormError(null);

    try {
      const response = await fetch(`/api/admin/packages/${encodeURIComponent(packageCode)}`, {
        method: "DELETE",
        headers: buildHeaders(),
        cache: "no-store",
      });

      const responsePayload = (await response.json()) as unknown;
      if (!response.ok) {
        const message = extractApiError(responsePayload, "Failed to delete package");
        if (handleAuthFailure(response.status, message)) {
          setGlobalError(message);
          return;
        }
        throw new Error(message);
      }

      if (editingCode === packageCode) {
        resetForm();
        setShowDialog(false);
      }
      setGlobalSuccess("Package deleted.");
      await loadPackages();
    } catch (error) {
      setGlobalError(error instanceof Error ? error.message : "Failed to delete package");
    } finally {
      setRowLoadingCode(null);
    }
  };

  const isBlocked = Boolean(authBlocked);

  return (
    <section className="space-y-6">
      <div className="clay-panel space-y-3 p-5">
        <div className="space-y-2">
          <h1 className="section-title">
            <span className="gradient-text">Admin Packages</span>
          </h1>
          <p className="section-subtitle">
            Manage tier-as-package entries and replace their bound admin groups from one workflow.
          </p>
        </div>
      </div>

      <div className="block-card space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Session & Data</h2>
          <button
            className="btn-ghost"
            type="button"
            onClick={() => {
              void Promise.all([loadPackages(), loadGroups()]);
            }}
            disabled={isLoadingPackages || isLoadingGroups}
          >
            Refresh data
          </button>
        </div>
        <p className="text-sm text-[var(--portal-muted)]">
          Session token: {isHydrated && sessionToken ? "Loaded from localStorage" : "Not found"}
        </p>
        {authBlocked ? (
          <div
            className="rounded-xl border border-red-400/40 bg-red-500/10 p-3 text-sm text-red-700 dark:border-red-400/60 dark:bg-red-500/20 dark:text-red-300"
            role="alert"
          >
            Blocked workflow: {authBlocked}
          </div>
        ) : null}
        {globalSuccess ? (
          <div
            className="rounded-xl border border-emerald-400/40 bg-emerald-500/10 p-3 text-sm text-emerald-700 dark:border-emerald-400/60 dark:bg-emerald-500/20 dark:text-emerald-300"
            aria-live="polite"
          >
            {globalSuccess}
          </div>
        ) : null}
        {globalError ? (
          <div
            className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300"
            role="alert"
          >
            {globalError}
          </div>
        ) : null}
      </div>

      {/* Package List */}
      <div className="block-card space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Packages</h2>
          <div className="flex items-center gap-3">
            <span className="text-xs text-[var(--portal-muted)]">
              {isLoadingPackages ? "Loading..." : `${packages.length} package(s)`}
            </span>
            <button
              type="button"
              className="btn-primary px-3 py-1.5 text-xs"
              disabled={isBlocked || isSubmittingForm}
              onClick={() => { resetForm(); setShowDialog(true); }}
            >
              + New Package
            </button>
          </div>
        </div>

        {isLoadingPackages ? (
          <p className="text-sm text-[var(--portal-muted)]">Loading packages...</p>
        ) : packages.length === 0 ? (
          <div className="rounded-xl border border-dashed border-[var(--portal-line)] p-4 text-sm text-[var(--portal-muted)]">
            No packages found yet. Create the first package to bind one or more groups.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full border-separate border-spacing-y-2 text-sm">
              <thead>
                <tr className="text-left text-[var(--portal-muted)]">
                  <th className="px-2 py-1">Code</th>
                  <th className="px-2 py-1">Name</th>
                  <th className="px-2 py-1">Level</th>
                  <th className="px-2 py-1">Price</th>
                  <th className="px-2 py-1">Value</th>
                  <th className="px-2 py-1">Visible</th>
                  <th className="px-2 py-1">Published</th>
                  <th className="px-2 py-1">Bound groups</th>
                  <th className="px-2 py-1">Updated</th>
                  <th className="px-2 py-1">Actions</th>
                </tr>
              </thead>
              <tbody>
                {packages.map((pkg: AdminPackage) => {
                  const isRowBusy = rowLoadingCode === pkg.code;
                  return (
                    <tr key={pkg.code} className="rounded-lg bg-[var(--portal-clay)] align-top">
                      <td className="px-2 py-2 font-mono text-xs text-[var(--portal-muted)]">{pkg.code}</td>
                      <td className="px-2 py-2 font-medium text-[var(--portal-ink)]">{pkg.name}</td>
                      <td className="px-2 py-2">
                        <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${
                          pkg.level === "distributor"
                            ? "bg-violet-500/10 text-violet-700 dark:text-violet-300"
                            : "bg-slate-500/10 text-slate-600 dark:text-slate-300"
                        }`}>
                          {pkg.level === "distributor" ? "Distributor" : "Admin"}
                        </span>
                      </td>
                      <td className="px-2 py-2 text-sm text-[var(--portal-ink)]">
                        {pkg.price_micros > 0 ? `¥${(pkg.price_micros / 1000000).toFixed(2)}` : "Free"}
                      </td>
                      <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">
                        {pkg.value_type
                          ? `${pkg.value_type === "days" ? pkg.value_amount + "d" : "¥" + (pkg.value_amount / 1000000).toFixed(2)}`
                          : "-"}
                      </td>
                      <td className="px-2 py-2">
                        <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${pkg.is_visible ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300" : "bg-slate-500/10 text-slate-500 dark:text-slate-400"}`}>
                          {pkg.is_visible ? "Shown" : "Hidden"}
                        </span>
                      </td>
                      <td className="px-2 py-2">
                        <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${pkg.is_published ? "bg-sky-500/10 text-sky-700 dark:text-sky-300" : "bg-amber-500/10 text-amber-700 dark:text-amber-300"}`}>
                          {pkg.is_published ? "On sale" : "Off sale"}
                        </span>
                      </td>
                      <td className="px-2 py-2">
                        <div className="flex flex-wrap gap-2">
                          {pkg.group_ids.length === 0 ? (
                            <span className="text-xs text-[var(--portal-muted)]">No groups bound</span>
                          ) : (
                            pkg.group_ids.map((groupID: number) => {
                              const group = availableGroupByID.get(groupID);
                              const label = group?.name ?? `Group #${groupID}`;
                              return (
                              <span
                                key={`${pkg.code}-${groupID}`}
                                className="inline-flex rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] px-2 py-1 text-xs text-[var(--portal-ink)]"
                              >
                                {label}
                              </span>
                              );
                            })
                          )}
                        </div>
                      </td>
                      <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">
                        {formatDateTime(pkg.updated_at)}
                      </td>
                      <td className="px-2 py-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                            disabled={isBlocked || isRowBusy || isSubmittingForm}
                            onClick={() => { void handleEdit(pkg.code).then(() => setShowDialog(true)); }}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs text-red-600 hover:text-red-700 dark:text-red-300 dark:hover:text-red-200"
                            disabled={isBlocked || isRowBusy || isSubmittingForm}
                            onClick={() => { void handleDelete(pkg.code); }}
                          >
                            Delete
                          </button>
                          {isRowBusy ? (
                            <span className="text-xs text-[var(--portal-muted)]">Working...</span>
                          ) : null}
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

      {/* Package Dialog */}
      {showDialog ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="relative max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] p-6 shadow-2xl">
            {/* Dialog header */}
            <div className="flex items-center justify-between gap-3 mb-4">
              <h2 className="text-lg font-semibold text-[var(--portal-ink)]">
                {mode === "create" ? "Create Package" : `Edit Package (${editingCode})`}
              </h2>
              <button
                type="button"
                className="cursor-pointer text-xl leading-none text-[var(--portal-muted)] hover:text-[var(--portal-ink)]"
                onClick={() => setShowDialog(false)}
              >
                &times;
              </button>
            </div>

            {formError ? (
              <div className="mb-4 rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300" role="alert">
                {formError}
              </div>
            ) : null}

            {isLoadingDetail ? (
              <p className="mb-4 text-sm text-[var(--portal-muted)]">Loading package detail...</p>
            ) : (
              <form className="grid gap-4" onSubmit={handleCreateOrUpdate}>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Package code</span>
                  <input
                    className="field font-mono"
                    type="text"
                    value={formState.code}
                    onChange={(event: ChangeEvent<HTMLInputElement>) => handleFormChange("code", event.target.value)}
                    disabled={mode === "edit" || isBlocked || isSubmittingForm}
                    required={mode === "create"}
                    aria-describedby="package-code-help"
                  />
                </label>
                <p id="package-code-help" className="-mt-2 text-xs text-[var(--portal-muted)]">
                  Stable package identifier used by the backend tier record. It cannot be changed after creation.
                </p>

                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Package name</span>
                  <input
                    className="field"
                    type="text"
                    value={formState.name}
                    onChange={(event: ChangeEvent<HTMLInputElement>) => handleFormChange("name", event.target.value)}
                    disabled={isBlocked || isSubmittingForm}
                    required
                  />
                </label>

                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Package level</span>
                  <select
                    className="field"
                    value={formState.level}
                    onChange={(event: ChangeEvent<HTMLSelectElement>) => handleFormChange("level", event.target.value)}
                    disabled={isBlocked || isSubmittingForm}
                  >
                    <option value="admin">Admin</option>
                    <option value="distributor">Distributor</option>
                  </select>
                </label>
                <p className="-mt-2 text-xs text-[var(--portal-muted)]">
                  Admin packages stay available to the normal public/admin flow. Distributor packages are only assignable by distributors.
                </p>

                {/* Value Type */}
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Value Type</span>
                  <select
                    className="field"
                    value={formState.valueType}
                    onChange={(e: ChangeEvent<HTMLSelectElement>) => handleFormChange("valueType", e.target.value)}
                    disabled={isBlocked || isSubmittingForm}
                  >
                    <option value="">None (group-only)</option>
                    <option value="days">Subscription (days)</option>
                    <option value="balance">Balance credit</option>
                  </select>
                </label>

                {/* Value Amount */}
                {formState.valueType ? (
                  <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                    <span>{formState.valueType === "days" ? "Days" : "Amount (micros)"}</span>
                    <input
                      className="field"
                      type="number"
                      min="1"
                      value={formState.valueAmount || ""}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => handleFormChange("valueAmount", e.target.value)}
                      disabled={isBlocked || isSubmittingForm}
                      required
                    />
                  </label>
                ) : null}

                {/* Price */}
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Price (CNY, yuan)</span>
                  <input
                    className="field"
                    type="number"
                    min="0"
                    step="0.01"
                    value={formState.priceMicros ? (formState.priceMicros / 1000000).toString() : ""}
                    onChange={(e: ChangeEvent<HTMLInputElement>) => {
                      const yuan = parseFloat(e.target.value) || 0;
                      handleFormChange("priceMicros", String(Math.round(yuan * 1000000)));
                    }}
                    disabled={isBlocked || isSubmittingForm}
                  />
                </label>

                {/* Description */}
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Description</span>
                  <textarea
                    className="field min-h-[80px] resize-y"
                    rows={3}
                    value={formState.description}
                    onChange={(e: ChangeEvent<HTMLTextAreaElement>) => handleFormChange("description", e.target.value)}
                    disabled={isBlocked || isSubmittingForm}
                  />
                </label>

                {/* Features */}
                <fieldset className="grid gap-3 rounded-xl border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
                  <legend className="px-1 text-sm font-semibold text-[var(--portal-ink)]">Features</legend>
                  {formState.features.map((feature: string, index: number) => (
                    <div key={index} className="flex gap-2">
                      <input
                        className="field flex-1"
                        type="text"
                        placeholder={`Feature ${index + 1}`}
                        value={feature}
                        onChange={(e: ChangeEvent<HTMLInputElement>) =>
                          setFormState((prev: PackageFormState) => ({
                            ...prev,
                            features: updateFeature(prev.features, index, e.target.value),
                          }))
                        }
                        disabled={isBlocked || isSubmittingForm}
                      />
                      <button
                        type="button"
                        className="btn-ghost px-2 text-xs text-red-500 hover:text-red-700"
                        onClick={() =>
                          setFormState((prev: PackageFormState) => ({
                            ...prev,
                            features: removeFeature(prev.features, index),
                          }))
                        }
                        disabled={isBlocked || isSubmittingForm}
                      >
                        Remove
                      </button>
                    </div>
                  ))}
                  <button
                    type="button"
                    className="btn-ghost text-xs"
                    onClick={() =>
                      setFormState((prev: PackageFormState) => ({
                        ...prev,
                        features: addFeature(prev.features),
                      }))
                    }
                    disabled={isBlocked || isSubmittingForm}
                  >
                    + Add Feature
                  </button>
                </fieldset>

                <fieldset className="grid gap-3 rounded-xl border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
                  <legend className="px-1 text-sm font-semibold text-[var(--portal-ink)]">Availability</legend>
                  <label className="flex items-center gap-3 text-sm text-[var(--portal-muted)]">
                    <input
                      className="size-4 accent-emerald-500"
                      type="checkbox"
                      checked={formState.isVisible}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => handleFormChange("isVisible", e.target.checked)}
                      disabled={isBlocked || isSubmittingForm}
                    />
                    <span>Show on public package page</span>
                  </label>
                  <label className="flex items-center gap-3 text-sm text-[var(--portal-muted)]">
                    <input
                      className="size-4 accent-sky-500"
                      type="checkbox"
                      checked={formState.isPublished}
                      onChange={(e: ChangeEvent<HTMLInputElement>) => handleFormChange("isPublished", e.target.checked)}
                      disabled={isBlocked || isSubmittingForm}
                    />
                    <span>Published for checkout and admin assignment</span>
                  </label>
                </fieldset>

                {/* Bound groups */}
                <fieldset className="grid gap-3 rounded-xl border border-[var(--portal-line)] bg-[var(--portal-clay)] p-3">
                  <legend className="px-1 text-sm font-semibold text-[var(--portal-ink)]">Bound groups</legend>
                  <p className="text-xs text-[var(--portal-muted)]">
                    Select the groups that should be bound to this package. Saving replaces the full group binding set.
                  </p>

                  {selectedGroups.length > 0 ? (
                    <div className="flex flex-wrap gap-2">
                      {selectedGroups.map((group: AdminGroup) => (
                        <span
                          key={`selected-${group.id}`}
                          className="inline-flex items-center rounded-full border border-emerald-400/40 bg-emerald-500/10 px-2 py-1 text-xs font-semibold text-emerald-700 dark:border-emerald-400/60 dark:bg-emerald-500/20 dark:text-emerald-300"
                        >
                          {group.name}
                          <span className="ml-1 font-mono text-[10px] opacity-75">#{group.id}</span>
                          <span className="ml-1 text-[10px] opacity-75">{groupBillingLabel(group)}</span>
                        </span>
                      ))}
                    </div>
                  ) : (
                    <p className="text-sm text-[var(--portal-muted)]">No groups selected yet.</p>
                  )}

                  {isLoadingGroups ? (
                    <p className="text-sm text-[var(--portal-muted)]">Loading available groups...</p>
                  ) : availableGroups.length === 0 ? (
                    <p className="text-sm text-[var(--portal-muted)]">No available groups returned by backend.</p>
                  ) : (
                    <div className="grid gap-2">
                      {availableGroups.map((group: AdminGroup) => {
                        const isChecked = formState.groupIds.includes(group.id);
                        const meta = [group.platform, groupBillingLabel(group), group.subscription_type || group.type].filter(Boolean).join(" · ");
                        const isDisabled = isBlocked || isSubmittingForm;
                        return (
                          <label
                            key={group.id}
                            className={`flex items-start gap-3 rounded-xl border px-3 py-3 transition-colors ${
                              isDisabled ? "cursor-not-allowed opacity-55" : "cursor-pointer"
                            } ${
                              isChecked
                                ? "border-emerald-400/45 bg-emerald-500/10 dark:border-emerald-400/60 dark:bg-emerald-500/20"
                                : "border-[var(--portal-line)] bg-[var(--portal-clay-strong)] hover:bg-[var(--portal-clay)]"
                            }`}
                          >
                            <input
                              className="mt-1 size-4 accent-emerald-500"
                              type="checkbox"
                              checked={isChecked}
                              onChange={() => toggleGroupID(group.id)}
                              disabled={isDisabled}
                            />
                            <span className="grid gap-1">
                              <span className="text-sm font-semibold text-[var(--portal-ink)]">{group.name}</span>
                              <span className="font-mono text-xs text-[var(--portal-muted)]">#{group.id}</span>
                              {meta ? <span className="text-xs text-[var(--portal-muted)]">{meta}</span> : null}
                            </span>
                          </label>
                        );
                      })}
                    </div>
                  )}
                </fieldset>

                <div className="flex flex-wrap items-center gap-2">
                  <button className="btn-primary" type="submit" disabled={isBlocked || isSubmittingForm || isLoadingGroups}>
                    {isSubmittingForm ? "Saving..." : mode === "create" ? "Create package" : "Save changes"}
                  </button>
                  <button
                    className="btn-ghost"
                    type="button"
                    disabled={isSubmittingForm}
                    onClick={() => { resetForm(); setShowDialog(false); }}
                  >
                    Cancel
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      ) : null}
    </section>
  );
}
