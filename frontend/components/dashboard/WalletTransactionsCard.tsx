"use client";

import { useEffect, useState } from "react";
import { useTranslations } from "next-intl";

import { parseWalletTransactions } from "@/lib/dashboard-home";
import type { WalletTransactionItem } from "@/lib/dashboard-types";

const TRANSACTIONS_LIMIT = 8;

/** 非法货币码会让 Intl.NumberFormat 抛 RangeError,先收敛到合法形态。 */
function formatAmount(value: number, currency: string) {
  const code = /^[A-Za-z]{3}$/.test(currency) ? currency.toUpperCase() : "CNY";
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: code,
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Math.abs(value));
}

function formatTransactionTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "--";
  }
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

/** 首页余额变动卡:最近 8 笔钱包流水 + 每笔后余额。卡片自取数,
 *  失败静默降级为空态,不影响页面其他部分(与 CacheRateCard 同策略)。
 *  金额符号/颜色按 amount 正负渲染,不按 tx_type 硬编码——后端将来
 *  新增消费类型时 UI 自动适配;未知 type 显示原始文本。 */
export function WalletTransactionsCard({ sessionToken }: { sessionToken: string }) {
  const t = useTranslations("dashboard");
  const [transactions, setTransactions] = useState<WalletTransactionItem[] | null>(null);

  useEffect(() => {
    if (!sessionToken) {
      return;
    }
    let cancelled = false;
    const controller = new AbortController();

    const load = async () => {
      try {
        const response = await fetch(`/api/wallet/transactions?limit=${TRANSACTIONS_LIMIT}`, {
          method: "GET",
          headers: {
            "content-type": "application/json",
            accept: "application/json",
            Authorization: `Bearer ${sessionToken}`,
          },
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) {
          return;
        }
        const payload = (await response.json()) as unknown;
        if (!cancelled) {
          setTransactions(parseWalletTransactions(payload));
        }
      } catch (fetchError) {
        if ((fetchError as Error).name === "AbortError") {
          return;
        }
        // 静默:流水取不到就维持空态,不打断首屏。
      }
    };

    void load();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [sessionToken]);

  const list = transactions ?? [];

  return (
    <article className="block-card min-w-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="text-2xl font-bold text-[var(--portal-ink)]">{t("walletTransactions")}</h2>
          <p className="mt-2 text-sm text-[var(--portal-muted)]">{t("walletTransactionsDescription")}</p>
        </div>
        <div className="shrink-0 rounded-full border border-[var(--portal-line)] bg-[var(--portal-clay)] px-3 py-1 text-xs font-semibold text-[var(--portal-muted)]">
          {t("transactionCount", { count: list.length })}
        </div>
      </div>

      {transactions === null ? (
        <div className="mt-4 space-y-2" aria-hidden="true">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-10 animate-pulse rounded-[1rem] bg-[var(--portal-clay)]" />
          ))}
        </div>
      ) : list.length === 0 ? (
        <p className="mt-4 rounded-[1rem] border border-dashed border-[var(--portal-line)] bg-[var(--portal-clay)] p-5 text-sm text-[var(--portal-muted)]">
          {t("walletTransactionsEmpty")}
        </p>
      ) : (
        <ol className="mt-4 divide-y divide-[var(--portal-line)] rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)]">
          {list.map((txn) => {
            const positive = txn.amount > 0;
            const negative = txn.amount < 0;
            return (
              <li key={txn.id} className="flex items-center gap-3 px-4 py-3">
                <span className="shrink-0 rounded-full border border-[var(--portal-line)] bg-[var(--bone)]/55 px-2.5 py-0.5 text-xs font-bold text-[var(--portal-muted)]">
                  {txn.type === "recharge" ? t("transactionRecharge") : txn.type}
                </span>
                <span
                  className={`shrink-0 font-semibold tabular-nums ${
                    positive ? "text-emerald-600 dark:text-emerald-300" : negative ? "text-red-600 dark:text-red-300" : "text-[var(--portal-muted)]"
                  }`}
                >
                  {positive ? "+" : negative ? "−" : ""}
                  {formatAmount(txn.amount, txn.currency)}
                </span>
                <span className="min-w-0 flex-1 truncate text-right text-xs text-[var(--portal-muted)]">
                  {t("balance")}: {formatAmount(txn.balance_after, txn.currency)} · {formatTransactionTime(txn.created_at)}
                </span>
              </li>
            );
          })}
        </ol>
      )}
    </article>
  );
}
