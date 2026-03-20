import { NextResponse } from "next/server";

type RegisterSuccessPayload = {
  user_id: number;
  email: string;
  name: string;
  session_token: string;
  email_verified: boolean;
  require_email_verification?: boolean;
};

function isRegisterSuccessPayload(payload: unknown): payload is RegisterSuccessPayload {
  if (!payload || typeof payload !== "object") {
    return false;
  }
  const data = payload as Record<string, unknown>;
  return (
    typeof data.user_id === "number" &&
    typeof data.email === "string" &&
    typeof data.name === "string" &&
    typeof data.session_token === "string" &&
    typeof data.email_verified === "boolean"
  );
}

function getApiBaseUrl() {
  const baseUrl = process.env.NEXT_PUBLIC_API_BASE_URL?.trim();
  if (!baseUrl) {
    throw new Error("NEXT_PUBLIC_API_BASE_URL is not set");
  }
  return baseUrl.replace(/\/$/, "");
}

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

  const upstream = await fetch(`${apiBaseUrl}/auth/register`, {
    method: "POST",
    headers: {
      "content-type": request.headers.get("content-type") ?? "application/json",
      accept: request.headers.get("accept") ?? "application/json",
      Authorization: request.headers.get("Authorization") ?? "",
    },
    body: requestBody,
    cache: "no-store",
  });

  try {
    const payload = await upstream.json();
    if (upstream.ok && isRegisterSuccessPayload(payload)) {
      return NextResponse.json(
        {
          ...payload,
          require_email_verification: payload.require_email_verification ?? true,
        },
        { status: upstream.status },
      );
    }
    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json(
      { error: "invalid json response from upstream" },
      { status: 502 },
    );
  }
}
