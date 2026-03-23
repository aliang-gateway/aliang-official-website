import { NextResponse } from "next/server";

import { getApiBaseUrl, makeTicketSubmissionResult } from "../_shared";

type DashboardTicketRequest = {
  title?: unknown;
  category?: unknown;
  message?: unknown;
};

export async function POST(request: Request) {
  try {
    getApiBaseUrl();
  } catch (error) {
    return NextResponse.json(
      { error: error instanceof Error ? error.message : "server misconfiguration" },
      { status: 500 },
    );
  }

  const authorization = request.headers.get("Authorization")?.trim() ?? "";
  if (!authorization) {
    return NextResponse.json(
      { error: "authorization header is required" },
      { status: 401 },
    );
  }

  let payload: DashboardTicketRequest;
  try {
    payload = (await request.json()) as DashboardTicketRequest;
  } catch {
    return NextResponse.json({ error: "invalid json body" }, { status: 400 });
  }

  const title = typeof payload.title === "string" ? payload.title.trim() : "";
  const category = typeof payload.category === "string" ? payload.category.trim() : "";
  const message = typeof payload.message === "string" ? payload.message.trim() : "";

  if (!title) {
    return NextResponse.json({ error: "title is required" }, { status: 400 });
  }
  if (!category) {
    return NextResponse.json({ error: "category is required" }, { status: 400 });
  }
  if (!message) {
    return NextResponse.json({ error: "message is required" }, { status: 400 });
  }

  const result = makeTicketSubmissionResult(new Date().toISOString());
  return NextResponse.json(result, { status: 201 });
}
