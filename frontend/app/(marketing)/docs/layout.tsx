import type { ReactNode } from "react";
import { getApiBaseUrl } from "@/lib/server/api-base-url";
import { DocsSidebar } from "./components/DocsSidebar";

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

type DocsLayoutProps = {
  children: ReactNode;
};

export default async function DocsLayout({ children }: DocsLayoutProps) {
  const categories = await getDocsCategories();

  return (
    <div className="container">
      <div className="docs-layout">
        <DocsSidebar categories={categories} />
        <div className="docs-main">{children}</div>
      </div>
    </div>
  );
}
