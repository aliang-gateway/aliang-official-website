import { NextResponse } from "next/server";

const DEFAULT_PROGRESS = 15;

function clampProgress(value: number): number {
  if (!Number.isFinite(value)) return DEFAULT_PROGRESS;
  return Math.min(Math.max(Math.round(value), 0), 100);
}

export async function GET() {
  const raw = process.env.OPENSOURCE_PROGRESS ?? process.env.NEXT_PUBLIC_OPENSOURCE_PROGRESS;
  const progress = raw ? Number(raw) : DEFAULT_PROGRESS;
  return NextResponse.json({ progress: clampProgress(progress) });
}


