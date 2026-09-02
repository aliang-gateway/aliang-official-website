"use client";

import { useTranslations } from "next-intl";

import { SelectMenu } from "@/components/ui/SelectMenu";
import { useTicketForm } from "@/lib/hooks/use-ticket-form";

type TicketCardProps = {
  sessionToken: string;
  /** Render without the outer block-card wrapper (for use inside a modal). */
  bare?: boolean;
};

export function TicketCard({ sessionToken, bare = false }: TicketCardProps) {
  const t = useTranslations("dashboard");
  const {
    title,
    setTitle,
    category,
    setCategory,
    message,
    setMessage,
    submitting,
    submitMessage,
    handleSubmit,
  } = useTicketForm({ sessionToken });

  const ticketMessageClassName =
    submitMessage?.tone === "error"
      ? "text-red-500 dark:text-red-400"
      : submitMessage?.tone === "success"
        ? "text-[var(--accent)]"
        : "text-[var(--portal-muted)]";

  return (
    <article className={`min-w-0 space-y-4 ${bare ? "p-5 sm:p-6" : "block-card"}`}>
      <div>
        <h2 className="mt-2 text-2xl font-bold text-[var(--portal-ink)]">{t("supportEntry")}</h2>
        <p className="mt-2 text-sm text-[var(--portal-muted)]">{t("ticketDescription")}</p>
      </div>

      <div className="rounded-[1rem] border border-[var(--portal-line)] bg-[var(--portal-clay)] p-4">
        <div className="grid gap-3">
          <div>
            <label htmlFor="dashboard-ticket-title" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
              {t("titleLabel")}
            </label>
            <input
              id="dashboard-ticket-title"
              className="field mt-2"
              type="text"
              maxLength={120}
              placeholder={t("titlePlaceholder")}
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              disabled={submitting}
            />
          </div>

          <div>
            <label htmlFor="dashboard-ticket-category" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
              {t("category")}
            </label>
            {/* 弹窗内必须用自绘下拉:原生 <select> 的弹出层由浏览器绘制,在 fixed 弹窗内会错锚甚至不弹出。 */}
            <div className="mt-2">
              <SelectMenu
                id="dashboard-ticket-category"
                value={category}
                options={[
                  { value: "delivery_issue", label: t("deliveryIssue") },
                  { value: "model_feedback", label: t("modelFeedback") },
                  { value: "billing_question", label: t("billingQuestion") },
                  { value: "other", label: t("other") },
                ]}
                onChange={setCategory}
                disabled={submitting}
              />
            </div>
          </div>

          <div>
            <label htmlFor="dashboard-ticket-message" className="text-xs uppercase tracking-[0.18em] text-[var(--portal-muted)]">
              {t("messageLabel")}
            </label>
            <textarea
              id="dashboard-ticket-message"
              className="field mt-2 min-h-[108px] resize-y"
              placeholder={t("messagePlaceholder")}
              value={message}
              onChange={(event) => setMessage(event.target.value)}
              disabled={submitting}
            />
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-3">
        <button type="button" className="btn-primary w-fit" onClick={() => void handleSubmit()} disabled={submitting}>
          {submitting ? t("submittingTicket") : t("createFeedbackTicket")}
        </button>
      </div>

      {submitMessage ? <p className={`text-sm ${ticketMessageClassName}`}>{submitMessage.text}</p> : null}
    </article>
  );
}
