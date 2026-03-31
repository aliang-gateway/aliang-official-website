import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

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

  const requestBody = await request.text();

  let upstream: Response;
  try {
    upstream = await fetch(`${apiBaseUrl}/wallet/redeem`, {
      method: "POST",
      headers: {
        "content-type": request.headers.get("content-type") ?? "application/json",
        accept: request.headers.get("accept") ?? "application/json",
        Authorization: request.headers.get("Authorization") ?? "",
      },
      body: requestBody,
      cache: "no-store",
    });
  } catch {
    return NextResponse.json(
      { error: "upstream request failed" },
      { status: 502 },
    );
  }

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
