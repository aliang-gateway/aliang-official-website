"use client";

import { useCallback, useEffect, useState } from "react";

const SESSION_TOKEN_STORAGE_KEY = "session_token";

type ArticleSummary = {
  id: number;
  legacy_id?: number;
  slug: string;
  title: string;
  status: string;
  published_at?: string | null;
  created_at?: string;
  updated_at?: string;
};

type ArticleDetail = {
  id?: number;
  legacy_id?: number;
  slug: string;
  title: string;
  excerpt: string;
  cover_image_url: string;
  tag: string;
  read_time: string;
  author_name: string;
  author_avatar_url?: string;
  author_icon?: string;
  mdx_body: string;
  status: string;
  published_at?: string | null;
};

type ArticlesListResponse = {
  articles?: ArticleSummary[];
  error?: string;
};

type ArticleDetailResponse = {
  article?: ArticleDetail;
  error?: string;
};

type FormState = {
  title: string;
  slug: string;
  excerpt: string;
  tag: string;
  cover_image_url: string;
  read_time: string;
  author_name: string;
  author_avatar_url: string;
  author_icon: string;
  mdx_body: string;
  status: "draft" | "published";
};

const defaultFormState: FormState = {
  title: "",
  slug: "",
  excerpt: "",
  tag: "",
  cover_image_url: "",
  read_time: "5 min read",
  author_name: "",
  author_avatar_url: "",
  author_icon: "",
  mdx_body: "",
  status: "draft",
};

function slugifyTitle(input: string) {
  return input
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
}

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

