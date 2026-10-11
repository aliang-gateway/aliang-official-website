import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// 注册邮箱验证码发码（portal 自有）。开关关闭时上游 404，前端隐藏入口。
export async function POST(request: Request) {
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
  headers.set("content-type", request.headers.get("content-type") ?? "application/json");
  headers.set("accept", "application/json");

  const upstream = await fetch(`${apiBaseUrl}/auth/register/send-code`, {
    method: "POST",
    headers,
    body: await request.arrayBuffer(),
    cache: "no-store",
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: upstream.headers,
  });
}
