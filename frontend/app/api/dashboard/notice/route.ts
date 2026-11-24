import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// Dashboard 公告：GET 读当前公告+确认状态；POST 记录当前用户已确认。
// 都是登录态端点，必须透传 Authorization。
async function proxy(request: Request, method: "GET" | "POST") {
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
  headers.set("accept", "application/json");
  const authorization = request.headers.get("authorization");
  if (authorization) {
    headers.set("authorization", authorization);
  }

  const url = new URL(request.url);
  const upstream = await fetch(`${apiBaseUrl}/dashboard/notice${url.search}`, {
    method,
    headers,
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

export async function POST(request: Request) {
  return proxy(request, "POST");
}
