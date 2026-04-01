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
  const platform = url.searchParams.get("platform") ?? "";
  const software = url.searchParams.get("software") ?? "";
  const version = url.searchParams.get("version") ?? "";

  let upstreamURL = `${apiBaseUrl}/public/downloads/check?platform=${encodeURIComponent(platform)}&version=${encodeURIComponent(version)}`;
  if (software) {
    upstreamURL += `&software=${encodeURIComponent(software)}`;
  }

  const upstream = await fetch(upstreamURL, {
    method: "GET",
    headers: {
      "content-type": "application/json",
      accept: "application/json",
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
