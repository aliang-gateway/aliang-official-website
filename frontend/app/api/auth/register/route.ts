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

  const requestBody = await request.arrayBuffer();

  const headers = new Headers();
  const contentType = request.headers.get("content-type");
  const accept = request.headers.get("accept");
  const authorization = request.headers.get("authorization");
  headers.set("content-type", contentType ?? "application/json");
  headers.set("accept", accept ?? "application/json");
  if (authorization) {
    headers.set("authorization", authorization);
  }

  const upstream = await fetch(`${apiBaseUrl}/auth/register`, {
    method: "POST",
    headers,
    body: requestBody,
    cache: "no-store",
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: upstream.headers,
  });
}
