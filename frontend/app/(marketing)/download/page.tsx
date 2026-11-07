"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { unzip } from "fflate";

type MockCard = {
  name: string;
  platform: "desktop" | "mobile";
  icon: string;
  desc: string;
  meta: string[];
  disabled?: boolean;
};
type DownloadItem = {
  id: number;
  software_name: string;
  platform: string;
  file_type: string;
  download_url: string;
  version: string;
  is_default?: boolean;
  changelog?: string;
};
type MacVariant = { label: string; url?: string };
type Card = {
  name: string;
  platform: "desktop" | "mobile";
  icon: string;
  desc: string;
  meta: string[];
  disabled?: boolean;
  downloadUrl?: string;
  /** download_url 指向 zip 包（COS 拦 .apk 直链，Android 走 zip + 网页内解压）。 */
  zipDownload?: boolean;
  /** 解压后触发浏览器保存时使用的 .apk 文件名。 */
  downloadName?: string;
  macVariants?: MacVariant[];
};

const PLATFORM_MAP: Record<string, { name: string; icon: string }> = {
  darwin: { name: "macOS", icon: "Mac" },
  macos: { name: "macOS", icon: "Mac" },
  windows: { name: "Windows", icon: "Win" },
  win: { name: "Windows", icon: "Win" },
  linux: { name: "Linux", icon: "Lin" },
};

// macOS 架构识别(按 file_type 关键字)
const isAppleSilicon = (ft: string) => /arm64|aarch64|apple|silicon/i.test(ft);
const isIntel = (ft: string) => /x64|x86_64|intel/i.test(ft);

