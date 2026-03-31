import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

export async function GET(request: Request) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const url = new URL(request.url);
  const upstreamURL = new URL(`${apiBaseUrl}/admin/payments`);
  const limit = url.searchParams.get("limit")?.trim() ?? "";
  if (limit) {
    upstreamURL.searchParams.set("limit", limit);
  }

  const upstream = await fetch(upstreamURL.toString(), {
    method: "GET",
    headers: {
      accept: request.headers.get("accept") ?? "application/json",
      Authorization: request.headers.get("authorization") ?? "",
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
