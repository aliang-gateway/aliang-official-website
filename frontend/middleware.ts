import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

const LEGACY_ID_TO_SLUG: Record<string, string> = {
  "1": "gpt4o-vs-claude35",
  "2": "cursor-tips",
  "3": "deepseek-vs-qwen",
  "4": "deepseek-vs-qwen-2",
  "5": "api-key-security",
};

export function middleware(request: NextRequest) {
  const { pathname } = request.nextUrl;
  const match = pathname.match(/^\/blog\/(\d+)$/);
  if (!match) {
    return NextResponse.next();
  }

  const slug = LEGACY_ID_TO_SLUG[match[1]];
  if (!slug) {
    return NextResponse.next();
  }

  const url = request.nextUrl.clone();
  url.pathname = `/blog/${slug}`;
  return NextResponse.redirect(url, 308);
}

export const config = {
  matcher: "/blog/:id*",
};
