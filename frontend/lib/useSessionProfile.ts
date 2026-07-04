"use client";

import { useCallback, useEffect, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

export type SessionProfile = {
  email: string;
  name: string;
  role: string;
};

export const SESSION_TOKEN_KEY = "session_token";

export function hasAdminAccess(role: string) {
  return role === "admin" || role === "distributor";
}

export function adminEntryHref(role: string) {
  if (role === "distributor") return "/distributor";
  return "/admin";
}

export function buildAvatarLabel(user: SessionProfile | null) {
  if (!user) {
    return "??";
  }

  const source = (user.name || user.email.split("@")[0] || "").trim();
  if (!source) {
    return "??";
  }

  const compact = source.replace(/\s+/g, "");
  return Array.from(compact).slice(0, 2).join("").toUpperCase() || "??";
}

/**
 * Shared auth-aware session state for site chrome.
 * Re-fetches the profile on route change so both the app header and the
 * editorial header stay in sync after login/logout/navigation.
 */
export function useSessionProfile() {
  const router = useRouter();
  const pathname = usePathname();
  const [user, setUser] = useState<SessionProfile | null>(null);

  useEffect(() => {
    const sessionToken = localStorage.getItem(SESSION_TOKEN_KEY);
    if (!sessionToken) {
      setUser(null);
      return;
    }

    fetch("/api/auth/me", {
      method: "GET",
      headers: {
        "content-type": "application/json",
        accept: "application/json",
        Authorization: `Bearer ${sessionToken}`,
      },
      cache: "no-store",
    })
      .then((res) => {
        if (!res.ok) {
          localStorage.removeItem(SESSION_TOKEN_KEY);
          return null;
        }
        return res.json();
      })
      .then((data) => {
        if (!data) return;
        const profile = data?.data ?? data;
        if (profile?.email) {
          setUser({ email: profile.email, name: profile.name ?? "", role: profile.role ?? "user" });
        } else {
          localStorage.removeItem(SESSION_TOKEN_KEY);
        }
      })
      .catch(() => {
        localStorage.removeItem(SESSION_TOKEN_KEY);
      });
  }, [pathname]);

  const logout = useCallback(async () => {
    const sessionToken = localStorage.getItem(SESSION_TOKEN_KEY);
    try {
      await fetch("/api/auth/logout", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
          ...(sessionToken ? { Authorization: `Bearer ${sessionToken}` } : {}),
        },
        cache: "no-store",
      });
    } catch {
      // best-effort; clear local state regardless
    }
    localStorage.removeItem(SESSION_TOKEN_KEY);
    setUser(null);
    router.replace("/login");
  }, [router]);

  const isLoggedIn = user !== null;
  const adminHref = user && hasAdminAccess(user.role) ? adminEntryHref(user.role) : null;

  return { user, isLoggedIn, logout, avatarLabel: buildAvatarLabel(user), adminHref };
}
