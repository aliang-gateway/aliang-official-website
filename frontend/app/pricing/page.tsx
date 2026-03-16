"use client";

import { useState } from "react";

const pricingTiers = [
  {
    code: "free",
    name: "Free",
    price: "$0",
    period: "/month",
    description: "Perfect for getting started and exploring our API",
    features: [
      "1,000 API calls/month",
      "Basic models access",
      "Community support",
      "Rate limit: 10 req/min",
    ],
    cta: "Get Started",
    popular: false,
  },
  {
    code: "pro",
    name: "Pro",
    price: "$20",
    period: "/month",
    description: "For developers building production applications",
    features: [
      "100,000 API calls/month",
      "All models access",
      "Priority support",
      "Rate limit: 100 req/min",
      "Advanced analytics",
    ],
    cta: "Start Free Trial",
    popular: true,
  },
  {
    code: "enterprise",
    name: "Enterprise",
    price: "Custom",
    period: "",
    description: "For organizations with custom needs",
    features: [
      "Unlimited API calls",
      "Dedicated infrastructure",
      "24/7 premium support",
      "Custom rate limits",
      "SLA guarantee",
      "Custom integrations",
    ],
    cta: "Contact Sales",
    popular: false,
  },
];

const usagePricing = [
  { model: "GPT-4 Turbo", input: "$0.01", output: "$0.03", unit: "per 1K tokens" },
  { model: "GPT-4", input: "$0.03", output: "$0.06", unit: "per 1K tokens" },
  { model: "GPT-3.5 Turbo", input: "$0.001", output: "$0.002", unit: "per 1K tokens" },
  { model: "Embeddings", input: "$0.0001", output: "-", unit: "per 1K tokens" },
];

const faqItems = [
  {
    q: "How does usage-based billing work?",
    a: "You're charged based on the number of tokens processed. Input tokens and output tokens are priced separately.",
  },
  {
    q: "Can I switch plans anytime?",
    a: "Yes, you can upgrade or downgrade your plan at any time. Changes take effect immediately.",
  },
  {
    q: "Is there a free trial?",
    a: "New users get $5 in free credits upon signup. No credit card required.",
  },
  {
    q: "What payment methods do you accept?",
    a: "We accept all major credit cards, PayPal, and wire transfers for enterprise customers.",
  },
];

