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
  const serviceItemCode = url.searchParams.get("service_item_code")?.trim() ?? "";
  if (!serviceItemCode) {
    return NextResponse.json({ error: "service_item_code is required" }, { status: 400 });
  }

  const upstreamUrl = new URL(`${apiBaseUrl}/admin/unit-prices`);
  upstreamUrl.searchParams.set("service_item_code", serviceItemCode);

  const tierCode = url.searchParams.get("tier_code")?.trim() ?? "";
  if (tierCode) {
    upstreamUrl.searchParams.set("tier_code", tierCode);
  }

  const upstream = await fetch(upstreamUrl.toString(), {
    method: "GET",
    headers: {
      "content-type": request.headers.get("content-type") ?? "application/json",
      accept: request.headers.get("accept") ?? "application/json",
      Authorization: request.headers.get("Authorization") ?? "",
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
