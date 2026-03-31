import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

export async function POST(
  request: Request,
  context: { params: Promise<{ slug: string }> },
) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const { slug } = await context.params;

  const upstream = await fetch(
    `${apiBaseUrl}/admin/articles/${encodeURIComponent(slug)}/unpublish`,
    {
      method: "POST",
      headers: {
        "content-type": request.headers.get("content-type") ?? "application/json",
        accept: request.headers.get("accept") ?? "application/json",
        Authorization: request.headers.get("Authorization") ?? "",
      },
      cache: "no-store",
    },
  );

  try {
    const payload = await upstream.json();
    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json(
      { error: "invalid json response from upstream" },
      { status: 502 },
    );
  }
}