export default function PricingPage() {
  const [billingCycle, setBillingCycle] = useState<"monthly" | "yearly">("monthly");

  return (
    <div className="space-y-16">
      <section className="text-center space-y-4">
        <p className="text-emerald-400 text-sm font-medium tracking-wide uppercase">Pricing</p>
        <h1 className="text-4xl md:text-5xl font-bold">
          <span className="gradient-text">Simple, transparent pricing</span>
        </h1>
        <p className="text-gray-400 text-lg max-w-2xl mx-auto">
          Choose the plan that fits your needs. All plans include access to our API with no hidden fees.
        </p>
      </section>

      <section className="flex justify-center">
        <div className="inline-flex items-center gap-2 bg-black/40 rounded-full p-1">
          <button
            type="button"
            onClick={() => setBillingCycle("monthly")}
            className={`px-4 py-2 rounded-full text-sm font-medium transition-colors ${
              billingCycle === "monthly"
                ? "bg-emerald-500 text-black"
                : "text-gray-400 hover:text-white"
            }`}
          >
            Monthly
          </button>
          <button
            type="button"
            onClick={() => setBillingCycle("yearly")}
            className={`px-4 py-2 rounded-full text-sm font-medium transition-colors ${
              billingCycle === "yearly"
                ? "bg-emerald-500 text-black"
                : "text-gray-400 hover:text-white"
            }`}
          >
            Yearly <span className="text-xs text-emerald-300">-20%</span>
          </button>
        </div>
      </section>

      <section className="grid md:grid-cols-3 gap-6 max-w-6xl mx-auto">
        {pricingTiers.map((tier) => (
          <div
            key={tier.code}
            className={`relative rounded-2xl p-6 transition-all ${
              tier.popular
                ? "bg-gradient-to-b from-emerald-900/30 to-black/60 border border-emerald-500/50 scale-105"
                : "bg-black/40 border border-white/10 hover:border-emerald-500/30"
            }`}
          >
            {tier.popular && (
              <div className="absolute -top-3 left-1/2 -translate-x-1/2">
                <span className="bg-emerald-500 text-black text-xs font-bold px-3 py-1 rounded-full">
                  MOST POPULAR
                </span>
              </div>
            )}
            <div className="space-y-4">
              <h3 className="text-xl font-bold text-white">{tier.name}</h3>
              <div className="flex items-baseline gap-1">
                <span className="text-4xl font-bold text-white">
                  {tier.price === "$0"
                    ? "$0"
                    : tier.price === "Custom"
                    ? "Custom"
                    : billingCycle === "yearly"
                    ? `$${Math.round(parseInt(tier.price.slice(1)) * 0.8)}`
                    : tier.price}
                </span>
                <span className="text-gray-400">{tier.period}</span>
              </div>
              <p className="text-gray-400 text-sm">{tier.description}</p>
              <ul className="space-y-3 pt-4">
                {tier.features.map((feature) => (
                  <li key={feature} className="flex items-start gap-2 text-sm text-gray-300">
                    <svg
                      className="w-5 h-5 text-emerald-400 flex-shrink-0 mt-0.5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      aria-hidden="true"
                    >
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                    </svg>
                    {feature}
                  </li>
                ))}
              </ul>
              <button
                type="button"
                className={`w-full py-3 rounded-lg font-medium transition-all mt-6 ${
                  tier.popular
                    ? "bg-emerald-500 text-black hover:bg-emerald-400"
                    : "bg-white/10 text-white hover:bg-white/20"
                }`}
              >
                {tier.cta}
              </button>
            </div>
          </div>
        ))}
      </section>

      <section className="max-w-4xl mx-auto space-y-8">
        <div className="text-center space-y-2">
          <h2 className="text-2xl font-bold text-white">Usage-Based Pricing</h2>
          <p className="text-gray-400">Pay only for what you use. No minimum commitments.</p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-white/10">
                <th className="text-left py-4 px-4 text-gray-400 font-medium">Model</th>
                <th className="text-right py-4 px-4 text-gray-400 font-medium">Input</th>
                <th className="text-right py-4 px-4 text-gray-400 font-medium">Output</th>
              </tr>
            </thead>
            <tbody>
              {usagePricing.map((item) => (
                <tr key={item.model} className="border-b border-white/5 hover:bg-white/5 transition-colors">
                  <td className="py-4 px-4 text-white font-medium">{item.model}</td>
                  <td className="py-4 px-4 text-right text-gray-300">{item.input}</td>
                  <td className="py-4 px-4 text-right text-gray-300">{item.output}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="max-w-3xl mx-auto space-y-6">
        <h2 className="text-2xl font-bold text-white text-center">Frequently Asked Questions</h2>
        <div className="space-y-4">
          {faqItems.map((faq) => (
            <div key={faq.q} className="bg-black/40 border border-white/10 rounded-xl p-5 space-y-2">
              <h3 className="text-white font-medium">{faq.q}</h3>
              <p className="text-gray-400 text-sm">{faq.a}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="text-center space-y-6 py-8">
        <h2 className="text-3xl font-bold text-white">Ready to get started?</h2>
        <p className="text-gray-400">Start building with our API today. No credit card required.</p>
        <div className="flex justify-center gap-4">
          <a
            href="/account"
            className="px-6 py-3 bg-emerald-500 text-black font-medium rounded-lg hover:bg-emerald-400 transition-colors"
          >
            Create Account
          </a>
          <a
            href="/download"
            className="px-6 py-3 bg-white/10 text-white font-medium rounded-lg hover:bg-white/20 transition-colors"
          >
            View Documentation
          </a>
        </div>
      </section>
    </div>
  );
}
