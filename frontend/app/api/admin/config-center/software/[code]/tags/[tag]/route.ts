import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

type RouteContext = { params: Promise<{ code: string; tag: string }> };

export async function DELETE(request: Request, context: RouteContext) {
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const { code, tag } = await context.params;
  const upstream = await fetch(
    `${apiBaseUrl}/admin/config-center/software/${encodeURIComponent(code)}/tags/${encodeURIComponent(tag)}`,
    {
      method: "DELETE",
      headers: {
        "content-type": request.headers.get("content-type") ?? "application/json",
        accept: request.headers.get("accept") ?? "application/json",
        Authorization: request.headers.get("Authorization") ?? "",
      },
      cache: "no-store",
    },
  );

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
