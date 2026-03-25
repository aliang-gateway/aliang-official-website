import { NextResponse } from "next/server";

function getApiBaseUrl() {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  if (!baseUrl) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return baseUrl.replace(/\/$/, "");
}

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

  const headers = new Headers();
  const contentType = request.headers.get("content-type");
  const accept = request.headers.get("accept");
  const authorization = request.headers.get("authorization");
  headers.set("content-type", contentType ?? "application/json");
  headers.set("accept", accept ?? "application/json");
  if (authorization) {
    headers.set("authorization", authorization);
  }

  const url = new URL(request.url);
  const upstream = await fetch(`${apiBaseUrl}/dashboard/models${url.search}`, {
    method: "GET",
    headers,
    cache: "no-store",
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: upstream.headers,
  });
}
