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
  const qs = request.url.includes("?") ? request.url.slice(request.url.indexOf("?")) : "";
  const upstream = await fetch(`${apiBaseUrl}/auth/scan/status${qs}`, {
    method: "GET",
    headers: { accept: "application/json" },
    cache: "no-store",
  });
  return new Response(upstream.body, { status: upstream.status, headers: upstream.headers });
}
