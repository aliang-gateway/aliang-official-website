import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// 透传后端 GET /wallet/transactions(用户钱包流水,按 id 倒序)。
// 与 redeem 同款 BFF 直通:上游状态码原样返回,limit 由查询串控制。
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
  let upstream: Response;
  try {
    upstream = await fetch(`${apiBaseUrl}/wallet/transactions${url.search}`, {
      method: "GET",
      headers: {
        "content-type": request.headers.get("content-type") ?? "application/json",
        accept: request.headers.get("accept") ?? "application/json",
        Authorization: request.headers.get("Authorization") ?? "",
      },
      cache: "no-store",
    });
  } catch {
    return NextResponse.json({ error: "upstream request failed" }, { status: 502 });
  }

  try {
    const payload = await upstream.json();
    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json({ error: "invalid json response from upstream" }, { status: 502 });
  }
}
