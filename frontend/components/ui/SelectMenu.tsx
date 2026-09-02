"use client";

import { useEffect, useRef, useState } from "react";

import { MaterialIcon } from "@/components/ui/MaterialIcon";

export type SelectMenuOption = {
  value: string;
  label: string;
  hint?: string;
};

type SelectMenuProps = {
  id?: string;
  value: string;
  options: SelectMenuOption[];
  onChange: (value: string) => void;
  /** Trigger text when value is empty or matches no option. */
  placeholder?: string;
  disabled?: boolean;
  /** Extra classes for the trigger button (e.g. "mt-2"). */
  triggerClassName?: string;
};

/**
 * Custom dropdown meant for use inside position:fixed dialogs. A native
 * <select> popup is drawn by the browser outside our CSS control — inside
 * fixed overlays it can misanchor to a screen corner or fail to open at all
 * on some environments. The option list here is plain DOM positioned with
 * position:fixed at the trigger's viewport coordinates: it escapes ancestor
 * overflow clipping (rounded/scrollable modal panels) while always anchoring
 * to the trigger, flipping above when the space below is insufficient.
 */
export function SelectMenu({
  id,
  value,
  options,
  onChange,
  placeholder = "—",
  disabled = false,
  triggerClassName = "",
}: SelectMenuProps) {
  const [rect, setRect] = useState<{ left: number; width: number; top: number | null; bottom: number | null } | null>(null);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  const selected = options.find((option) => option.value === value);

  const toggle = () => {
    if (disabled) return;
    setRect((current) => {
      if (current) return null;
      const el = triggerRef.current;
      if (!el) return null;
      const box = el.getBoundingClientRect();
      const openUp = window.innerHeight - box.bottom < 248;
      return {
        left: box.left,
        width: box.width,
        top: openUp ? null : box.bottom + 4,
        bottom: openUp ? window.innerHeight - box.top + 4 : null,
      };
    });
  };

  useEffect(() => {
    if (!rect) return;
    const close = () => setRect(null);
    window.addEventListener("resize", close);
    return () => window.removeEventListener("resize", close);
  }, [rect]);

  return (
    <>
      <button
        type="button"
        ref={triggerRef}
        id={id}
        aria-haspopup="listbox"
        aria-expanded={rect !== null}
        disabled={disabled}
        onClick={toggle}
        onKeyDown={(event) => {
          if (event.key === "Escape" && rect) {
            // Esc 只关下拉,不冒泡给外层 Modal(避免把整个弹窗关掉)。
            event.stopPropagation();
            setRect(null);
          }
        }}
        className={`field flex items-center justify-between gap-2 text-left ${triggerClassName}`}
      >
        <span className={`truncate ${selected ? "" : "text-[var(--ink-muted)]"}`}>{selected ? selected.label : placeholder}</span>
        <MaterialIcon name={rect ? "expand_less" : "expand_more"} size={18} className="shrink-0 text-[var(--ink-muted)]" />
      </button>
      {rect ? (
        <>
          <button type="button" aria-hidden tabIndex={-1} className="fixed inset-0 z-10 cursor-default" onClick={() => setRect(null)} />
          <ul
            role="listbox"
            className="fixed z-20 max-h-60 overflow-y-auto rounded-xl border border-[var(--line)] bg-[var(--paper)] py-1 shadow-[var(--shadow)]"
            style={
              rect.top !== null
                ? { left: rect.left, width: rect.width, top: rect.top }
                : { left: rect.left, width: rect.width, bottom: rect.bottom ?? 0 }
            }
          >
            {options.length === 0 ? (
              <li className="px-4 py-2 text-sm text-[var(--ink-muted)]">{placeholder}</li>
            ) : (
              options.map((option) => {
                const active = option.value === value;
                return (
                  <li key={option.value}>
                    <button
                      type="button"
                      role="option"
                      aria-selected={active}
                      onClick={() => {
                        onChange(option.value);
                        setRect(null);
                      }}
                      className={`flex w-full items-center justify-between gap-2 px-4 py-2 text-left text-sm transition-colors hover:bg-[var(--accent-wash)] ${
                        active ? "bg-[var(--accent-wash)] font-bold text-[var(--accent-ink)]" : "text-[var(--ink)]"
                      }`}
                    >
                      <span className="truncate">{option.label}</span>
                      {option.hint ? <span className="shrink-0 text-xs text-[var(--ink-muted)]">{option.hint}</span> : null}
                    </button>
                  </li>
                );
              })
            )}
          </ul>
        </>
      ) : null}
    </>
  );
}
