"use client";

// Self-hosted slider captcha (backend: go-captcha). Renders the challenge
// master image plus the draggable tile; the parent receives the drop
// position in native image coordinates once the user releases the tile.

import { useCallback, useEffect, useRef, useState } from "react";

import { Modal } from "@/components/ui/Modal";
import { createPortal } from "react-dom";

export type SlideCaptchaChallenge = {
  captcha_key: string;
  master_image: string;
  tile_image: string;
  tile_x: number;
  tile_y: number;
  tile_size: number;
  width: number;
  height: number;
};

export type SlideCaptchaSolve = { key: string; x: number; y: number };

type DragState = {
  pointerId: number;
  startX: number;
  startY: number;
  baseX: number;
  baseY: number;
};

export function SlideCaptcha({
  challenge,
  onSolve,
  onReset,
}: {
  challenge: SlideCaptchaChallenge;
  onSolve: (solve: SlideCaptchaSolve) => void;
  // Fired when the user starts dragging again after a solve, so the parent
  // can clear a previously recorded (now stale) answer.
  onReset?: () => void;
}) {
  const [tilePos, setTilePos] = useState({
    x: challenge.tile_x,
    y: challenge.tile_y,
  });
  const [dragging, setDragging] = useState(false);
  const dragRef = useRef<DragState | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const solvedRef = useRef(false);

  useEffect(() => {
    setTilePos({ x: challenge.tile_x, y: challenge.tile_y });
    solvedRef.current = false;
  }, [challenge]);

  const clamp = useCallback(
    (x: number, y: number) => ({
      x: Math.min(Math.max(x, 0), challenge.width - challenge.tile_size),
      y: Math.min(Math.max(y, 0), challenge.height - challenge.tile_size),
    }),
    [challenge.width, challenge.height, challenge.tile_size],
  );

  const handlePointerDown = (event: React.PointerEvent<HTMLImageElement>) => {
    event.preventDefault();
    // Capture failure (e.g. synthetic pointers in automation) must not abort
    // the drag — move/up keep working on the element itself.
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // ignore
    }
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      baseX: tilePos.x,
      baseY: tilePos.y,
    };
    setDragging(true);
    if (solvedRef.current) {
      solvedRef.current = false;
      onReset?.();
    }
  };

  const handlePointerMove = (event: React.PointerEvent<HTMLImageElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) {
      return;
    }
    setTilePos(
      clamp(
        drag.baseX + event.clientX - drag.startX,
        drag.baseY + event.clientY - drag.startY,
      ),
    );
  };

  const handlePointerUp = (event: React.PointerEvent<HTMLImageElement>) => {
    const drag = dragRef.current;
    if (!drag || drag.pointerId !== event.pointerId) {
      return;
    }
    dragRef.current = null;
    setDragging(false);
    const final = clamp(
      drag.baseX + event.clientX - drag.startX,
      drag.baseY + event.clientY - drag.startY,
    );
    setTilePos(final);
    solvedRef.current = true;
    onSolve({
      key: challenge.captcha_key,
      x: Math.round(final.x),
      y: Math.round(final.y),
    });
  };

  return (
    <div
      ref={containerRef}
      className="slide-captcha"
      style={{
        width: challenge.width,
        height: challenge.height,
        position: "relative",
        touchAction: "none",
      }}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={challenge.master_image}
        alt=""
        width={challenge.width}
        height={challenge.height}
        draggable={false}
        style={{ display: "block", borderRadius: 8 }}
      />
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={challenge.tile_image}
        alt=""
        width={challenge.tile_size}
        height={challenge.tile_size}
        draggable={false}
        onPointerDown={handlePointerDown}
        onPointerMove={handlePointerMove}
        onPointerUp={handlePointerUp}
        onPointerCancel={handlePointerUp}
        style={{
          position: "absolute",
          left: tilePos.x,
          top: tilePos.y,
          cursor: dragging ? "grabbing" : "grab",
          touchAction: "none",
        }}
      />
    </div>
  );
}

// fetchRegisterCaptcha loads a challenge; null means the backend has the
// slider disabled (404) or the request failed — the form then works without
// a captcha, matching a backend that never required one.
export async function fetchRegisterCaptcha(): Promise<SlideCaptchaChallenge | null> {
  try {
    const response = await fetch("/api/auth/register-captcha", {
      cache: "no-store",
    });
    if (!response.ok) {
      return null;
    }
    return (await response.json()) as SlideCaptchaChallenge;
  } catch {
    return null;
  }
}

// --- compact form-field wrapper: button + dialog ---

export type SliderCaptchaFieldLabels = {
  button: string;
  verified: string;
  hint: string;
  refresh: string;
  close: string;
};

// SliderCaptchaField keeps the registration form compact: a single
// button-shaped row that opens the actual slider inside a dialog. The
// challenge canvas only mounts while the dialog is open.
export function SliderCaptchaField({
  challenge,
  solved,
  onSolve,
  onReset,
  onRefresh,
  labels,
}: {
  challenge: SlideCaptchaChallenge | null;
  solved: boolean;
  onSolve: (solve: SlideCaptchaSolve) => void;
  onReset: () => void;
  onRefresh: () => void;
  labels: SliderCaptchaFieldLabels;
}) {
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => setMounted(true), []);

  const handleSolve = (solve: SlideCaptchaSolve) => {
    onSolve(solve);
    // Let the user see the solved state for a beat, then close.
    window.setTimeout(() => setOpen(false), 350);
  };

  return (
    <>
      <button
        type="button"
        ref={triggerRef}
        onClick={() => setOpen(true)}
        className={`flex w-full items-center justify-between rounded-[var(--radius,0.75rem)] border px-4 py-3 text-sm transition-colors ${
          solved
            ? "border-emerald-500/60 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300"
            : "border-[var(--line)] bg-[var(--paper)] text-[var(--ink)]/70 hover:border-[var(--ink)]/40"
        }`}
      >
        <span className="inline-flex items-center gap-2">
          <span aria-hidden>{solved ? "✓" : "🛡"}</span>
          {solved ? labels.verified : labels.button}
        </span>
        {!solved && challenge ? (
          <span
            role="button"
            tabIndex={0}
            aria-label={labels.refresh}
            className="text-xs underline decoration-dotted opacity-60 hover:opacity-100"
            onClick={(event) => {
              event.stopPropagation();
              onReset();
              onRefresh();
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.stopPropagation();
                onReset();
                onRefresh();
              }
            }}
          >
            {labels.refresh}
          </span>
        ) : null}
      </button>

      {/* Portal 到 body：营销布局的祖先样式会破坏 fixed 定位，导致弹窗被困在表单列里 */}
      {mounted
        ? createPortal(
            <Modal
              isOpen={open}
              onClose={() => setOpen(false)}
              closeLabel={labels.close}
              triggerRef={triggerRef}
            >
              <div className="flex flex-col items-center gap-3 p-5">
                {challenge ? (
                  <>
                    <SlideCaptcha
                      challenge={challenge}
                      onSolve={handleSolve}
                      onReset={onReset}
                    />
                    <p className="text-xs opacity-60">{labels.hint}</p>
                    <button
                      type="button"
                      className="text-xs underline decoration-dotted opacity-60 hover:opacity-100"
                      onClick={() => {
                        onReset();
                        onRefresh();
                      }}
                    >
                      {labels.refresh}
                    </button>
                  </>
                ) : (
                  <p className="py-10 text-sm opacity-60">…</p>
                )}
              </div>
            </Modal>,
            document.body,
          )
        : null}
    </>
  );
}
