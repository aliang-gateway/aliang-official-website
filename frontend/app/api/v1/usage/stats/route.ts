import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// The aliangVibeCodingPhone client calls this path verbatim
// (`GET /api/v1/usage/stats?period=today`). Next.js does NOT otherwise proxy
// `/api/v1/*` to the Go backend (it has no /api/v1/* routes and no rewrite), so
// without this route the request dies with a Next.js 404 before ever reaching
// the backend. We forward to the backend's existing root route `GET /usage/stats`
// (which itself proxies upstream to sub2api `/api/v1/usage/stats`).
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

  const headers = new Headers();
  const contentType = request.headers.get("content-type");
  const accept = request.headers.get("accept");
  const authorization = request.headers.get("authorization");
  headers.set("content-type", contentType ?? "application/json");
  headers.set("accept", accept ?? "application/json");
  if (authorization) {
    headers.set("authorization", authorization);
  }

  const url = new URL(request.url);
  const upstream = await fetch(`${apiBaseUrl}/usage/stats${url.search}`, {
    method: "GET",
    headers,
    cache: "no-store",
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: upstream.headers,
  });
}
