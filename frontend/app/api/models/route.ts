import { NextResponse } from "next/server";

/**
 * Per-key model list. The frontend sends the user's own API key in the
 * `x-api-key` header; we forward it to the AI gateway's OpenAI-compatible
 * GET /v1/models, which returns the models that key can access (scoped to the
 * key's group → upstream accounts). Server-side, so no browser CORS.
 *
 * The gateway (api.aliang.one) is the AI inference host — distinct from the
 * portal backend (NEXT_PUBLIC_API_BASE_URL). Override with GATEWAY_BASE_URL.
 */
const GATEWAY_BASE_URL = process.env.GATEWAY_BASE_URL ?? "https://api.aliang.one";

export async function GET(request: Request) {
  const apiKey = request.headers.get("x-api-key");
  if (!apiKey) {
    return NextResponse.json({ error: "missing x-api-key header" }, { status: 400 });
  }

  try {
    const upstream = await fetch(`${GATEWAY_BASE_URL}/v1/models`, {
      method: "GET",
      headers: {
        accept: "application/json",
        Authorization: `Bearer ${apiKey}`,
      },
      cache: "no-store",
    });

    const text = await upstream.text();
    return new Response(text, {
      status: upstream.status,
      headers: { "content-type": upstream.headers.get("content-type") ?? "application/json" },
    });
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "gateway unreachable" },
      { status: 502 },
    );
  }
}
