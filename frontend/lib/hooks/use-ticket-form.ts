"use client";

import { useCallback, useState } from "react";

import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";

export type TicketMessageTone = "success" | "error";

export type TicketForm = {
  title: string;
  setTitle: (value: string) => void;
  category: string;
  setCategory: (value: string) => void;
  message: string;
  setMessage: (value: string) => void;
  submitting: boolean;
  submitMessage: { tone: TicketMessageTone; text: string } | null;
  handleSubmit: () => Promise<void>;
};

type UseTicketFormArgs = {
  sessionToken: string;
};

export function useTicketForm({ sessionToken }: UseTicketFormArgs): TicketForm {
  const [title, setTitleState] = useState("");
  const [category, setCategoryState] = useState("delivery_issue");
  const [message, setMessageState] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitMessage, setSubmitMessage] = useState<{ tone: TicketMessageTone; text: string } | null>(null);

  const setTitle = useCallback((value: string) => {
    setTitleState(value);
    setSubmitMessage(null);
  }, []);

  const setCategory = useCallback((value: string) => {
    setCategoryState(value);
    setSubmitMessage(null);
  }, []);

  const setMessage = useCallback((value: string) => {
    setMessageState(value);
    setSubmitMessage(null);
  }, []);

  const handleSubmit = useCallback(async () => {
    setSubmitMessage(null);

    if (!sessionToken) {
      setSubmitMessage({ tone: "error", text: "Your session token is missing. Sign in again before creating a feedback ticket." });
      return;
    }

    const normalizedTitle = title.trim();
    const normalizedMessage = message.trim();

    if (!normalizedTitle || !category.trim() || !normalizedMessage) {
      setSubmitMessage({ tone: "error", text: "Title, category, and message are required before submitting your feedback ticket." });
      return;
    }

    setSubmitting(true);

    try {
      const response = await fetch("/api/dashboard/tickets", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          Authorization: `Bearer ${sessionToken}`,
        },
        body: JSON.stringify({
          title: normalizedTitle,
          category,
          message: normalizedMessage,
        }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Ticket submission is unavailable right now."));
      }

      const ticketEnvelope = unwrapData<{ ticket_id?: string }>(payload);
      const ticketRoot = asRecord(payload);
      const legacyTicketResult = asRecord(ticketRoot?.result);
      const ticketId =
        asString(ticketEnvelope?.ticket_id) ||
        asString(ticketRoot?.ticket_id) ||
        asString(legacyTicketResult?.ticket_id);

      setTitleState("");
      setCategoryState("delivery_issue");
      setMessageState("");
      setSubmitMessage({
        tone: "success",
        text: `Feedback ticket submitted successfully${ticketId ? ` (ID: ${ticketId})` : ""}.`,
      });
    } catch (submitError) {
      setSubmitMessage({
        tone: "error",
        text: submitError instanceof Error ? `Ticket submission failed: ${submitError.message}` : "Ticket submission failed.",
      });
    } finally {
      setSubmitting(false);
    }
  }, [category, message, sessionToken, title]);

  return {
    title,
    setTitle,
    category,
    setCategory,
    message,
    setMessage,
    submitting,
    submitMessage,
    handleSubmit,
  };
}