export default function AdminArticlesPage() {
  const [sessionToken, setSessionToken] = useState("");
  const [isHydrated, setIsHydrated] = useState(false);

  const [articles, setArticles] = useState<ArticleSummary[]>([]);
  const [isLoadingList, setIsLoadingList] = useState(false);
  const [listError, setListError] = useState<string | null>(null);
  const [globalSuccess, setGlobalSuccess] = useState<string | null>(null);

  const [isSubmittingForm, setIsSubmittingForm] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [formState, setFormState] = useState<FormState>(defaultFormState);

  const [mode, setMode] = useState<"create" | "edit">("create");
  const [editingSlug, setEditingSlug] = useState<string | null>(null);
  const [isLoadingDetail, setIsLoadingDetail] = useState(false);
  const [rowLoadingSlug, setRowLoadingSlug] = useState<string | null>(null);

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

  const resetForm = useCallback(() => {
    setFormState(defaultFormState);
    setFormError(null);
    setMode("create");
    setEditingSlug(null);
  }, []);

  const handleAuthFailure = useCallback((status: number, message?: string) => {
    if (status === 401 || status === 403) {
      setAuthBlocked(message ?? "Unauthorized. Admin permission is required.");
      return true;
    }
    return false;
  }, []);

  const loadArticles = useCallback(async () => {
    if (!sessionToken) {
      setArticles([]);
      setListError("Missing session token. Please login from /account.");
      setAuthBlocked("Blocked: no session token found.");
      return;
    }

    setIsLoadingList(true);
    setListError(null);

    try {
      const response = await fetch("/api/admin/articles", {
        method: "GET",
        headers: {
          ...buildHeaders(),
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as ArticlesListResponse;
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setArticles([]);
          setListError(payload.error ?? "Access denied");
          return;
        }
        throw new Error(payload.error ?? "Failed to load articles");
      }

      setAuthBlocked(null);
      setArticles(payload.articles ?? []);
    } catch (error) {
      setListError(error instanceof Error ? error.message : "Failed to load articles");
    } finally {
      setIsLoadingList(false);
    }
  }, [buildHeaders, handleAuthFailure, sessionToken]);

  useEffect(() => {
    if (!isHydrated) {
      return;
    }
    void loadArticles();
  }, [isHydrated, loadArticles]);

  const handleFormChange = (key: keyof FormState, value: string) => {
    setFormError(null);
    setFormState((previous) => {
      if (key === "title") {
        const shouldAutoGenerateSlug =
          mode === "create" && (!previous.slug || previous.slug === slugifyTitle(previous.title));
        return {
          ...previous,
          title: value,
          slug: shouldAutoGenerateSlug ? slugifyTitle(value) : previous.slug,
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

  const handleCreateOrUpdate = async (event: { preventDefault: () => void }) => {
    event.preventDefault();
    setFormError(null);
    setGlobalSuccess(null);

    if (!sessionToken) {
      setFormError("Missing session token. Please login first.");
      return;
    }
    if (!formState.title.trim() || !formState.slug.trim() || !formState.mdx_body.trim()) {
      setFormError("Title, slug and MDX body are required.");
      return;
    }

    setIsSubmittingForm(true);

    const payload = {
      title: formState.title.trim(),
      slug: formState.slug.trim(),
      excerpt: formState.excerpt.trim(),
      cover_image_url: formState.cover_image_url.trim(),
      tag: formState.tag.trim(),
      read_time: formState.read_time.trim(),
      author_name: formState.author_name.trim(),
      author_avatar_url: formState.author_avatar_url.trim(),
      author_icon: formState.author_icon.trim(),
      mdx_body: formState.mdx_body,
      status: formState.status,
    };

    try {
      const targetSlug = mode === "edit" ? editingSlug : null;
      const endpoint = targetSlug
        ? `/api/admin/articles/${encodeURIComponent(targetSlug)}`
        : "/api/admin/articles";
      const method = targetSlug ? "PUT" : "POST";

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
          setFormError(body.error ?? "Unauthorized");
          return;
        }
        throw new Error(body.error ?? "Failed to save article");
      }

      setGlobalSuccess(targetSlug ? "Article updated." : "Draft article created.");
      resetForm();
      await loadArticles();
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to save article");
    } finally {
      setIsSubmittingForm(false);
    }
  };

  const handleEdit = async (slug: string) => {
    setGlobalSuccess(null);
    setFormError(null);
    setIsLoadingDetail(true);

    try {
      const response = await fetch(`/api/admin/articles/${encodeURIComponent(slug)}`, {
        method: "GET",
        headers: {
          ...buildHeaders(),
        },
        cache: "no-store",
      });

      const payload = (await response.json()) as ArticleDetailResponse;
      if (!response.ok || !payload.article) {
        if (handleAuthFailure(response.status, payload.error)) {
          setFormError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to load article detail");
      }

      setAuthBlocked(null);
      setMode("edit");
      setEditingSlug(slug);
      setFormState({
        title: payload.article.title ?? "",
        slug: payload.article.slug ?? "",
        excerpt: payload.article.excerpt ?? "",
        cover_image_url: payload.article.cover_image_url ?? "",
        tag: payload.article.tag ?? "",
        read_time: payload.article.read_time ?? "",
        author_name: payload.article.author_name ?? "",
        author_avatar_url: payload.article.author_avatar_url ?? "",
        author_icon: payload.article.author_icon ?? "",
        mdx_body: payload.article.mdx_body ?? "",
        status: payload.article.status === "published" ? "published" : "draft",
      });
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Failed to load article detail");
    } finally {
      setIsLoadingDetail(false);
    }
  };

  const handleTogglePublish = async (article: ArticleSummary) => {
    const shouldPublish = article.status !== "published";
    const confirmed = window.confirm(
      shouldPublish
        ? `Publish article \"${article.title}\" now?`
        : `Unpublish article \"${article.title}\" now?`,
    );
    if (!confirmed) {
      return;
    }

    setRowLoadingSlug(article.slug);
    setGlobalSuccess(null);
    setListError(null);

    try {
      const path = shouldPublish ? "publish" : "unpublish";
      const response = await fetch(`/api/admin/articles/${encodeURIComponent(article.slug)}/${path}`, {
        method: "POST",
        headers: {
          ...buildHeaders(),
        },
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setListError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to update publication status");
      }

      setGlobalSuccess(shouldPublish ? "Article published." : "Article unpublished.");
      await loadArticles();
    } catch (error) {
      setListError(
        error instanceof Error ? error.message : "Failed to update publication status",
      );
    } finally {
      setRowLoadingSlug(null);
    }
  };

  const handleDelete = async (article: ArticleSummary) => {
    const confirmed = window.confirm(
      `Delete article \"${article.title}\"? This action cannot be undone.`,
    );
    if (!confirmed) {
      return;
    }

    setRowLoadingSlug(article.slug);
    setGlobalSuccess(null);
    setListError(null);

    try {
      const response = await fetch(`/api/admin/articles/${encodeURIComponent(article.slug)}`, {
        method: "DELETE",
        headers: {
          ...buildHeaders(),
        },
      });

      const payload = (await response.json()) as { error?: string };
      if (!response.ok) {
        if (handleAuthFailure(response.status, payload.error)) {
          setListError(payload.error ?? "Unauthorized");
          return;
        }
        throw new Error(payload.error ?? "Failed to delete article");
      }

      setGlobalSuccess("Article deleted.");
      if (editingSlug === article.slug) {
        resetForm();
      }
      await loadArticles();
    } catch (error) {
      setListError(error instanceof Error ? error.message : "Failed to delete article");
    } finally {
      setRowLoadingSlug(null);
    }
  };

  const isBlocked = Boolean(authBlocked);

  return (
    <section className="space-y-6">
      <div className="clay-panel space-y-2 p-5">
        <h1 className="section-title">
          <span className="gradient-text">Admin Articles</span>
        </h1>
        <p className="section-subtitle">
          Manage draft/published MDX articles with secure session-token authorization.
        </p>
      </div>

      <div className="block-card space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Session & Access</h2>
          <button className="btn-ghost" type="button" onClick={() => void loadArticles()}>
            Refresh list
          </button>
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
        {listError ? (
          <div
            className="rounded-xl border border-amber-400/45 dark:border-amber-400/60 bg-amber-500/10 dark:bg-amber-500/20 p-3 text-sm text-amber-700 dark:text-amber-300"
            role="alert"
          >
            {listError}
          </div>
        ) : null}
      </div>

      <div className="block-card space-y-4">
        <div className="flex items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">Article List</h2>
          <span className="text-xs text-[var(--portal-muted)]">
            {isLoadingList ? "Loading..." : `${articles.length} item(s)`}
          </span>
        </div>

        {isLoadingList ? (
          <p className="text-sm text-[var(--portal-muted)]">Loading articles...</p>
        ) : articles.length === 0 ? (
          <p className="text-sm text-[var(--portal-muted)]">No articles found.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full border-separate border-spacing-y-2 text-sm">
              <thead>
                <tr className="text-left text-[var(--portal-muted)]">
                  <th className="px-2 py-1">Title</th>
                  <th className="px-2 py-1">Slug</th>
                  <th className="px-2 py-1">Status</th>
                  <th className="px-2 py-1">Published</th>
                  <th className="px-2 py-1">Actions</th>
                </tr>
              </thead>
              <tbody>
                {articles.map((article) => {
                  const isRowBusy = rowLoadingSlug === article.slug;
                  const isPublished = article.status === "published";
                  return (
                    <tr key={article.slug} className="rounded-lg bg-[var(--portal-clay)]">
                      <td className="px-2 py-2 font-medium text-[var(--portal-ink)]">{article.title}</td>
                      <td className="px-2 py-2 font-mono text-xs text-[var(--portal-muted)]">{article.slug}</td>
                      <td className="px-2 py-2">
                        <span
                          className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${
                            isPublished
                              ? "bg-emerald-500/20 dark:bg-emerald-500/30 text-emerald-700 dark:text-emerald-300"
                              : "bg-[var(--stitch-text-muted)]/20 text-[var(--stitch-text-muted)]"
                          }`}
                        >
                          {article.status}
                        </span>
                      </td>
                      <td className="px-2 py-2 text-xs text-[var(--portal-muted)]">
                        {formatDateTime(article.published_at)}
                      </td>
                      <td className="px-2 py-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => void handleEdit(article.slug)}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            className="btn-ghost cursor-pointer px-3 py-1.5 text-xs"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => void handleTogglePublish(article)}
                          >
                            {isPublished ? "Unpublish" : "Publish"}
                          </button>
                          <button
                            type="button"
                            className="cursor-pointer rounded-xl border border-red-400/40 dark:border-red-400/60 bg-red-500/10 dark:bg-red-500/20 px-3 py-1.5 text-xs font-semibold text-red-700 dark:text-red-300"
                            disabled={isBlocked || isRowBusy}
                            onClick={() => void handleDelete(article)}
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

      <div className="block-card space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-lg font-semibold text-[var(--portal-ink)]">
            {mode === "create" ? "Create Article" : `Edit Article (${editingSlug})`}
          </h2>
          {mode === "edit" ? (
            <button className="btn-ghost" type="button" onClick={resetForm}>
              Switch to create
            </button>
          ) : null}
        </div>

        {isLoadingDetail ? (
          <p className="text-sm text-[var(--portal-muted)]">Loading article detail...</p>
        ) : null}
        {formError ? (
          <div
            className="rounded-xl border border-amber-400/45 dark:border-amber-400/60 bg-amber-500/10 dark:bg-amber-500/20 p-3 text-sm text-amber-700 dark:text-amber-300"
            role="alert"
          >
            {formError}
          </div>
        ) : null}

        <form className="grid gap-3" onSubmit={handleCreateOrUpdate}>
          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>Title</span>
            <input
              className="field"
              type="text"
              value={formState.title}
              onChange={(event) => handleFormChange("title", event.target.value)}
              disabled={isBlocked || isSubmittingForm}
              required
            />
          </label>
          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>Slug</span>
            <input
              className="field"
              type="text"
              value={formState.slug}
              onChange={(event) => handleFormChange("slug", slugifyTitle(event.target.value))}
              disabled={isBlocked || isSubmittingForm}
              required
            />
          </label>
          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>Excerpt</span>
            <textarea
              className="field min-h-20 resize-y"
              value={formState.excerpt}
              onChange={(event) => handleFormChange("excerpt", event.target.value)}
              disabled={isBlocked || isSubmittingForm}
            />
          </label>

          <div className="grid gap-3 md:grid-cols-2">
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Tag</span>
              <input
                className="field"
                type="text"
                value={formState.tag}
                onChange={(event) => handleFormChange("tag", event.target.value)}
                disabled={isBlocked || isSubmittingForm}
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Read time</span>
              <input
                className="field"
                type="text"
                value={formState.read_time}
                onChange={(event) => handleFormChange("read_time", event.target.value)}
                disabled={isBlocked || isSubmittingForm}
              />
            </label>
          </div>

          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>Cover image URL</span>
            <input
              className="field"
              type="url"
              value={formState.cover_image_url}
              onChange={(event) => handleFormChange("cover_image_url", event.target.value)}
              disabled={isBlocked || isSubmittingForm}
            />
          </label>

          <div className="grid gap-3 md:grid-cols-3">
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Author name</span>
              <input
                className="field"
                type="text"
                value={formState.author_name}
                onChange={(event) => handleFormChange("author_name", event.target.value)}
                disabled={isBlocked || isSubmittingForm}
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Author avatar URL</span>
              <input
                className="field"
                type="url"
                value={formState.author_avatar_url}
                onChange={(event) => handleFormChange("author_avatar_url", event.target.value)}
                disabled={isBlocked || isSubmittingForm}
              />
            </label>
            <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
              <span>Author icon</span>
              <input
                className="field"
                type="text"
                value={formState.author_icon}
                onChange={(event) => handleFormChange("author_icon", event.target.value)}
                disabled={isBlocked || isSubmittingForm}
              />
            </label>
          </div>

          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>Status</span>
            <select
              className="field"
              value={formState.status}
              onChange={(event) => handleFormChange("status", event.target.value)}
              disabled={isBlocked || isSubmittingForm}
            >
              <option value="draft">draft</option>
              <option value="published">published</option>
            </select>
          </label>

          <label className="grid gap-1 text-sm text-[var(--portal-muted)]">
            <span>MDX body</span>
            <textarea
              className="field min-h-72 resize-y font-mono"
              value={formState.mdx_body}
              onChange={(event) => handleFormChange("mdx_body", event.target.value)}
              disabled={isBlocked || isSubmittingForm}
              required
            />
          </label>

          <div className="flex flex-wrap items-center gap-2">
            <button className="btn-primary" type="submit" disabled={isBlocked || isSubmittingForm}>
              {isSubmittingForm
                ? "Saving..."
                : mode === "create"
                  ? "Create draft"
                  : "Save changes"}
            </button>
            <button
              className="btn-ghost"
              type="button"
              disabled={isSubmittingForm}
              onClick={resetForm}
            >
              Reset
            </button>
          </div>
        </form>
      </div>
    </section>
  );
}
