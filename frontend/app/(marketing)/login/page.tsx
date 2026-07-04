"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { ScanLoginPanel } from "@/components/auth/ScanLoginPanel";
import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";
import { useTranslations } from "next-intl";

type LoginResponse = {
  access_token?: string;
  session_token?: string;
  refresh_token?: string;
  user?: {
    id?: number;
    email?: string;
    name?: string;
    role?: "user" | "admin" | "distributor";
  };
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

export default function LoginPage() {
  return (
    <Suspense>
      <LoginContent />
    </Suspense>
  );
}

function LoginContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const t = useTranslations("login");
  const a = useTranslations("editorial.auth");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isCheckingSession, setIsCheckingSession] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<"password" | "scan">("password");

  const nextPath = searchParams.get("next")?.trim() ?? "";

  useEffect(() => {
    const sessionToken = localStorage.getItem(SESSION_TOKEN_STORAGE_KEY);
    if (sessionToken) {
      router.replace("/dashboard");
      return;
    }
    setIsCheckingSession(false);
  }, [router]);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    setIsSubmitting(true);

    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
        },
        body: JSON.stringify({
          email: email.trim(),
          password,
        }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Login failed"));
      }

      const data = unwrapData<LoginResponse>(payload);
      const legacyPayload = asRecord(payload);
      const nestedPayload = asRecord(legacyPayload?.data);
      const sessionToken =
        asString(legacyPayload?.session_token) ||
        asString(nestedPayload?.session_token) ||
        asString(data?.session_token) ||
        asString(data?.access_token);
      if (!sessionToken) {
        throw new Error(extractApiError(payload, "Login succeeded but access token is missing"));
      }
      localStorage.setItem(SESSION_TOKEN_STORAGE_KEY, sessionToken);

      const role = data?.user?.role ?? asRecord(legacyPayload?.user)?.role;
      const safeNextPath = nextPath.startsWith("/") && !nextPath.startsWith("//") ? nextPath : "";
      router.replace(
        safeNextPath ||
          (role === "distributor" ? "/distributor" : role === "admin" ? "/admin/users" : "/dashboard"),
      );
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Login failed");
    } finally {
      setIsSubmitting(false);
    }
  };

  if (isCheckingSession) {
    return null;
  }

  return (
    <div className="container">
      <div className="auth-split">
        <aside className="auth-hero">
          <div className="label">{a("loginLabel")}</div>
          <h1>
            {a("loginTitle")}
            <span className="dot">.</span>
          </h1>
          <p>{a("loginLead")}</p>
          <ul className="auth-points">
            <li>{a("loginPoint1")}</li>
            <li>{a("loginPoint2")}</li>
            <li>{a("loginPoint3")}</li>
          </ul>
        </aside>

        <section className="auth-card">
          <h2>{t("welcomeBack")}</h2>
          <p className="auth-sub">{t("subtitle")}</p>

          <div className="auth-tabs" aria-label="login mode">
            <button
              type="button"
              aria-pressed={mode === "password"}
              onClick={() => setMode("password")}
            >
              {t("passwordTab")}
            </button>
            <button
              type="button"
              aria-pressed={mode === "scan"}
              onClick={() => setMode("scan")}
            >
              {t("scanTab")}
            </button>
          </div>

          {mode === "password" && (
            <form className="auth-form" onSubmit={handleSubmit}>
              <div className="field">
                <label htmlFor="login-email">{t("email")}</label>
                <input
                  id="login-email"
                  className="field-input"
                  placeholder={t("emailPlaceholder")}
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  required
                />
              </div>

              <div className="field">
                <label htmlFor="login-password">{t("password")}</label>
                <div className="field-row">
                  <input
                    id="login-password"
                    className="field-input"
                    placeholder={t("passwordPlaceholder")}
                    type="password"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    required
                  />
                  <span className="field-icon">
                    <MaterialIcon name="lock" size={20} />
                  </span>
                </div>
              </div>

              {error ? <p className="auth-error">{error}</p> : null}

              <button className="btn primary" type="submit" disabled={isSubmitting}>
                {isSubmitting ? t("loggingIn") : t("loginButton")}
              </button>
            </form>
          )}

          {mode === "scan" && <ScanLoginPanel nextPath={nextPath} />}

          <div className="auth-foot">
            {t("noAccount")}
            <Link href="/register">{t("createAccount")}</Link>
          </div>
        </section>
      </div>
    </div>
  );
}
