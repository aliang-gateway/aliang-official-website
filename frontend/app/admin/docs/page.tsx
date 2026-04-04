"use client";

import { useCallback, useEffect, useState } from "react";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type DocCategory = {
  id: number;
  slug: string;
  title: string;
  description?: string;
  icon?: string;
  sort_order: number;
  status: string;
  created_at?: string;
  updated_at?: string;
};

type DocPage = {
  id: number;
  slug: string;
  title: string;
  category_id: number;
  mdx_body: string;
  sort_order: number;
  status: string;
  created_at?: string;
  updated_at?: string;
};

type CategoryListResponse = {
  categories?: DocCategory[];
  error?: string;
};

type CategoryDetailResponse = {
  category?: DocCategory;
  error?: string;
};

type PageListResponse = {
  pages?: DocPage[];
  error?: string;
};

type PageDetailResponse = {
  page?: DocPage;
  error?: string;
};

type CategoryFormState = {
  title: string;
  slug: string;
  description: string;
  icon: string;
  sort_order: string;
  status: "draft" | "published";
};

type PageFormState = {
  title: string;
  slug: string;
  category_id: string;
  mdx_body: string;
  sort_order: string;
  status: "draft" | "published";
};

const defaultCategoryFormState: CategoryFormState = {
  title: "",
  slug: "",
  description: "",
  icon: "",
  sort_order: "0",
  status: "draft",
};

const defaultPageFormState: PageFormState = {
  title: "",
  slug: "",
  category_id: "",
  mdx_body: "",
  sort_order: "0",
  status: "draft",
};

function slugify(input: string) {
  return input
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
}

