import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import Link from "next/link";
import ThemeToggle from "./components/ThemeToggle";
import Logo from "./components/Logo";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "AI API Portal",
  description: "Frontend scaffold for AI API Portal",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <div className="portal-shell">
          <header className="portal-header clay-panel px-6 py-4">
            <Logo />
            <nav className="flex flex-wrap gap-2 text-sm">
              <Link href="/blog" className="nav-pill">
                Blog
              </Link>
              <Link href="/download" className="nav-pill">
                Download
              </Link>
              <Link href="/pricing" className="nav-pill">
                Pricing
              </Link>
            </nav>
            <div className="flex items-center gap-3">
              <ThemeToggle />
              <Link href="/account" className="btn-primary text-xs">
                Login
              </Link>
            </div>
          </header>
          <main className="portal-main">{children}</main>
        </div>
      </body>
    </html>
  );
}
