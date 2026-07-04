"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";

type RegisterResponse = {
  user_id?: number;
  email?: string;
  name?: string;
  access_token?: string;
  session_token?: string;
  email_verified?: boolean;
  require_email_verification?: boolean;
};

type VerifyEmailResponse = {
  verified?: boolean;
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

export default function RegisterPage() {
  const router = useRouter();
  const t = useTranslations("register");
  const a = useTranslations("editorial.auth");
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [agreeTerms, setAgreeTerms] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isVerifying, setIsVerifying] = useState(false);
  const [verificationCode, setVerificationCode] = useState("");
  const [pendingVerifyEmail, setPendingVerifyEmail] = useState("");
  const [requireEmailVerification, setRequireEmailVerification] = useState<boolean | null>(null);
  const [isCheckingSession, setIsCheckingSession] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

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
    setSuccess(null);

    if (password !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }
    if (!agreeTerms) {
      setError("Please agree to the terms first");
      return;
    }

    setIsSubmitting(true);
    try {
      const response = await fetch("/api/auth/register", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
        },
        body: JSON.stringify({
          email: email.trim(),
          name: username.trim(),
          password,
        }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Register failed"));
      }

      const registerPayload =
        unwrapData<RegisterResponse>(payload) ??
        ((asRecord(payload) as RegisterResponse | null) ?? {});
      const sessionToken = asString(registerPayload.session_token) || asString(registerPayload.access_token);
      if (sessionToken) {
        localStorage.setItem(SESSION_TOKEN_STORAGE_KEY, sessionToken);
      }

      const requiresVerification = Boolean(registerPayload.require_email_verification);
      setRequireEmailVerification(requiresVerification);
      if (requiresVerification) {
        setPendingVerifyEmail(asString(registerPayload.email, email.trim()));
        setSuccess("Registration submitted. Please enter the email verification code to complete registration.");
      } else {
        setPendingVerifyEmail("");
        setVerificationCode("");
        setSuccess("Registration succeeded. You can login now.");
      }
      setPassword("");
      setConfirmPassword("");
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Register failed");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleVerifyEmail = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    setSuccess(null);

    const targetEmail = pendingVerifyEmail.trim() || email.trim();

    if (!targetEmail) {
      setError("Please provide an email first");
      return;
    }
    if (!verificationCode.trim()) {
      setError("Verification code is required");
      return;
    }

    setIsVerifying(true);
    try {
      const response = await fetch("/api/auth/verify-email", {
        method: "POST",
        headers: {
          "content-type": "application/json",
          accept: "application/json",
        },
        body: JSON.stringify({
          email: targetEmail,
          code: verificationCode.trim(),
        }),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Email verification failed"));
      }

      const verifyPayload =
        unwrapData<VerifyEmailResponse>(payload) ??
        ((asRecord(payload) as VerifyEmailResponse | null) ?? {});
      if (verifyPayload.verified === true) {
        setPendingVerifyEmail("");
        setVerificationCode("");
        setSuccess("Email verified. You can login now.");
      } else {
        setError("Email verification failed");
      }
    } catch (verifyError) {
      setError(verifyError instanceof Error ? verifyError.message : "Email verification failed");
    } finally {
      setIsVerifying(false);
    }
  };

  if (isCheckingSession) {
    return null;
  }

  return (
    <div className="container">
      <div className="auth-split">
        <aside className="auth-hero">
          <div className="label">{a("registerLabel")}</div>
          <h1>
            {a("registerTitle")}
            <span className="dot">.</span>
          </h1>
          <p>{a("registerLead")}</p>
          <ul className="auth-points">
            <li>{a("registerPoint1")}</li>
            <li>{a("registerPoint2")}</li>
            <li>{a("registerPoint3")}</li>
          </ul>
        </aside>

        <section className="auth-card">
          <h2>{t("title")}</h2>
          <p className="auth-sub">{t("subtitle")}</p>

          <form className="auth-form" onSubmit={handleSubmit}>
            <div className="field">
              <label htmlFor="register-username">{t("username")}</label>
              <div className="field-row">
                <input
                  id="register-username"
                  className="field-input"
                  placeholder={t("usernamePlaceholder")}
                  type="text"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  required
                />
                <span className="field-icon">
                  <MaterialIcon name="person" size={20} />
                </span>
              </div>
            </div>

            <div className="field">
              <label htmlFor="register-email">{t("emailAddress")}</label>
              <div className="field-row">
                <input
                  id="register-email"
                  className="field-input"
                  placeholder={t("emailPlaceholder")}
                  type="email"
                  value={email}
                  onChange={(event) => setEmail(event.target.value)}
                  required
                />
                <span className="field-icon">
                  <MaterialIcon name="mail" size={20} />
                </span>
              </div>
            </div>

            <div className="field">
              <label htmlFor="register-password">{t("password")}</label>
              <div className="field-row">
                <input
                  id="register-password"
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

            <div className="field">
              <label htmlFor="register-confirm-password">{t("confirmPassword")}</label>
              <div className="field-row">
                <input
                  id="register-confirm-password"
                  className="field-input"
                  placeholder={t("confirmPasswordPlaceholder")}
                  type="password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  required
                />
                <span className="field-icon">
                  <MaterialIcon name="verified_user" size={20} />
                </span>
              </div>
            </div>

            <label className="auth-terms" htmlFor="register-terms">
              <input
                id="register-terms"
                type="checkbox"
                checked={agreeTerms}
                onChange={(event) => setAgreeTerms(event.target.checked)}
              />
              <span>
                {t("agreeTerms")}
                <Link href="/terms">{t("termsAndConditions")}</Link>
                {t("and")}
                <Link href="/privacy">{t("privacyPolicy")}</Link>
              </span>
            </label>

            {error ? <p className="auth-error">{error}</p> : null}
            {success ? <p className="auth-success">{success}</p> : null}

            <button className="btn primary" type="submit" disabled={isSubmitting}>
              {isSubmitting ? t("creatingAccount") : t("createAccountButton")}
            </button>
          </form>

          <div className="auth-foot">
            {t("hasAccount")}
            <Link href="/login">{t("logIn")}</Link>
          </div>

          {requireEmailVerification !== false && pendingVerifyEmail.trim() ? (
            <>
              <hr className="auth-divider" />
              <form className="auth-form" onSubmit={handleVerifyEmail}>
                <div className="field">
                  <label htmlFor="register-code">{t("emailVerification")}</label>
                  <input
                    id="register-code"
                    className="field-input"
                    placeholder={t("verificationCodePlaceholder")}
                    value={verificationCode}
                    onChange={(event) => setVerificationCode(event.target.value)}
                  />
                </div>
                <button className="btn" type="submit" disabled={isVerifying}>
                  {isVerifying ? t("verifying") : t("verifyEmail")}
                </button>
              </form>
            </>
          ) : null}
        </section>
      </div>
    </div>
  );
}
