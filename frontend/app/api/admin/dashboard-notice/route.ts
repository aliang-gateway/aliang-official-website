import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// Admin：读取/编辑 Dashboard 公告（单条当前公告，启用时标题、正文必填）。
async function proxy(request: Request, method: "GET" | "PUT") {
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
  headers.set("content-type", "application/json");
  headers.set("accept", "application/json");
  const authorization = request.headers.get("authorization");
  if (authorization) {
    headers.set("authorization", authorization);
  }

  const url = new URL(request.url);
  const upstream = await fetch(`${apiBaseUrl}/admin/dashboard-notice${url.search}`, {
    method,
    headers,
    body: method === "PUT" ? await request.arrayBuffer() : undefined,
    cache: "no-store",
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: upstream.headers,
  });
}

export async function GET(request: Request) {
  return proxy(request, "GET");
}

export async function PUT(request: Request) {
  return proxy(request, "PUT");
}
