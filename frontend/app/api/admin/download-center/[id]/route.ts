import { NextResponse } from "next/server";

import { getApiBaseUrl } from "@/lib/server/api-base-url";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const upstream = await fetch(`${apiBaseUrl}/admin/download-center/${id}`, {
    method: "GET",
    headers: {
      "content-type": request.headers.get("content-type") ?? "application/json",
      accept: request.headers.get("accept") ?? "application/json",
      Authorization: request.headers.get("Authorization") ?? "",
    },
    cache: "no-store",
  });

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

export async function PUT(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
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

  const upstream = await fetch(`${apiBaseUrl}/admin/download-center/${id}`, {
    method: "PUT",
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
    return NextResponse.json(payload, { status: upstream.status });
  } catch {
    return NextResponse.json(
      { error: "invalid json response from upstream" },
      { status: 502 },
    );
  }
}

export async function DELETE(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;
  let apiBaseUrl: string;
  try {
    apiBaseUrl = getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const upstream = await fetch(`${apiBaseUrl}/admin/download-center/${id}`, {
    method: "DELETE",
    headers: {
      "content-type": request.headers.get("content-type") ?? "application/json",
      accept: request.headers.get("accept") ?? "application/json",
      Authorization: request.headers.get("Authorization") ?? "",
    },
    cache: "no-store",
  });

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
