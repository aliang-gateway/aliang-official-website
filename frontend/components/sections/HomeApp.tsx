"use client";

import Link from "next/link";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type HomeVariant = "full" | "compact";

interface HomeAppProps {
  variant?: HomeVariant;
}

const appFeatures = [
  { icon: "smartphone", titleKey: "feature1Title", descriptionKey: "feature1Description" },
  { icon: "sync", titleKey: "feature2Title", descriptionKey: "feature2Description" },
  { icon: "notifications_active", titleKey: "feature3Title", descriptionKey: "feature3Description" },
] as const;

export function HomeApp({ variant = "full" }: HomeAppProps) {
  const isCompact = variant === "compact";
  const t = useTranslations("homeApp");

  return (
    <section
      id="mobile-app"
      data-od-id="home-app"
      className={`relative overflow-hidden bg-[var(--stitch-bg-elevated)] ${isCompact ? "py-12" : "py-20"}`}
    >
      {/* 背景装饰：单一克制的绿色辉光，呼应品牌且不抢眼 */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute -right-32 top-1/2 hidden -translate-y-1/2 lg:block"
        style={{
          width: 520,
          height: 520,
          borderRadius: "50%",
          background: "radial-gradient(circle, rgba(33,196,93,0.10) 0%, transparent 70%)",
        }}
      />

      <div className="relative mx-auto max-w-7xl px-6 md:px-20">
        <div
          className={`grid grid-cols-1 items-center gap-12 lg:grid-cols-2 ${
            isCompact ? "gap-10" : "gap-16"
          }`}
        >
          {/* 左侧：文案 + 特性列表 */}
          <div className={`flex flex-col ${isCompact ? "gap-6" : "gap-8"}`}>
            <div
              className={`inline-flex w-fit items-center gap-2 rounded-full bg-[var(--stitch-primary)]/10 px-3 py-1 text-[10px] font-bold uppercase tracking-wider text-[var(--stitch-primary)]`}
            >
              <span className="relative flex h-2 w-2">
                <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75" />
                <span className="relative inline-flex h-2 w-2 rounded-full bg-[var(--stitch-primary)]" />
              </span>
              {t("badge")}
            </div>

            <div className={`flex flex-col ${isCompact ? "gap-3" : "gap-4"}`}>
              <h2
                className={`${isCompact ? "text-3xl md:text-4xl" : "text-4xl md:text-5xl"} font-black leading-tight tracking-tight text-[var(--stitch-text)]`}
              >
                {t("title")}
                <span className="text-[var(--stitch-primary)]">{t("titleHighlight")}</span>
              </h2>
              <p
                className={`max-w-xl leading-relaxed text-[var(--stitch-text-muted)] ${
                  isCompact ? "text-base" : "text-lg"
                }`}
              >
                {t("description")}
              </p>
            </div>

            {/* 特性列表：列表式而非卡片网格，与 HomeFeatures 的卡片网格区分开，形成节奏变化 */}
            <ul className={`flex flex-col ${isCompact ? "gap-4" : "gap-5"}`}>
              {appFeatures.map((feature) => (
                <li key={feature.titleKey} className="flex items-start gap-4">
                  <div
                    className={`flex flex-shrink-0 items-center justify-center rounded-lg bg-[var(--stitch-primary)]/10 ${
                      isCompact ? "size-9" : "size-10"
                    }`}
                  >
                    <MaterialIcon
                      name={feature.icon}
                      size={isCompact ? 18 : 20}
                      className="text-[var(--stitch-primary)]"
                    />
                  </div>
                  <div className="flex flex-col gap-1">
                    <h3
                      className={`${isCompact ? "text-sm" : "text-base"} font-bold text-[var(--stitch-text)]`}
                    >
                      {t(feature.titleKey)}
                    </h3>
                    <p
                      className={`leading-relaxed text-[var(--stitch-text-muted)] ${
                        isCompact ? "text-xs" : "text-sm"
                      }`}
                    >
                      {t(feature.descriptionKey)}
                    </p>
                  </div>
                </li>
              ))}
            </ul>

            {/* 下载入口：应用商店按钮 */}
            <div className={`flex flex-wrap items-center gap-3 pt-2`}>
              <a
                href="#"
                className="flex cursor-pointer items-center gap-2 rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-4 py-2.5 text-sm font-semibold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg-elevated)]"
              >
                <MaterialIcon name="phone_iphone" size={18} />
                {t("iosStore")}
              </a>
              <a
                href="#"
                className="flex cursor-pointer items-center gap-2 rounded-lg border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-4 py-2.5 text-sm font-semibold text-[var(--stitch-text)] transition-colors hover:bg-[var(--stitch-bg-elevated)]"
              >
                <MaterialIcon name="android" size={18} />
                {t("androidStore")}
              </a>
              <Link
                href="/services"
                className={`flex items-center gap-1 font-bold text-[var(--stitch-primary)] hover:underline ${
                  isCompact ? "text-xs" : "text-sm"
                }`}
              >
                {t("learnMore")}
                <MaterialIcon name="arrow_forward" size={isCompact ? 12 : 14} />
              </Link>
            </div>
          </div>

          {/* 右侧：纯 CSS iPhone mockup + 聊天界面 */}
          <div className="relative flex justify-center lg:justify-end">
            {/* 手机外框 */}
            <div
              className="relative"
              style={{
                width: isCompact ? 240 : 280,
                height: isCompact ? 500 : 580,
                borderRadius: 42,
                background: "#0a0a0a",
                padding: 10,
                boxShadow:
                  "0 30px 60px -15px rgba(0,0,0,0.35), 0 0 0 1px rgba(255,255,255,0.06)",
              }}
            >
              {/* 屏幕 */}
              <div
                className="relative flex h-full w-full flex-col overflow-hidden"
                style={{
                  borderRadius: 32,
                  background: "var(--stitch-bg)",
                }}
              >
                {/* Dynamic Island */}
                <div
                  className="absolute left-1/2 top-2 z-10 -translate-x-1/2"
                  style={{
                    width: isCompact ? 72 : 84,
                    height: isCompact ? 20 : 24,
                    borderRadius: 999,
                    background: "#000",
                  }}
                />

                {/* 状态栏 */}
                <div
                  className={`flex items-center justify-between px-5 pt-2.5 text-[10px] font-semibold text-[var(--stitch-text)]`}
                  style={{ height: 36 }}
                >
                  <span className="tracking-tight">9:41</span>
                  <div className="flex items-center gap-1 opacity-80">
                    <MaterialIcon name="signal_cellular_alt" size={11} />
                    <MaterialIcon name="wifi" size={11} />
                    <MaterialIcon name="battery_full" size={12} />
                  </div>
                </div>

                {/* App 导航栏 */}
                <div
                  className={`flex items-center justify-between border-b border-[var(--stitch-border)] px-4`}
                  style={{ height: 44 }}
                >
                  <div className="flex items-center gap-1.5">
                    <div
                      className="flex items-center justify-center rounded-md"
                      style={{
                        width: 22,
                        height: 22,
                        background: "var(--stitch-primary)",
                      }}
                    >
                      <MaterialIcon
                        name="bolt"
                        size={13}
                        className="text-white"
                      />
                    </div>
                    <span
                      className={`text-[11px] font-bold tracking-tight text-[var(--stitch-text)]`}
                    >
                      ALiang
                    </span>
                  </div>
                  <MaterialIcon
                    name="more_horiz"
                    size={16}
                    className="text-[var(--stitch-text-muted)]"
                  />
                </div>

                {/* 会话标签 */}
                <div
                  className={`flex items-center gap-2 px-4 py-2.5 text-[10px]`}
                >
                  <div className="flex items-center gap-1.5 rounded-md bg-[var(--stitch-primary)]/10 px-2 py-0.5 text-[var(--stitch-primary)]">
                    <span className="h-1.5 w-1.5 rounded-full bg-[var(--stitch-primary)]" />
                    <span className="font-bold">vibe-coding</span>
                  </div>
                  <span className="text-[var(--stitch-text-muted)]">
                    aliang-gateway · main
                  </span>
                </div>

                {/* 聊天内容 */}
                <div
                  className={`flex flex-1 flex-col gap-2.5 overflow-hidden px-3 ${
                    isCompact ? "py-2" : "py-3"
                  }`}
                >
                  {/* 用户消息 */}
                  <div className="flex justify-end">
                    <div
                      className={`rounded-2xl rounded-br-sm px-3 py-1.5 text-[10px] leading-relaxed text-white`}
                      style={{
                        maxWidth: "78%",
                        background: "var(--stitch-primary)",
                      }}
                    >
                      在网关里加一个 /v1/embeddings 路由
                    </div>
                  </div>

                  {/* AI 回复 */}
                  <div className="flex justify-start">
                    <div
                      className={`rounded-2xl rounded-bl-sm border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-3 py-1.5 text-[10px] leading-relaxed text-[var(--stitch-text)]`}
                      style={{ maxWidth: "82%" }}
                    >
                      <div className="mb-1 flex items-center gap-1">
                        <span
                          className="inline-block rounded-sm px-1 py-px font-mono text-[8px] font-bold"
                          style={{
                            background: "rgba(33,196,93,0.15)",
                            color: "var(--stitch-primary)",
                          }}
                        >
                          claude
                        </span>
                        <span className="text-[var(--stitch-text-muted)]">
                          正在编辑
                        </span>
                      </div>
                      <span className="font-mono text-[9px] text-[var(--stitch-text)]">
                        routes/embeddings.ts
                      </span>
                      <div className="mt-1 font-mono text-[8px] leading-tight text-[var(--stitch-text-muted)]">
                        <div>+ app.post(&quot;/v1/embeddings&quot;, handler)</div>
                        <div>+ validateBody(schema)</div>
                      </div>
                    </div>
                  </div>

                  {/* 状态条 */}
                  <div className="flex items-center gap-1.5 px-1 text-[9px] text-[var(--stitch-text-muted)]">
                    <span className="relative flex h-1.5 w-1.5">
                      <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[var(--stitch-primary)] opacity-75" />
                      <span className="relative inline-flex h-1.5 w-1.5 rounded-full bg-[var(--stitch-primary)]" />
                    </span>
                    正在运行测试 · 4 项通过
                  </div>

                  {/* AI 第二条回复：diff 状态 */}
                  <div className="flex justify-start">
                    <div
                      className={`flex items-center gap-2 rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg-elevated)] px-2.5 py-1.5`}
                    >
                      <MaterialIcon
                        name="check_circle"
                        size={14}
                        className="text-[var(--stitch-primary)]"
                      />
                      <div className="flex flex-col">
                        <span className="text-[9px] font-bold text-[var(--stitch-text)]">
                          PR #142 已创建
                        </span>
                        <span className="text-[8px] text-[var(--stitch-text-muted)]">
                          feat: add /v1/embeddings route
                        </span>
                      </div>
                    </div>
                  </div>
                </div>

                {/* 底部输入栏 */}
                <div
                  className={`flex items-center gap-2 border-t border-[var(--stitch-border)] px-3`}
                  style={{ height: 44, marginBottom: 8 }}
                >
                  <div
                    className="flex flex-1 items-center rounded-full border border-[var(--stitch-border)] bg-[var(--stitch-bg)] px-3 py-1.5"
                  >
                    <span className="text-[10px] text-[var(--stitch-text-muted)]">
                      给 AI 发指令…
                    </span>
                  </div>
                  <div
                    className="flex items-center justify-center rounded-full"
                    style={{
                      width: 30,
                      height: 30,
                      background: "var(--stitch-primary)",
                    }}
                  >
                    <MaterialIcon name="arrow_upward" size={15} className="text-white" />
                  </div>
                </div>

                {/* Home indicator */}
                <div
                  className="absolute bottom-1.5 left-1/2 -translate-x-1/2 rounded-full"
                  style={{
                    width: 100,
                    height: 4,
                    background: "var(--stitch-text-muted)",
                    opacity: 0.4,
                  }}
                />
              </div>
            </div>

            {/* 扫码下载块 */}
            {!isCompact && (
              <div
                className="absolute -left-6 bottom-6 hidden flex-col items-center gap-2 rounded-xl border border-[var(--stitch-border)] bg-[var(--stitch-bg)] p-3 shadow-lg md:flex"
                style={{ width: 116 }}
              >
                {/* CSS 画一个示意二维码 */}
                <div
                  className="relative grid grid-cols-7 gap-px rounded-md bg-white p-1.5"
                  style={{ width: 76, height: 76 }}
                  aria-label={t("scanToDownload")}
                >
                  {Array.from({ length: 49 }).map((_, i) => {
                    // 简化的伪随机分布，呈现二维码视觉特征
                    const filled =
                      // 三个角的定位方块
                      i < 8 ||
                      i % 7 < 1 ||
                      i % 7 === 6 ||
                      i > 41 ||
                      // 中心区域散点
                      ((i * 5) % 7 === 0 && i % 3 === 0) ||
                      ((i * 3) % 5 === 0 && i % 2 === 1);
                    return (
                      <div
                        key={i}
                        style={{
                          background: filled ? "#0a0a0a" : "transparent",
                        }}
                      />
                    );
                  })}
                </div>
                <span className="text-center text-[10px] font-medium leading-tight text-[var(--stitch-text-muted)]">
                  {t("scanToDownload")}
                </span>
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
