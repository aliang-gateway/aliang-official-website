import { notFound } from "next/navigation";
import { MDXRemote } from "next-mdx-remote/rsc";
import remarkGfm from "remark-gfm";
import rehypeHighlight from "rehype-highlight";
import { useMDXComponents as getMDXComponents } from "@/mdx-components";
import { getApiBaseUrl } from "@/lib/server/api-base-url";

type DocsPageDetail = {
  slug: string;
  title: string;
  mdx_body: string;
};

type DocsPageDetailResponse = {
  page?: DocsPageDetail;
  error?: string;
};

async function getDocsPageBySlug(slug: string): Promise<DocsPageDetail | null> {
  const response = await fetch(
    `${getApiBaseUrl()}/public/docs/pages/${encodeURIComponent(slug)}`,
    {
      method: "GET",
      headers: {
        "content-type": "application/json",
        accept: "application/json",
      },
      cache: "no-store",
    },
  );

  if (response.status === 404) {
    return null;
  }

  const payload = (await response.json()) as DocsPageDetailResponse;

  if (!response.ok || !payload.page) {
    throw new Error(payload.error ?? "Failed to load docs page");
  }

  return payload.page;
}

export default async function DocsSlugPage({
  params,
}: {
  params: Promise<{ slug: string }>;
}) {
  const { slug } = await params;
  const page = await getDocsPageBySlug(slug);
  const mdxComponents = getMDXComponents({});

  if (!page) {
    notFound();
  }

  return (
    <article className="prose">
      <h1>{page.title}</h1>
      <MDXRemote
        source={page.mdx_body}
        components={mdxComponents}
        options={{
          mdxOptions: {
            remarkPlugins: [remarkGfm],
            rehypePlugins: [rehypeHighlight],
          },
        }}
      />
    </article>
  );
}