export default function AdminDocsPage() {
  const [sessionToken, setSessionToken] = useState("");
  const [isHydrated, setIsHydrated] = useState(false);

  // Categories state
  const [categories, setCategories] = useState<DocCategory[]>([]);
  const [isLoadingCategories, setIsLoadingCategories] = useState(false);
  const [categoryListError, setCategoryListError] = useState<string | null>(null);

  // Pages state
  const [pages, setPages] = useState<DocPage[]>([]);
  const [isLoadingPages, setIsLoadingPages] = useState(false);
  const [pageListError, setPageListError] = useState<string | null>(null);

  const [globalSuccess, setGlobalSuccess] = useState<string | null>(null);
  const [globalError, setGlobalError] = useState<string | null>(null);

  // Category form
  const [showCategoryDialog, setShowCategoryDialog] = useState(false);
  const [categoryMode, setCategoryMode] = useState<"create" | "edit">("create");
  const [editingCategoryId, setEditingCategoryId] = useState<number | null>(null);
  const [categoryFormState, setCategoryFormState] = useState<CategoryFormState>(defaultCategoryFormState);
  const [isSubmittingCategory, setIsSubmittingCategory] = useState(false);
  const [categoryFormError, setCategoryFormError] = useState<string | null>(null);
  const [isLoadingCategoryDetail, setIsLoadingCategoryDetail] = useState(false);

  // Page form
  const [showPageDialog, setShowPageDialog] = useState(false);
  const [pageMode, setPageMode] = useState<"create" | "edit">("create");
  const [editingPageId, setEditingPageId] = useState<number | null>(null);
  const [pageFormState, setPageFormState] = useState<PageFormState>(defaultPageFormState);
  const [isSubmittingPage, setIsSubmittingPage] = useState(false);
  const [pageFormError, setPageFormError] = useState<string | null>(null);
  const [isLoadingPageDetail, setIsLoadingPageDetail] = useState(false);

  const [rowLoadingId, setRowLoadingId] = useState<number | null>(null);
  const [rowLoadingType, setRowLoadingType] = useState<"category" | "page" | null>(null);

  const [authBlocked, setAuthBlocked] = useState<string | null>(null);

  useEffect(() => {
    setIsHydrated(true);
    const token = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY) ?? "";
    setSessionToken(token);
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

  const resetCategoryForm = useCallback(() => {
    setCategoryFormState(defaultCategoryFormState);
    setCategoryFormError(null);
    setCategoryMode("create");
    setEditingCategoryId(null);
  }, []);

  const resetPageForm = useCallback(() => {
    setPageFormState(defaultPageFormState);
    setPageFormError(null);
    setPageMode("create");
    setEditingPageId(null);
  }, []);

  const loadCategories = useCallback(async () => {
    if (!sessionToken) {
      setCategories([]);
      setCategoryListError("Missing session token. Please login from /account.");
      setAuthBlocked("Blocked: no session token found.");
      return;
    }

    setIsLoadingCategories(true);
    setCategoryListError(null);

    try {
      const response = await fetch("/api/admin/docs/categories", {
        method: "GET",
        headers: {
          ...buildHeaders(),
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as CategoryListResponse;
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setCategories([]);
          setCategoryListError(payload.error ?? "Access denied");
          return;
        }
        throw new Error(payload.error ?? "Failed to load categories");
      }

      setAuthBlocked(null);
      setCategories(payload.categories ?? []);
    } catch (error) {
      setCategoryListError(error instanceof Error ? error.message : "Failed to load categories");
    } finally {
      setIsLoadingCategories(false);
    }
  }, [buildHeaders, handleAuthFailure, sessionToken]);

  const loadPages = useCallback(async () => {
    if (!sessionToken) {
      setPages([]);
      setPageListError("Missing session token. Please login from /account.");
      return;
    }

    setIsLoadingPages(true);
    setPageListError(null);

    try {
      const response = await fetch("/api/admin/docs/pages", {
        method: "GET",
        headers: {
          ...buildHeaders(),
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as PageListResponse;
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setPages([]);
          setPageListError(payload.error ?? "Access denied");
          return;
        }
        throw new Error(payload.error ?? "Failed to load pages");
      }

      setAuthBlocked(null);
      setPages(payload.pages ?? []);
    } catch (error) {
      setPageListError(error instanceof Error ? error.message : "Failed to load pages");
    } finally {
      setIsLoadingPages(false);
    }
  }, [buildHeaders, handleAuthFailure, sessionToken]);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }
    void loadCategories();
    void loadPages();
  }, [isHydrated, loadCategories, loadPages]);

  // Category form handlers
  const handleCategoryFormChange = (key: keyof CategoryFormState, value: string) => {
    setCategoryFormError(null);
    setCategoryFormState((previous) => {
      if (key === "title") {
        const shouldAutoGenerateSlug =
          categoryMode === "create" && (!previous.slug || previous.slug === slugify(previous.title));
        return {
          ...previous,
          title: value,
          slug: shouldAutoGenerateSlug ? slugify(value) : previous.slug,
        };
      }
      if (key === "status") {
        return {
          ...previous,
          status: value === "published" ? "published" : "draft",
        };
      }
      return {
        ...previous,
        [key]: value,
      };
    });
  };

  const handleCreateOrUpdateCategory = async (event: { preventDefault: () => void }) => {
    event.preventDefault();
    setCategoryFormError(null);
    setGlobalSuccess(null);
    setGlobalError(null);

    if (!sessionToken) {
      setCategoryFormError("Missing session token. Please login first.");
      return;
    }
    if (!categoryFormState.title.trim() || !categoryFormState.slug.trim()) {
      setCategoryFormError("Title and slug are required.");
      return;
    }

    setIsSubmittingCategory(true);

    const payload = {
      title: categoryFormState.title.trim(),
      slug: categoryFormState.slug.trim(),
      description: categoryFormState.description.trim(),
      icon: categoryFormState.icon.trim(),
      sort_order: parseInt(categoryFormState.sort_order, 10) || 0,
      status: categoryFormState.status,
    };

    try {
      const endpoint = categoryMode === "edit" && editingCategoryId
        ? `/api/admin/docs/categories/${editingCategoryId}`
        : "/api/admin/docs/categories";
      const method = categoryMode === "edit" ? "PUT" : "POST";

      const response = await fetch(endpoint, {
        method,
        headers: {
          ...buildHeaders(),
        },
        body: JSON.stringify(payload),
      });

      const body = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, body.error)) {
          setCategoryFormError(body.error ?? "Unauthorized");
          return;
        }
        throw new Error(body.error ?? "Failed to save category");
      }

      setGlobalSuccess(categoryMode === "edit" ? "Category updated." : "Category created.");
      resetCategoryForm();
      setShowCategoryDialog(false);
      await loadCategories();
    } catch (error) {
      setCategoryFormError(error instanceof Error ? error.message : "Failed to save category");
    } finally {
      setIsSubmittingCategory(false);
    }
  };

  const handleEditCategory = async (id: number) => {
    setGlobalSuccess(null);
    setCategoryFormError(null);
    setIsLoadingCategoryDetail(true);

    try {
      const response = await fetch(`/api/admin/docs/categories/${id}`, {
        method: "GET",
        headers: {
          ...buildHeaders(),
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as CategoryDetailResponse;
      if (!response.ok || !payload.category) {
        if (handleAuthFailure(response.status, payload.error)) {
          setCategoryFormError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to load category detail");
      }

      setAuthBlocked(null);
      setCategoryMode("edit");
      setEditingCategoryId(id);
      setCategoryFormState({
        title: payload.category.title ?? "",
        slug: payload.category.slug ?? "",
        description: payload.category.description ?? "",
        icon: payload.category.icon ?? "",
        sort_order: String(payload.category.sort_order ?? 0),
        status: payload.category.status === "published" ? "published" : "draft",
      });
    } catch (error) {
      setCategoryFormError(error instanceof Error ? error.message : "Failed to load category detail");
    } finally {
      setIsLoadingCategoryDetail(false);
    }
  };

  const handleTogglePublishCategory = async (category: DocCategory) => {
    const shouldPublish = category.status !== "published";
    const confirmed = window.confirm(
      shouldPublish
        ? `Publish category "${category.title}" now?`
        : `Unpublish category "${category.title}" now?`,
    );
    if (!confirmed) {
      return;
    }

    setRowLoadingId(category.id);
    setRowLoadingType("category");
    setGlobalSuccess(null);
    setGlobalError(null);
    setCategoryListError(null);

    try {
      const path = shouldPublish ? "publish" : "unpublish";
      const response = await fetch(`/api/admin/docs/categories/${category.id}/${path}`, {
        method: "POST",
        headers: {
          ...buildHeaders(),
        },
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setCategoryListError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to update publication status");
      }

      setGlobalSuccess(shouldPublish ? "Category published." : "Category unpublished.");
      await loadCategories();
    } catch (error) {
      setCategoryListError(
        error instanceof Error ? error.message : "Failed to update publication status",
      );
    } finally {
      setRowLoadingId(null);
      setRowLoadingType(null);
    }
  };

  const handleDeleteCategory = async (category: DocCategory) => {
    const confirmed = window.confirm(
      `Delete category "${category.title}"? All associated doc pages will also be deleted. This action cannot be undone.`,
    );
    if (!confirmed) {
      return;
    }

    setRowLoadingId(category.id);
    setRowLoadingType("category");
    setGlobalSuccess(null);
    setGlobalError(null);
    setCategoryListError(null);

    try {
      const response = await fetch(`/api/admin/docs/categories/${category.id}`, {
        method: "DELETE",
        headers: {
          ...buildHeaders(),
        },
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setCategoryListError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to delete category");
      }

      setGlobalSuccess("Category deleted.");
      if (editingCategoryId === category.id) {
        resetCategoryForm();
      }
      await loadCategories();
      await loadPages();
    } catch (error) {
      setCategoryListError(error instanceof Error ? error.message : "Failed to delete category");
    } finally {
      setRowLoadingId(null);
      setRowLoadingType(null);
    }
  };

  // Page form handlers
  const handlePageFormChange = (key: keyof PageFormState, value: string) => {
    setPageFormError(null);
    setPageFormState((previous) => {
      if (key === "title") {
        const shouldAutoGenerateSlug =
          pageMode === "create" && (!previous.slug || previous.slug === slugify(previous.title));
        return {
          ...previous,
          title: value,
          slug: shouldAutoGenerateSlug ? slugify(value) : previous.slug,
        };
      }
      if (key === "status") {
        return {
          ...previous,
          status: value === "published" ? "published" : "draft",
        };
      }
      return {
        ...previous,
        [key]: value,
      };
    });
  };

  const handleCreateOrUpdatePage = async (event: { preventDefault: () => void }) => {
    event.preventDefault();
    setPageFormError(null);
    setGlobalSuccess(null);
    setGlobalError(null);

    if (!sessionToken) {
      setPageFormError("Missing session token. Please login first.");
      return;
    }
    if (!pageFormState.title.trim() || !pageFormState.slug.trim() || !pageFormState.mdx_body.trim()) {
      setPageFormError("Title, slug and MDX body are required.");
      return;
    }
    if (!pageFormState.category_id) {
      setPageFormError("Category is required.");
      return;
    }

    setIsSubmittingPage(true);

    const payload = {
      title: pageFormState.title.trim(),
      slug: pageFormState.slug.trim(),
      category_id: parseInt(pageFormState.category_id, 10),
      mdx_body: pageFormState.mdx_body,
      sort_order: parseInt(pageFormState.sort_order, 10) || 0,
      status: pageFormState.status,
    };

    try {
      const endpoint = pageMode === "edit" && editingPageId
        ? `/api/admin/docs/pages/${editingPageId}`
        : "/api/admin/docs/pages";
      const method = pageMode === "edit" ? "PUT" : "POST";

      const response = await fetch(endpoint, {
        method,
        headers: {
          ...buildHeaders(),
        },
        body: JSON.stringify(payload),
      });

      const body = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, body.error)) {
          setPageFormError(body.error ?? "Unauthorized");
          return;
        }
        throw new Error(body.error ?? "Failed to save page");
      }

      setGlobalSuccess(pageMode === "edit" ? "Doc page updated." : "Doc page created.");
      resetPageForm();
      setShowPageDialog(false);
      await loadPages();
    } catch (error) {
      setPageFormError(error instanceof Error ? error.message : "Failed to save page");
    } finally {
      setIsSubmittingPage(false);
    }
  };

  const handleEditPage = async (id: number) => {
    setGlobalSuccess(null);
    setPageFormError(null);
    setIsLoadingPageDetail(true);

    try {
      const response = await fetch(`/api/admin/docs/pages/${id}`, {
        method: "GET",
        headers: {
          ...buildHeaders(),
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as PageDetailResponse;
      if (!response.ok || !payload.page) {
        if (handleAuthFailure(response.status, payload.error)) {
          setPageFormError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to load page detail");
      }

      setAuthBlocked(null);
      setPageMode("edit");
      setEditingPageId(id);
      setPageFormState({
        title: payload.page.title ?? "",
        slug: payload.page.slug ?? "",
        category_id: String(payload.page.category_id ?? ""),
        mdx_body: payload.page.mdx_body ?? "",
        sort_order: String(payload.page.sort_order ?? 0),
        status: payload.page.status === "published" ? "published" : "draft",
      });
    } catch (error) {
      setPageFormError(error instanceof Error ? error.message : "Failed to load page detail");
    } finally {
      setIsLoadingPageDetail(false);
    }
  };

  const handleTogglePublishPage = async (page: DocPage) => {
    const shouldPublish = page.status !== "published";
    const confirmed = window.confirm(
      shouldPublish
        ? `Publish doc page "${page.title}" now?`
        : `Unpublish doc page "${page.title}" now?`,
    );
    if (!confirmed) {
      return;
    }

    setRowLoadingId(page.id);
    setRowLoadingType("page");
    setGlobalSuccess(null);
    setGlobalError(null);
    setPageListError(null);

    try {
      const path = shouldPublish ? "publish" : "unpublish";
      const response = await fetch(`/api/admin/docs/pages/${page.id}/${path}`, {
        method: "POST",
        headers: {
          ...buildHeaders(),
        },
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setPageListError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to update publication status");
      }

      setGlobalSuccess(shouldPublish ? "Doc page published." : "Doc page unpublished.");
      await loadPages();
    } catch (error) {
      setPageListError(
        error instanceof Error ? error.message : "Failed to update publication status",
      );
    } finally {
      setRowLoadingId(null);
      setRowLoadingType(null);
    }
  };

  const handleDeletePage = async (page: DocPage) => {
    const confirmed = window.confirm(
      `Delete doc page "${page.title}"? This action cannot be undone.`,
    );
    if (!confirmed) {
      return;
    }

    setRowLoadingId(page.id);
    setRowLoadingType("page");
    setGlobalSuccess(null);
    setGlobalError(null);
    setPageListError(null);

    try {
      const response = await fetch(`/api/admin/docs/pages/${page.id}`, {
        method: "DELETE",
        headers: {
          ...buildHeaders(),
        },
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setPageListError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to delete page");
      }

      setGlobalSuccess("Doc page deleted.");
      if (editingPageId === page.id) {
        resetPageForm();
      }
      await loadPages();
    } catch (error) {
      setPageListError(error instanceof Error ? error.message : "Failed to delete page");
    } finally {
      setRowLoadingId(null);
      setRowLoadingType(null);
    }
  };

  const getCategoryTitle = (categoryId: number) => {
    const category = categories.find((c) => c.id === categoryId);
    return category ? category.title : `#${categoryId}`;
  };

  const isBlocked = Boolean(authBlocked);

  return (
    <section className="space-y-6">
      <div className="clay-panel space-y-2 p-5">
        <h1 className="section-title">
          <span className="gradient-text">Admin Docs</span>
        </h1>
        <p className="section-subtitle">
          Manage doc categories and doc pages with secure session-token authorization.
        </p>
      </div>

      <div className="block-card space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Session & Access</h2>
          <div className="flex items-center gap-2">
            <button className="btn-ghost" type="button" onClick={() => void loadCategories()}>
              Refresh categories
            </button>
            <button className="btn-ghost" type="button" onClick={() => void loadPages()}>
              Refresh pages
            </button>
          </div>
        </div>
        <p className="text-sm text-[var(--portal-muted)]">
          Session token: {isHydrated && sessionToken ? "Loaded from localStorage" : "Not found"}
        </p>
        {authBlocked ? (
          <div
            className="rounded-xl border border-red-400/40 dark:border-red-400/60 bg-red-500/10 dark:bg-red-500/20 p-3 text-sm text-red-700 dark:text-red-300"
            role="alert"
          >
            Blocked workflow: {authBlocked}
          </div>
        ) : null}
        {globalSuccess ? (
          <div
            className="rounded-xl border border-emerald-400/40 dark:border-emerald-400/60 bg-emerald-500/10 dark:bg-emerald-500/20 p-3 text-sm text-emerald-700 dark:text-emerald-300"
            role="status"
          >
            {globalSuccess}
          </div>
        ) : null}
        {globalError ? (
          <div
            className="rounded-xl border border-amber-400/45 dark:border-amber-400/60 bg-amber-500/10 dark:bg-amber-500/20 p-3 text-sm text-amber-700 dark:text-amber-300"
            role="alert"
          >
            {globalError}
          </div>
        ) : null}
      </div>

      {/* ===== Category Management Section ===== */}
      <div className="block-card space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Doc Categories</h2>
          <div className="flex items-center gap-3">
            <span className="text-xs text-[var(--portal-muted)]">
              {isLoadingCategories ? "Loading..." : `${categories.length} categor(ies)`}
            </span>
            <button
              type="button"
              className="btn-primary px-3 py-1.5 text-xs"
              disabled={isBlocked || isSubmittingCategory}
              onClick={() => { resetCategoryForm(); setShowCategoryDialog(true); }}
            >
              + New Category
            </button>
          </div>
        </div>

        {categoryListError ? (
          <div
            className="rounded-xl border border-amber-400/45 dark:border-amber-400/60 bg-amber-500/10 dark:bg-amber-500/20 p-3 text-sm text-amber-700 dark:text-amber-300"
            role="alert"
          >
            {categoryListError}
          </div>
        ) : null}

        {isLoadingCategories ? (
          <p className="text-sm text-[var(--portal-muted)]">Loading categories...</p>
        ) : categories.length === 0 ? (
          <p className="text-sm text-[var(--portal-muted)]">No categories found.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full border-separate border-spacing-y-2 text-sm">
              <thead>
                <tr className="text-left text-[var(--portal-muted)]">
                  <th className="px-2 py-1">Title</th>
                  <th className="px-2 py-1">Slug</th>
                  <th className="px-2 py-1">Sort Order</th>
                  <th className="px-2 py-1">Status</th>
                  <th className="px-2 py-1">Actions</th>
                </tr>
              </thead>
              <tbody>
                {categories.map((category) => {
                  const isRowBusy = rowLoadingId === category.id && rowLoadingType === "category";
                  const isPublished = category.status === "published";
                  return (
                    <tr key={category.id} className="rounded-lg bg-[var(--portal-clay)]">
                      <td className="px-2 py-2 font-medium text-[var(--portal-ink)]">{category.title}</td>
                      <td className="px-2 py-2 font-mono text-xs text-[var(--portal-muted)]">{category.slug}</td>
                      <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">{category.sort_order}</td>
                      <td className="px-2 py-2">
                        <span
                          className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${
                            isPublished
                              ? "bg-emerald-500/20 dark:bg-emerald-500/30 text-emerald-700 dark:text-emerald-300"
                              : "bg-[var(--stitch-text-muted)]/20 text-[var(--stitch-text-muted)]"
                          }`}
                        >
                          {category.status}
                        </span>
                      </td>
                      <td className="px-2 py-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => { void handleEditCategory(category.id).then(() => setShowCategoryDialog(true)); }}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => void handleTogglePublishCategory(category)}
                          >
                            {isPublished ? "Unpublish" : "Publish"}
                          </button>
                          <button
                            type="button"
                            className="cursor-pointer rounded-xl border border-red-400/40 dark:border-red-400/60 bg-red-500/10 dark:bg-red-500/20 px-3 py-1.5 text-xs font-semibold text-red-700 dark:text-red-300"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => void handleDeleteCategory(category)}
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

      {/* ===== Page Management Section ===== */}
      <div className="block-card space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Doc Pages</h2>
          <div className="flex items-center gap-3">
            <span className="text-xs text-[var(--portal-muted)]">
              {isLoadingPages ? "Loading..." : `${pages.length} page(s)`}
            </span>
            <button
              type="button"
              className="btn-primary px-3 py-1.5 text-xs"
              disabled={isBlocked || isSubmittingPage}
              onClick={() => { resetPageForm(); setShowPageDialog(true); }}
            >
              + New Page
            </button>
          </div>
        </div>

        {pageListError ? (
          <div
            className="rounded-xl border border-amber-400/45 dark:border-amber-400/60 bg-amber-500/10 dark:bg-amber-500/20 p-3 text-sm text-amber-700 dark:text-amber-300"
            role="alert"
          >
            {pageListError}
          </div>
        ) : null}

        {isLoadingPages ? (
          <p className="text-sm text-[var(--portal-muted)]">Loading doc pages...</p>
        ) : pages.length === 0 ? (
          <p className="text-sm text-[var(--portal-muted)]">No doc pages found.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full border-separate border-spacing-y-2 text-sm">
              <thead>
                <tr className="text-left text-[var(--portal-muted)]">
                  <th className="px-2 py-1">Title</th>
                  <th className="px-2 py-1">Slug</th>
                  <th className="px-2 py-1">Category</th>
                  <th className="px-2 py-1">Sort Order</th>
                  <th className="px-2 py-1">Status</th>
                  <th className="px-2 py-1">Actions</th>
                </tr>
              </thead>
              <tbody>
                {pages.map((page) => {
                  const isRowBusy = rowLoadingId === page.id && rowLoadingType === "page";
                  const isPublished = page.status === "published";
                  return (
                    <tr key={page.id} className="rounded-lg bg-[var(--portal-clay)]">
                      <td className="px-2 py-2 font-medium text-[var(--portal-ink)]">{page.title}</td>
                      <td className="px-2 py-2 font-mono text-xs text-[var(--portal-muted)]">{page.slug}</td>
                      <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">{getCategoryTitle(page.category_id)}</td>
                      <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">{page.sort_order}</td>
                      <td className="px-2 py-2">
                        <span
                          className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${
                            isPublished
                              ? "bg-emerald-500/20 dark:bg-emerald-500/30 text-emerald-700 dark:text-emerald-300"
                              : "bg-[var(--stitch-text-muted)]/20 text-[var(--stitch-text-muted)]"
                          }`}
                        >
                          {page.status}
                        </span>
                      </td>
                      <td className="px-2 py-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => { void handleEditPage(page.id).then(() => setShowPageDialog(true)); }}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => void handleTogglePublishPage(page)}
                          >
                            {isPublished ? "Unpublish" : "Publish"}
                          </button>
                          <button
                            type="button"
                            className="cursor-pointer rounded-xl border border-red-400/40 dark:border-red-400/60 bg-red-500/10 dark:bg-red-500/20 px-3 py-1.5 text-xs font-semibold text-red-700 dark:text-red-300"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => void handleDeletePage(page)}
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

      {/* ===== Category Dialog ===== */}
      {showCategoryDialog ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="relative max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] p-6 shadow-2xl">
            {/* Dialog header */}
            <div className="flex items-center justify-between gap-3 mb-4">
              <h2 className="text-lg font-semibold text-[var(--portal-ink)]">
                {categoryMode === "create" ? "Create Category" : `Edit Category (ID: ${editingCategoryId})`}
              </h2>
              <button
                type="button"
                className="cursor-pointer text-xl leading-none text-[var(--portal-muted)] hover:text-[var(--portal-ink)]"
                onClick={() => setShowCategoryDialog(false)}
              >
                &times;
              </button>
            </div>

            {categoryFormError ? (
              <div className="mb-4 rounded-xl border border-amber-400/45 dark:border-amber-400/60 bg-amber-500/10 dark:bg-amber-500/20 p-3 text-sm text-amber-700 dark:text-amber-300" role="alert">
                {categoryFormError}
              </div>
            ) : null}

            {isLoadingCategoryDetail ? (
              <p className="mb-4 text-sm text-[var(--portal-muted)]">Loading category detail...</p>
            ) : (
              <form className="grid gap-3" onSubmit={handleCreateOrUpdateCategory}>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Title</span>
                  <input
                    className="field"
                    type="text"
                    value={categoryFormState.title}
                    onChange={(event) => handleCategoryFormChange("title", event.target.value)}
                    disabled={isBlocked || isSubmittingCategory}
                    required
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Slug</span>
                  <input
                    className="field"
                    type="text"
                    value={categoryFormState.slug}
                    onChange={(event) => handleCategoryFormChange("slug", slugify(event.target.value))}
                    disabled={isBlocked || isSubmittingCategory}
                    required
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Description</span>
                  <textarea
                    className="field min-h-20 resize-y"
                    value={categoryFormState.description}
                    onChange={(event) => handleCategoryFormChange("description", event.target.value)}
                    disabled={isBlocked || isSubmittingCategory}
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Icon</span>
                  <input
                    className="field"
                    type="text"
                    value={categoryFormState.icon}
                    onChange={(event) => handleCategoryFormChange("icon", event.target.value)}
                    disabled={isBlocked || isSubmittingCategory}
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Sort Order</span>
                  <input
                    className="field"
                    type="number"
                    value={categoryFormState.sort_order}
                    onChange={(event) => handleCategoryFormChange("sort_order", event.target.value)}
                    disabled={isBlocked || isSubmittingCategory}
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Status</span>
                  <select
                    className="field"
                    value={categoryFormState.status}
                    onChange={(event) => handleCategoryFormChange("status", event.target.value)}
                    disabled={isBlocked || isSubmittingCategory}
                  >
                    <option value="draft">draft</option>
                    <option value="published">published</option>
                  </select>
                </label>

                <div className="flex flex-wrap items-center gap-2">
                  <button className="btn-primary" type="submit" disabled={isBlocked || isSubmittingCategory}>
                    {isSubmittingCategory
                      ? "Saving..."
                      : categoryMode === "create"
                        ? "Create category"
                        : "Save changes"}
                  </button>
                  <button
                    className="btn-ghost"
                    type="button"
                    disabled={isSubmittingCategory}
                    onClick={() => { resetCategoryForm(); setShowCategoryDialog(false); }}
                  >
                    Cancel
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      ) : null}

      {/* ===== Page Dialog ===== */}
      {showPageDialog ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
          <div className="relative max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-2xl border border-[var(--portal-line)] bg-[var(--portal-clay-strong)] p-6 shadow-2xl">
            {/* Dialog header */}
            <div className="flex items-center justify-between gap-3 mb-4">
              <h2 className="text-lg font-semibold text-[var(--portal-ink)]">
                {pageMode === "create" ? "Create Doc Page" : `Edit Doc Page (ID: ${editingPageId})`}
              </h2>
              <button
                type="button"
                className="cursor-pointer text-xl leading-none text-[var(--portal-muted)] hover:text-[var(--portal-ink)]"
                onClick={() => setShowPageDialog(false)}
              >
                &times;
              </button>
            </div>

            {pageFormError ? (
              <div className="mb-4 rounded-xl border border-amber-400/45 dark:border-amber-400/60 bg-amber-500/10 dark:bg-amber-500/20 p-3 text-sm text-amber-700 dark:text-amber-300" role="alert">
                {pageFormError}
              </div>
            ) : null}

            {isLoadingPageDetail ? (
              <p className="mb-4 text-sm text-[var(--portal-muted)]">Loading page detail...</p>
            ) : (
              <form className="grid gap-3" onSubmit={handleCreateOrUpdatePage}>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Title</span>
                  <input
                    className="field"
                    type="text"
                    value={pageFormState.title}
                    onChange={(event) => handlePageFormChange("title", event.target.value)}
                    disabled={isBlocked || isSubmittingPage}
                    required
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Slug</span>
                  <input
                    className="field"
                    type="text"
                    value={pageFormState.slug}
                    onChange={(event) => handlePageFormChange("slug", slugify(event.target.value))}
                    disabled={isBlocked || isSubmittingPage}
                    required
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Category</span>
                  <select
                    className="field"
                    value={pageFormState.category_id}
                    onChange={(event) => handlePageFormChange("category_id", event.target.value)}
                    disabled={isBlocked || isSubmittingPage}
                    required
                  >
                    <option value="">-- Select category --</option>
                    {categories.map((category) => (
                      <option key={category.id} value={category.id}>
                        {category.title}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Sort Order</span>
                  <input
                    className="field"
                    type="number"
                    value={pageFormState.sort_order}
                    onChange={(event) => handlePageFormChange("sort_order", event.target.value)}
                    disabled={isBlocked || isSubmittingPage}
                  />
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>Status</span>
                  <select
                    className="field"
                    value={pageFormState.status}
                    onChange={(event) => handlePageFormChange("status", event.target.value)}
                    disabled={isBlocked || isSubmittingPage}
                  >
                    <option value="draft">draft</option>
                    <option value="published">published</option>
                  </select>
                </label>
                <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
                  <span>MDX body</span>
                  <textarea
                    className="field min-h-72 resize-y font-mono"
                    value={pageFormState.mdx_body}
                    onChange={(event) => handlePageFormChange("mdx_body", event.target.value)}
                    disabled={isBlocked || isSubmittingPage}
                    required
                  />
                </label>

                <div className="flex flex-wrap items-center gap-2">
                  <button className="btn-primary" type="submit" disabled={isBlocked || isSubmittingPage}>
                    {isSubmittingPage
                      ? "Saving..."
                      : pageMode === "create"
                        ? "Create draft"
                        : "Save changes"}
                  </button>
                  <button
                    className="btn-ghost"
                    type="button"
                    disabled={isSubmittingPage}
                    onClick={() => { resetPageForm(); setShowPageDialog(false); }}
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
