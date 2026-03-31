import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

type RouteContext = { params: Promise<{ id: string }> };

export async function POST(request: Request, context: RouteContext) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const { id } = await context.params;

  const upstream = await fetch(`${apiBaseUrl}/admin/fulfillment/jobs/${encodeURIComponent(id)}/replay`, {
    method: "POST",
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
