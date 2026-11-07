import type { Metadata } from "next";

// iOS Safari 顶部原生 Smart App Banner(仅 Safari 渲染),与页内 App Store
// 徽章 CTA 互补;WebView/第三方浏览器不显示,不影响其他平台用户。
export const metadata: Metadata = {
  other: {
    "apple-itunes-app": "app-id=6817166383",
  },
};

export default function DownloadLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return children;
}
