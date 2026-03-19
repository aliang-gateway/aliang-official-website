import { NextResponse } from "next/server";

function getApiBaseUrl() {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  if (!baseUrl) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return baseUrl.replace(/\/$/, "");
}

export async function GET(
  request: Request,
  { params }: { params: Promise<{ slug: string }> },
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

  const { slug } = await params;

  const upstream = await fetch(`${apiBaseUrl}/public/articles/${slug}`, {
    method: request.method,
    headers: {
      "content-type": request.headers.get("content-type") ?? "application/json",
      accept: request.headers.get("accept") ?? "application/json",
    },
    cache: "no-store",
  });

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
