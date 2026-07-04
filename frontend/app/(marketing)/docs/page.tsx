import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { getApiBaseUrl } from "@/lib/server/api-base-url";

type DocsCategoryPage = {
  slug: string;
  title: string;
};

type DocsCategory = {
  slug: string;
  title: string;
  description?: string;
  pages: DocsCategoryPage[];
};

type DocsCategoriesResponse = {
  categories: DocsCategory[];
};

async function getDocsCategories(): Promise<DocsCategory[]> {
  try {
    const response = await fetch(
      `${getApiBaseUrl()}/public/docs/categories`,
      {
        method: "GET",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
        },
        cache: "no-store",
      },
    );

    if (!response.ok) {
      return [];
    }

    const payload = (await response.json()) as DocsCategoriesResponse;
    return payload.categories ?? [];
  } catch {
    return [];
  }
}

export default async function DocsIndexPage() {
  const categories = await getDocsCategories();
  const t = await getTranslations("editorial.docs");

  const firstPage = categories
    .flatMap((cat) => cat.pages)
    .find((page) => page.slug);

  if (firstPage) {
    redirect(`/docs/${firstPage.slug}`);
  }

  return (
    <div className="docs-empty">
      <span className="icon-chip" aria-hidden="true">
        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6">
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
          />
        </svg>
      </span>
      <h1 className="display">
        {t("comingSoonTitle")}
        <span className="dot">.</span>
      </h1>
      <p className="lead">{t("comingSoonLead")}</p>
    </div>
  );
}
