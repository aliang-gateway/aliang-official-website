import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

// Issues a slider challenge for the registration form. 404 from the backend
// means the slider captcha is disabled — the page renders without it.
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

  const upstream = await fetch(`${apiBaseUrl}/auth/register-captcha`, {
    method: "GET",
    headers: { accept: "application/json" },
    cache: "no-store",
  });
  return new Response(upstream.body, {
    status: upstream.status,
    headers: { "content-type": upstream.headers.get("content-type") ?? "application/json" },
  });
}
