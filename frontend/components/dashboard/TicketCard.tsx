"use client";

import { useTranslations } from "next-intl";

import { useTicketForm } from "@/lib/hooks/use-ticket-form";

type TicketCardProps = {
  sessionToken: string;
};

export function TicketCard({ sessionToken }: TicketCardProps) {
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
        ? "text-emerald-500 dark:text-emerald-400"
        : "text-[var(--portal-muted)]";

  return (
    <article className="block-card min-w-0 space-y-4">
      <div>
        <p className="text-sm font-semibold text-emerald-500 dark:text-emerald-400">{t("ticketFeedback")}</p>
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
            <select
              id="dashboard-ticket-category"
              className="field mt-2"
              value={category}
              onChange={(event) => setCategory(event.target.value)}
              disabled={submitting}
            >
              <option value="delivery_issue">{t("deliveryIssue")}</option>
              <option value="model_feedback">{t("modelFeedback")}</option>
              <option value="billing_question">{t("billingQuestion")}</option>
              <option value="other">{t("other")}</option>
            </select>
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
