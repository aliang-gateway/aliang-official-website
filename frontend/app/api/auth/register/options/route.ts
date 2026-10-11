import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// 注册选项（公开）：暴露运行时开关给前端渲染对应控件。
export async function GET() {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const upstream = await fetch(`${apiBaseUrl}/auth/register/options`, {
    method: "GET",
    headers: { accept: "application/json" },
    cache: "no-store",
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: upstream.headers,
  });
}
