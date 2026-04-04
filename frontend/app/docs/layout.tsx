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
    <section className="bg-[var(--stitch-bg)] text-[var(--stitch-text)]">
      <div className="mx-auto w-full max-w-6xl px-6 py-10 md:px-10 lg:px-12">
        <div className="flex flex-col gap-8 lg:flex-row lg:gap-12">
          <DocsSidebar categories={categories} />
          <article className="min-w-0 flex-1 pb-8">{children}</article>
        </div>
      </div>
    </section>
  );
}
