"use client";

import { useEffect, useRef, type ReactNode, type RefObject } from "react";

type ModalProps = {
  isOpen: boolean;
  onClose: () => void;
  closeLabel: string;
  /** Element to return focus to when the modal closes (the tile that opened it). */
  triggerRef: RefObject<HTMLElement | null>;
  children: ReactNode;
  /** Extra classes for the panel (e.g. a wider max-width). */
  panelClassName?: string;
};

/**
 * Reusable accessible modal shell: centred panel over an ink-tinted backdrop,
 * with Escape-to-close, a Tab focus trap, scroll lock, and focus return to the
 * triggering element. The body is provided via children; a close button sits
 * top-right. Mirrors the chrome used by ConfigModal so every dialog feels the same.
 */
export function Modal({ isOpen, onClose, closeLabel, triggerRef, children, panelClassName = "" }: ModalProps) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const closeBtnRef = useRef<HTMLButtonElement | null>(null);
  const hadOpenRef = useRef(false);

  useEffect(() => {
    if (!isOpen) {
      if (hadOpenRef.current) {
        triggerRef.current?.focus();
        hadOpenRef.current = false;
      }
      return;
    }

    hadOpenRef.current = true;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    closeBtnRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }
      if (event.key !== "Tab" || !panelRef.current) return;
      const focusable = panelRef.current.querySelectorAll<HTMLElement>(
        'button, [href], input, textarea, select, [tabindex]:not([tabindex="-1"])'
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;
      if (event.shiftKey && active === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen, onClose, triggerRef]);

  if (!isOpen) return null;

  return (
    <section
      className="fixed inset-0 z-[70] flex items-center justify-center p-4 sm:p-6"
      role="dialog"
      aria-modal="true"
    >
      <button
        type="button"
        className="absolute inset-0 bg-[var(--ink)]/55 backdrop-blur-sm"
        aria-label={closeLabel}
        onClick={onClose}
      />
      <div
        ref={panelRef}
        className={`relative z-[1] flex max-h-[90vh] w-full max-w-lg flex-col overflow-hidden rounded-[1.4rem] border border-[var(--line)] bg-[var(--paper-warm)] shadow-[var(--shadow)] ${panelClassName}`}
      >
        <button
          type="button"
          ref={closeBtnRef}
          className="absolute right-3 top-3 z-[2] inline-flex h-9 w-9 items-center justify-center rounded-full border border-[var(--line)] bg-[var(--paper)] text-xl font-semibold leading-none text-[var(--ink)] transition-transform duration-200 hover:-translate-y-[1px]"
          aria-label={closeLabel}
          onClick={onClose}
        >
          ×
        </button>
        <div className="min-h-0 overflow-y-auto">{children}</div>
      </div>
    </section>
  );
}