export default function DownloadPage() {
  const s = useTranslations("editorial.download");

  const mockCards = s.raw("cards") as MockCard[];
  const release = s.raw("release") as string[];
  const installSteps = s.raw("installSteps") as Record<string, string>;

  const mockByName = new Map(mockCards.map((c) => [c.name, c]));

  const [realDownloads, setRealDownloads] = useState<DownloadItem[] | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [status, setStatus] = useState<string | null>(null);
  const [zipBusy, setZipBusy] = useState(false);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/public/downloads", { cache: "no-store" })
      .then((res) => (res.ok ? res.json() : null))
      .then((data) => {
        if (cancelled) return;
        const items: DownloadItem[] = Array.isArray(data?.downloads) ? data.downloads : [];
        setRealDownloads(items);
      })
      .catch(() => {
        if (!cancelled) setRealDownloads([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Build the card list. Desktop cards come from real downloads when present,
  // otherwise the mock. macOS gets two variants (Apple Silicon / Intel) with a
  // three-tier fallback (split by file_type → share one → placeholder). Mobile
  // roadmap cards are always appended from the mock so Android/iOS stay visible.
  const cards: Card[] = (() => {
    const mobileMock = mockCards.filter((c) => c.platform === "mobile");
    const hasReal = (realDownloads?.length ?? 0) > 0;
    if (!hasReal) {
      return mockCards.map((c) => ({ ...c }));
    }
    const macItems = (realDownloads ?? []).filter((it) => /darwin|macos/i.test(it.platform ?? ""));
    const armItem = macItems.find((it) => isAppleSilicon(it.file_type ?? "")) ?? macItems[0];
    const intelItem = macItems.find((it) => isIntel(it.file_type ?? "")) ?? macItems[0] ?? armItem;

    const seen = new Set<string>();
    const realCards: Card[] = [];
    for (const item of realDownloads ?? []) {
      const m = PLATFORM_MAP[item.platform?.toLowerCase()];
      if (!m || seen.has(m.name)) continue;
      seen.add(m.name);
      const mock = mockByName.get(m.name);
      const meta = [
        item.version ? s("metaVersion", { version: item.version }) : s("metaLatest"),
        item.file_type ? s("metaType", { type: item.file_type }) : s("metaClient"),
      ];

      if (m.name === "macOS") {
        realCards.push({
          name: m.name,
          platform: "desktop",
          icon: m.icon,
          desc: mock?.desc ?? item.software_name ?? "",
          meta,
          macVariants: [
            { label: s("macArm"), url: armItem?.download_url || undefined },
            { label: s("macIntel"), url: intelItem?.download_url || undefined },
          ],
        });
        continue;
      }

      realCards.push({
        name: m.name,
        platform: "desktop",
        icon: m.icon,
        desc: mock?.desc ?? item.software_name ?? "",
        meta,
        downloadUrl: item.download_url || undefined,
      });
    }
    // Mobile: Android 用 admin 配的真实下载地址（platform=android），iOS 用 admin 配的 App Store 链接或 mock 占位。
    const androidReal = (realDownloads ?? []).find((it) => /android/i.test(it.platform ?? ""));
    const iosReal = (realDownloads ?? []).find((it) => /ios/i.test(it.platform ?? ""));
    const mobileBuilt = mobileMock.map((c) => {
      if (c.name === "Android" && androidReal?.download_url) {
        // 约定：Android 条目的 download_url 恒为 zip（内部文件中性化），
        // file_type 落库为 zip。走网页内 fetch→解压→Blob 存 .apk 流程。
        const isZip = /zip/i.test(androidReal.file_type ?? "");
        return {
          ...c,
          downloadUrl: androidReal.download_url,
          ...(isZip
            ? {
                zipDownload: true,
                downloadName: `aliang-${androidReal.version || "latest"}-arm64.apk`,
              }
            : {}),
        };
      }
      if (c.name === "iOS" && iosReal?.download_url) {
        return { ...c, downloadUrl: iosReal.download_url, disabled: false };
      }
      return { ...c };
    });
    return [...realCards, ...mobileBuilt];
  })();

  const desktopCards = cards.filter((c) => c.platform === "desktop");
  const mobileCards = cards.filter((c) => c.platform === "mobile");

  const installText = selected && installSteps[selected] ? installSteps[selected] : s("installDefault");

  // Android zip 下载：COS 拦 .apk 直链，安装包以 zip 分发（内部文件名刻意
  // 中性化为 app-arm64.bin）。页面内 fetch(带进度) → fflate 解压 → Blob 指定
  // .apk 文件名触发浏览器保存，zip 内部名与落盘名无关。任何一步失败都回退
  // 为 zip 直链（浏览器存 zip，用户手动解压）。
  const downloadAndroidZip = async (c: Card) => {
    const url = c.downloadUrl!;
    const name = c.downloadName ?? "aliang-arm64.apk";
    setZipBusy(true);
    try {
      setStatus(s("statusZipFetch"));
      const res = await fetch(url, { mode: "cors", cache: "no-store" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const total = Number(res.headers.get("content-length")) || 0;
      let buf: ArrayBuffer;
      if (res.body && total > 0) {
        const reader = res.body.getReader();
        const chunks: Uint8Array[] = [];
        let got = 0;
        for (;;) {
          const { done, value } = await reader.read();
          if (done) break;
          chunks.push(value);
          got += value.length;
          setStatus(s("statusZipFetchPct", { pct: Math.round((got / total) * 100) }));
        }
        buf = await new Blob(chunks as BlobPart[]).arrayBuffer();
      } else {
        buf = await res.arrayBuffer();
      }
      setStatus(s("statusZipUnzip"));
      const files = await new Promise<Record<string, Uint8Array>>((resolve, reject) => {
        unzip(new Uint8Array(buf), (err, data) => (err ? reject(err) : resolve(data)));
      });
      const entry =
        Object.entries(files).find(([n]) => /\.bin$/i.test(n)) ?? Object.entries(files)[0];
      if (!entry) throw new Error("zip empty");
      const blob = new Blob([entry[1] as BlobPart], {
        type: "application/vnd.android.package-archive",
      });
      const objUrl = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = objUrl;
      a.download = name;
      document.body.appendChild(a);
      a.click();
      a.remove();
      setTimeout(() => URL.revokeObjectURL(objUrl), 30_000);
      setStatus(s("statusZipDone", { name }));
    } catch {
      window.open(url, "_blank", "noopener,noreferrer");
      setStatus(s("statusZipError"));
    } finally {
      setZipBusy(false);
    }
  };

  const onCardAction = (c: Card) => {
    setSelected(c.name);
    if (c.zipDownload && c.downloadUrl && !zipBusy) {
      void downloadAndroidZip(c);
      return;
    }
    if (c.downloadUrl) {
      window.open(c.downloadUrl, "_blank", "noopener,noreferrer");
      setStatus(s("statusDownloading", { name: c.name }));
    } else {
      setStatus(s("statusSelected", { name: c.name }));
    }
  };

  const onMacAction = (c: Card, v: MacVariant) => {
    setSelected(c.name);
    if (v.url) {
      window.open(v.url, "_blank", "noopener,noreferrer");
      setStatus(s("statusDownloading", { name: `${c.name} (${v.label})` }));
    } else {
      setStatus(s("statusMacSelect", { name: c.name, variant: v.label }));
    }
  };

  const renderCard = (c: Card, i: number) => {
    const isSel = selected === c.name && !c.disabled;
    return (
      <article
        key={c.name}
        className={`download-card${c.disabled ? " is-disabled" : ""}`}
        data-selected={isSel ? "" : undefined}
        data-platform={c.platform}
        data-reveal
        data-reveal-delay={String(Math.min(i, 4))}
      >
        <div>
          <div className="platform">
            <span className="platform-icon">{c.icon}</span>
            <span className={`dl-tag${c.disabled ? " warn" : ""}`}>
              {c.disabled ? s("unsupported") : s("available")}
            </span>
          </div>
          <h3>{c.name}</h3>
          <p>{c.desc}</p>
        </div>
        <div className="meta">
          {c.meta.map((m) => (
            <span key={m}>{m}</span>
          ))}
        </div>
        {c.macVariants ? (
          <div className="download-btn-group">
            {c.macVariants.map((v) => (
              <button key={v.label} className="download-btn" type="button" onClick={() => onMacAction(c, v)}>
                {v.label}
              </button>
            ))}
          </div>
        ) : (
          <button
            className="download-btn"
            type="button"
            disabled={c.disabled || (c.zipDownload && zipBusy)}
            onClick={() => !c.disabled && !(c.zipDownload && zipBusy) && onCardAction(c)}
          >
            {c.disabled ? s("unsupported") : s("downloadBtn", { name: c.name })}
          </button>
        )}
      </article>
    );
  };

  const renderGroup = (
    list: Card[],
    labelKey: string,
    titleKey: string,
    titleId: string,
  ) => (
    <section className="download-group" aria-labelledby={titleId}>
      <div className="group-head" data-reveal>
        <div className="label">{s(labelKey)}</div>
        <h3 className="group-title" id={titleId}>
          {s(titleKey)}
          <span className="dot">.</span>
        </h3>
      </div>
      <div className="download-grid">{list.map((c, i) => renderCard(c, i))}</div>
    </section>
  );

  return (
    <div className="page-download">
      <header className="hero" aria-labelledby="hero-title">
        <div className="container wide hero-grid">
          <div data-reveal>
            <div className="label">{s("heroLabel")}</div>
            <h1 className="display" id="hero-title">
              {s("heroTitle")}
              <span className="dot">.</span>
            </h1>
            <p className="lead">{s("heroLead")}</p>
          </div>
          <figure className="plate" data-reveal>
            <img src="/editorial/cta.svg" alt="" width={1024} height={1024} loading="lazy" />
          </figure>
        </div>
      </header>

      <main>
        <section className="download-section" aria-labelledby="download-title">
          <div className="container wide">
            <div className="download-head" data-reveal>
              <div>
                <div className="label">{s("headLabel")}</div>
                <h2 className="display" id="download-title">
                  {s("headTitle")}
                  <span className="dot">.</span>
                </h2>
              </div>
              <p className="lead">{s("headLead")}</p>
            </div>

            {renderGroup(desktopCards, "groupDesktopLabel", "groupDesktopTitle", "desktop-title")}
            {renderGroup(mobileCards, "groupMobileLabel", "groupMobileTitle", "mobile-title")}

            <p className="status-line" aria-live="polite">
              {status ?? ""}
            </p>

            <div className="install-panel" data-reveal>
              <div className="code-box">
                <h3>{s("installTitle")}</h3>
                <pre>{installText}</pre>
              </div>
              <aside className="release-box">
                <h3>{s("releaseTitle")}</h3>
                <ul>
                  {release.map((r) => (
                    <li key={r}>{r}</li>
                  ))}
                </ul>
              </aside>
            </div>
          </div>
        </section>
      </main>

      <section className="closing" aria-labelledby="closing-title">
        <div className="container closing-grid">
          <div>
            <div className="label">{s("closingLabel")}</div>
            <h2 className="display" id="closing-title">
              {s("closingTitle")}
              <span className="dot">.</span>
            </h2>
            <p>{s("closingLead")}</p>
          </div>
          <Link className="btn" href="/services">
            {s("closingBtn")}
          </Link>
        </div>
      </section>
    </div>
  );
}
