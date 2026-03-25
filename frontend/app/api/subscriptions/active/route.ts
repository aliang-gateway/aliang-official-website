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

  const upstream = await fetch(`${apiBaseUrl}/subscriptions/active`, {
    method: "GET",
    headers: {
      "content-type": request.headers.get("content-type") ?? "application/json",
      accept: request.headers.get("accept") ?? "application/json",
      Authorization: request.headers.get("Authorization") ?? "",
    },
    cache: "no-store",
  });

  return new Response(upstream.body, {
    status: upstream.status,
    headers: upstream.headers,
  });
}
