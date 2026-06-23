import { NextResponse } from "next/server";

/**
 * Lightweight liveness probe.
 *
 * Returns 200 immediately without contacting the Go backend, so it can be used
 * by load balancers / orchestrators (or any caller) that only need to know the
 * Next.js process is up and serving requests. Deliberately cheap: no upstream
 * fetch, no caching, no polling.
 */
export async function GET() {
  return NextResponse.json({ status: "ok" }, { status: 200 });
}
