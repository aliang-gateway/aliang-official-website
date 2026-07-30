"use client";

import { useEffect, useMemo, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { parseAndReduce } from "@/lib/sse";
import { SAMPLES, type SampleKey } from "@/lib/sse/samples";
import type { Protocol } from "@/lib/sse";

type Tab = "text" | "usage" | "tools" | "timeline";

export function SseParserClient() {
  const t = useTranslations("editorial.tools.sseParser");
  const locale = useLocale();
  const isEn = locale === "en";
  const [raw, setRaw] = useState("");
  const [debouncedRaw, setDebouncedRaw] = useState("");
  const [forced, setForced] = useState<"auto" | Protocol>("auto");
  const [tab, setTab] = useState<Tab>("text");
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedRaw(raw), 250);
    return () => clearTimeout(timer);
  }, [raw]);

  const parseNow = () => setDebouncedRaw(raw);

  const parsed = useMemo(() => {
    if (!debouncedRaw.trim()) return null;
    return parseAndReduce(debouncedRaw, forced === "auto" ? null : forced);
  }, [debouncedRaw, forced]);

  const result = parsed?.result ?? null;
  const recognized = parsed?.protocol ?? null;

  const onCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 1500);
    } catch {
      /* ignore */
    }
  };

  const tabs: { key: Tab; label: string }[] = [
    { key: "text", label: t("tabText") },
    { key: "usage", label: t("tabUsage") },
    { key: "tools", label: t("tabTools") },
    { key: "timeline", label: t("tabTimeline") },
  ];

  const usageRows = result
    ? [
        { k: "usageModel", v: result.usage.model },
        { k: "usageInput", v: result.usage.inputTokens },
        { k: "usageOutput", v: result.usage.outputTokens },
        { k: "usageTotal", v: result.usage.totalTokens },
        { k: "usageCacheRead", v: result.usage.cacheReadTokens },
        { k: "usageCacheCreate", v: result.usage.cacheCreationTokens },
        { k: "usageReasoning", v: result.usage.reasoningTokens },
        { k: "usageStopReason", v: result.usage.stopReason },
      ]
    : [];

  return (
    <div className="tool-layout">
      <div className="tool-input">
        <label className="label" htmlFor="sse-raw">{t("inputLabel")}</label>
        <textarea
          id="sse-raw"
          value={raw}
          onChange={(e) => setRaw(e.target.value)}
          placeholder={t("inputPlaceholder")}
          spellCheck={false}
        />
        <div className="tool-controls">
          <button type="button" className="btn primary" onClick={parseNow}>{t("parseBtn")}</button>
          <label className="label" htmlFor="sse-proto">{t("protocolLabel")}</label>
          <select
            id="sse-proto"
            value={forced}
            onChange={(e) => setForced(e.target.value as "auto" | Protocol)}
          >
            <option value="auto">{t("protocolAuto")}</option>
            <option value="anthropic">{t("protocolAnthropic")}</option>
            <option value="openai">{t("protocolOpenai")}</option>
          </select>
          <select
            aria-label={t("sampleLabel")}
            value=""
            onChange={(e) => {
              const key = e.target.value as SampleKey;
              if (key) setRaw(SAMPLES[key].raw);
            }}
          >
            <option value="">{t("sampleLabel")}</option>
            {(Object.keys(SAMPLES) as SampleKey[]).map((k) => (
              <option key={k} value={k}>{isEn ? SAMPLES[k].labelEn : SAMPLES[k].labelZh}</option>
            ))}
          </select>
          <button type="button" className="btn" onClick={() => onCopy(raw)}>{t("copyBtn")}</button>
          <button type="button" className="btn" onClick={() => { setRaw(""); }}>{t("clearBtn")}</button>
        </div>
        <p className="tool-note" aria-hidden={copied ? undefined : true}>{copied ? t("copied") : ""}</p>
      </div>

      <div className="tool-result">
        {debouncedRaw.trim() && !recognized && (
          <div className="tool-error-banner">{t("unrecognized")}</div>
        )}

        {!result ? (
          <div className="tool-panel tool-empty">{debouncedRaw.trim() ? "" : t("empty")}</div>
        ) : (
          <>
            <div className="tool-tabs" role="tablist">
              {tabs.map((tb) => (
                <button
                  key={tb.key}
                  role="tab"
                  aria-selected={tab === tb.key}
                  onClick={() => setTab(tb.key)}
                >
                  {tb.label}
                </button>
              ))}
            </div>

            {tab === "text" && (
              <div className="tool-panel">
                {result.text ? <pre>{result.text}</pre> : <p className="tool-empty">{t("noText")}</p>}
                <div className="tool-copy">
                  <button type="button" className="btn" onClick={() => onCopy(result.text)}>{t("copyBtn")}</button>
                </div>
              </div>
            )}

            {tab === "usage" && (
              <div className="tool-panel">
                {result.usage.raw || usageRows.some((r) => r.v !== undefined && r.v !== "") ? (
                  <table className="tool-usage">
                    <tbody>
                      {usageRows.map((r) => (
                        <tr key={r.k}>
                          <th>{t(r.k as never)}</th>
                          <td className={typeof r.v === "number" ? "num" : ""}>
                            {r.v === undefined || r.v === "" ? "—" : String(r.v)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <p className="tool-empty">{t("noUsage")}</p>
                )}
              </div>
            )}

            {tab === "tools" && (
              <div className="tool-panel">
                {result.toolCalls.length === 0 ? (
                  <p className="tool-empty">{t("noTools")}</p>
                ) : (
                  result.toolCalls.map((tc) => (
                    <div className="tool-call" key={tc.index}>
                      <div className="tool-call-head">
                        <span className="tool-call-name">{tc.name || "—"}</span>
                        <span className="tool-call-id">{tc.id}</span>
                      </div>
                      <pre>{JSON.stringify(tc.arguments, null, 2)}</pre>
                      {tc.argumentsParseError && <p className="tool-note">{t("argsHint")}</p>}
                    </div>
                  ))
                )}
              </div>
            )}

            {tab === "timeline" && (
              <div className="tool-panel tool-timeline">
                <div className="tool-timeline-row tool-timeline-head" aria-hidden="true">
                  <span className="idx">{t("timelineIndex")}</span>
                  <span>{t("timelineType")}</span>
                  <span>{t("timelineDelta")}</span>
                </div>
                {parsed!.events.map((ev) => (
                  <div className={`tool-timeline-row${ev.ok ? "" : " is-error"}`} key={ev.index}>
                    <span className="idx">#{ev.index}</span>
                    <span>{ev.event || ((ev.json as Record<string, unknown> | null)?.type as string | undefined) || (ev.isDone ? "[DONE]" : "—")}</span>
                    <span className="delta">{ev.isDone ? "" : ev.error ?? ev.raw.slice(0, 120)}</span>
                  </div>
                ))}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
