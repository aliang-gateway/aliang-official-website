"use client";

import Link from "next/link";
import { type ChangeEvent, type FormEvent, useCallback, useEffect, useMemo, useState } from "react";

import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asRecord, extractApiError, unwrapData } from "@/lib/api-response";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type AdminGroup = {
  id: number;
  name: string;
  code?: string;
  platform?: string;
  subscription_type?: string;
  type?: string;
};

type AdminPackage = {
  code: string;
  name: string;
  group_ids: number[];
  price_micros: number;
  value_type: string;
  value_amount: number;
  description: string;
  features: string[];
  is_enabled: boolean;
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
  groupIds: number[];
  priceMicros: number;
  valueType: string;
  valueAmount: number;
  description: string;
  features: string[];
  isEnabled: boolean;
};

const defaultFormState: PackageFormState = {
  code: "",
  name: "",
  groupIds: [],
  priceMicros: 0,
  valueType: "",
  valueAmount: 0,
  description: "",
  features: [],
  isEnabled: true,
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
    });
  }

  return normalized;
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
        if (key === "isEnabled") {
          return { ...prev, isEnabled: value as boolean };
        }
        if (key === "priceMicros" || key === "valueAmount") {
          return { ...prev, [key]: Math.max(0, parseInt(String(value), 10) || 0) };
        }
        if (key === "features") {
          return { ...prev, features: Array.isArray(value) ? value : prev.features };
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
        groupIds: Array.isArray(pkg.group_ids) ? pkg.group_ids.map((id: number) => Number(id)).filter((id: number) => id > 0) : [],
        priceMicros: Number(pkg.price_micros) || 0,
        valueType: String(pkg.value_type ?? ""),
        valueAmount: Number(pkg.value_amount) || 0,
        description: String(pkg.description ?? ""),
        features: Array.isArray(pkg.features) ? pkg.features : [],
        isEnabled: pkg.is_enabled !== false,
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
          group_ids: uniqueGroupIDs,
          price_micros: formState.priceMicros,
          value_type: formState.valueType,
          value_amount: formState.valueAmount,
          description: formState.description,
          features_json: JSON.stringify(formState.features.filter((f: string) => f.trim() !== "")),
          is_enabled: formState.isEnabled,
        }
      : {
          code: normalizedCode,
          name: trimmedName,
          group_ids: uniqueGroupIDs,
          price_micros: formState.priceMicros,
          value_type: formState.valueType,
          value_amount: formState.valueAmount,
          description: formState.description,
          features_json: JSON.stringify(formState.features.filter((f: string) => f.trim() !== "")),
          is_enabled: formState.isEnabled,
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
      await loadPackages();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to save package");
    } finally {
      setIsSubmittingForm(false);
    }
  };

  const isBlocked = Boolean(authBlocked);

  return (
    <section className="portal-shell space-y-6 py-8">
      <div className="clay-panel space-y-3 p-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="space-y-2">
            <h1 className="section-title">
              <span className="gradient-text">Admin Packages</span>
            </h1>
            <p className="section-subtitle">
              Manage tier-as-package entries and replace their bound admin groups from one workflow.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Link href="/admin" className="nav-pill">
              <MaterialIcon name="tune" size={16} className="mr-1" />
              Unit Prices
            </Link>
            <Link href="/admin/articles" className="nav-pill">
              <MaterialIcon name="article" size={16} className="mr-1" />
              Articles
            </Link>
          </div>
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

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,0.8fr)]">
        <div className="block-card space-y-4">
          <div className="flex items-center justify-between gap-3">
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Package List</h2>
            <span className="text-xs text-[var(--portal-muted)]">
              {isLoadingPackages ? "Loading..." : `${packages.length} package(s)`}
            </span>
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
                    <th className="px-2 py-1">Price</th>
                    <th className="px-2 py-1">Value</th>
                    <th className="px-2 py-1">Enabled</th>
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
                        <td className="px-2 py-2 text-sm text-[var(--portal-ink)]">
                          {pkg.price_micros > 0 ? `¥${(pkg.price_micros / 1000000).toFixed(2)}` : "Free"}
                        </td>
                        <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">
                          {pkg.value_type
                            ? `${pkg.value_type === "days" ? pkg.value_amount + "d" : "¥" + (pkg.value_amount / 1000000).toFixed(2)}`
                            : "-"}
                        </td>
                        <td className="px-2 py-2">
                          <span className={`inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${pkg.is_enabled ? "bg-emerald-500/10 text-emerald-700 dark:text-emerald-300" : "bg-slate-500/10 text-slate-500 dark:text-slate-400"}`}>
                            {pkg.is_enabled ? "On" : "Off"}
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
                              onClick={() => void handleEdit(pkg.code)}
                            >
                              Edit
                            </button>
                            {isRowBusy ? (
                              <span className="text-xs text-[var(--portal-muted)]">Loading...</span>
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

        <div className="block-card space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h2 className="text-lg font-semibold text-[var(--portal-ink)]">
              {mode === "create" ? "Create Package" : `Edit Package (${editingCode})`}
            </h2>
            {mode === "edit" ? (
              <button className="btn-ghost" type="button" onClick={resetForm} disabled={isSubmittingForm}>
                Switch to create
              </button>
            ) : null}
          </div>

          {isLoadingDetail ? (
            <p className="text-sm text-[var(--portal-muted)]">Loading package detail...</p>
          ) : null}
          {formError ? (
            <div
              className="rounded-xl border border-amber-400/45 bg-amber-500/10 p-3 text-sm text-amber-700 dark:border-amber-400/60 dark:bg-amber-500/20 dark:text-amber-300"
              role="alert"
            >
              {formError}
            </div>
          ) : null}

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

            {/* Is Enabled */}
            <label className="flex items-center gap-3 text-sm text-[var(--portal-muted)]">
              <input
                className="size-4 accent-emerald-500"
                type="checkbox"
                checked={formState.isEnabled}
                onChange={(e: ChangeEvent<HTMLInputElement>) => handleFormChange("isEnabled", e.target.checked)}
                disabled={isBlocked || isSubmittingForm}
              />
              <span>Visible to users (enabled)</span>
            </label>

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
                    const meta = [group.platform, group.subscription_type || group.type].filter(Boolean).join(" · ");
                    return (
                      <label
                        key={group.id}
                        className={`flex cursor-pointer items-start gap-3 rounded-xl border px-3 py-3 transition-colors ${
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
                          disabled={isBlocked || isSubmittingForm}
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
              <button className="btn-ghost" type="button" disabled={isSubmittingForm} onClick={resetForm}>
                Reset
              </button>
            </div>
          </form>
        </div>
      </div>
    </section>
  );
}
