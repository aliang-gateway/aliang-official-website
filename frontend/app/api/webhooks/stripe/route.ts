import { proxyStripeWebhook } from "@/lib/server/stripe-webhook-proxy";

export const runtime = "nodejs";

export async function POST(request: Request) {
  return proxyStripeWebhook(request);
}
