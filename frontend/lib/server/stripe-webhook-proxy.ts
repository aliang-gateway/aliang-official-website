import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

export async function proxyStripeWebhook(request: Request) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const body = await request.arrayBuffer();
  const headers = new Headers();
  headers.set("content-type", request.headers.get("content-type") ?? "application/json");
  headers.set("accept", request.headers.get("accept") ?? "application/json");

  const stripeSignature = request.headers.get("stripe-signature");
  if (stripeSignature) {
    headers.set("stripe-signature", stripeSignature);
  }

  const upstream = await fetch(`${apiBaseUrl}/webhooks/stripe`, {
    method: "POST",
    headers,
    body,
    cache: "no-store",
  });

  const responseBody = await upstream.arrayBuffer();
  return new Response(responseBody, {
    status: upstream.status,
    headers: {
      "content-type": upstream.headers.get("content-type") ?? "application/json",
    },
  });
}
