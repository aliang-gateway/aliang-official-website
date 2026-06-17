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
  const headers = new Headers();
  const contentType = request.headers.get("content-type");
  const accept = request.headers.get("accept");
  headers.set("content-type", contentType ?? "application/json");
  headers.set("accept", accept ?? "application/json");
  const requestBody = await request.arrayBuffer();
  const upstream = await fetch(`${apiBaseUrl}/auth/scan/init`, {
    method: "POST",
    headers,
    body: requestBody,
    cache: "no-store",
  });
  return new Response(upstream.body, { status: upstream.status, headers: upstream.headers });
}
