"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useTranslations } from "next-intl";
import { MaterialIcon } from "@/components/ui/MaterialIcon";
import {
  SliderCaptchaField,
  fetchRegisterCaptcha,
  type SlideCaptchaChallenge,
  type SlideCaptchaSolve,
} from "@/components/auth/SlideCaptcha";
import { Turnstile, turnstileEnabled } from "@/components/auth/Turnstile";
import { asRecord, asString, extractApiError, unwrapData } from "@/lib/api-response";
import { buildRegisterBody, validateRegisterSubmit } from "@/lib/auth/register";

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
  const [turnstileToken, setTurnstileToken] = useState("");
  const [turnstileReset, setTurnstileReset] = useState(0);
  const [slideChallenge, setSlideChallenge] = useState<SlideCaptchaChallenge | null>(null);
  const [sliderSolve, setSliderSolve] = useState<SlideCaptchaSolve | null>(null);
  const [emailCodeRequired, setEmailCodeRequired] = useState(false);
  const [emailCode, setEmailCode] = useState("");
  const [sendCodeCooldown, setSendCodeCooldown] = useState(0);
  const [codeSentNotice, setCodeSentNotice] = useState<string | null>(null);
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

  // Load the slider challenge once the session check has settled. A null
  // result (slider disabled upstream or network hiccup) simply hides the
  // widget — the backend only enforces what it issued.
  useEffect(() => {
    if (isCheckingSession) {
      return;
    }
    let cancelled = false;
    fetchRegisterCaptcha().then((challenge) => {
      if (!cancelled) {
        setSlideChallenge(challenge);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [isCheckingSession]);

  const refreshSlideChallenge = useCallback(async () => {
    setSliderSolve(null);
    setSlideChallenge(await fetchRegisterCaptcha());
  }, []);

  // 注册选项探测：邮箱验证运行时开关（admin 可随时切换，这里只影响渲染）。
  useEffect(() => {
    if (isCheckingSession) {
      return;
    }
    let cancelled = false;
    fetch("/api/auth/register/options", { cache: "no-store" })
      .then((response) => (response.ok ? response.json() : null))
      .then((payload) => {
        if (!cancelled && payload && typeof payload === "object") {
          setEmailCodeRequired(Boolean((payload as Record<string, unknown>).email_verification));
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [isCheckingSession]);

  // 发码倒计时
  useEffect(() => {
    if (sendCodeCooldown <= 0) {
      return;
    }
    const timer = window.setInterval(() => {
      setSendCodeCooldown((seconds) => (seconds <= 1 ? 0 : seconds - 1));
    }, 1000);
    return () => window.clearInterval(timer);
  }, [sendCodeCooldown]);

  const handleSendCode = async () => {
    setError(null);
    setCodeSentNotice(null);
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      setError(t("invalidEmailForCode"));
      return;
    }
    try {
      const response = await fetch("/api/auth/register/send-code", {
        method: "POST",
        headers: { "content-type": "application/json", accept: "application/json" },
        body: JSON.stringify({ email: email.trim() }),
      });
      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        throw new Error(extractApiError(payload, "Failed to send code"));
      }
      setCodeSentNotice(t("codeSent"));
      setSendCodeCooldown(60);
    } catch (sendError) {
      setError(sendError instanceof Error ? sendError.message : "Failed to send code");
    }
  };

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    setSuccess(null);

    const validationError = validateRegisterSubmit({
      email,
      password,
      confirmPassword,
      agreeTerms,
      turnstileEnabled,
      turnstileToken,
      sliderRequired: slideChallenge !== null,
      sliderSolved: sliderSolve !== null,
      emailCodeRequired,
      emailCode,
    });
    if (validationError) {
      setError(validationError);
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
        body: JSON.stringify(
          buildRegisterBody({
            email,
            name: username,
            password,
            turnstileToken,
            slider: sliderSolve,
            emailCode,
          }),
        ),
      });

      const payload = (await response.json()) as unknown;
      if (!response.ok) {
        // Captcha artifacts are single-use: reset the Turnstile widget and
        // fetch a fresh slider so a retry starts from clean state.
        setTurnstileReset((signal) => signal + 1);
        if (slideChallenge !== null) {
          void refreshSlideChallenge();
        }
        if (emailCodeRequired) {
          setEmailCode("");
        }
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

            {emailCodeRequired ? (
              <div className="field">
                <label htmlFor="register-email-code">{t("emailCodeLabel")}</label>
                <div className="field-row">
                  <input
                    id="register-email-code"
                    className="field-input"
                    placeholder={t("emailCodePlaceholder")}
                    value={emailCode}
                    onChange={(event) => setEmailCode(event.target.value)}
                    autoComplete="one-time-code"
                  />
                  <button
                    type="button"
                    className="btn ghost"
                    onClick={handleSendCode}
                    disabled={sendCodeCooldown > 0 || isSubmitting}
                  >
                    {sendCodeCooldown > 0 ? t("resendIn", { seconds: sendCodeCooldown }) : t("sendCode")}
                  </button>
                </div>
                {codeSentNotice ? <p className="text-xs font-bold text-emerald-500">{codeSentNotice}</p> : null}
              </div>
            ) : null}

            {turnstileEnabled ? (
              <div className="field">
                <Turnstile onToken={setTurnstileToken} resetSignal={turnstileReset} />
              </div>
            ) : null}

            {slideChallenge ? (
              <div className="field">
                <SliderCaptchaField
                  challenge={slideChallenge}
                  solved={sliderSolve !== null}
                  onSolve={setSliderSolve}
                  onReset={() => setSliderSolve(null)}
                  onRefresh={refreshSlideChallenge}
                  labels={{
                    button: t("sliderCaptchaButton"),
                    verified: t("sliderCaptchaVerified"),
                    hint: t("sliderCaptchaHint"),
                    refresh: t("sliderCaptchaRefresh"),
                    close: t("sliderCaptchaClose"),
                  }}
                />
              </div>
            ) : null}

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
