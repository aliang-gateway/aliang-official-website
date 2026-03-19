"use client";

import { useState } from "react";
import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type RegisterResponse = {
  user_id: number;
  email: string;
  name: string;
  session_token: string;
  email_verified: boolean;
};

type VerifyEmailResponse = {
  verified: boolean;
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

export default function RegisterPage() {
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [agreeTerms, setAgreeTerms] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isVerifying, setIsVerifying] = useState(false);
  const [verificationCode, setVerificationCode] = useState("");
  const [pendingVerifyEmail, setPendingVerifyEmail] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

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

      const payload = (await response.json()) as RegisterResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "Register failed");
      }

      localStorage.setItem(SESSION_TOKEN_STORAGE_KEY, (payload as RegisterResponse).session_token);
      setPendingVerifyEmail((payload as RegisterResponse).email);
      setSuccess("Registration succeeded. Please verify your email before login.");
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

      const payload = (await response.json()) as VerifyEmailResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "Email verification failed");
      }

      if ((payload as VerifyEmailResponse).verified) {
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

  return (
    <section className="portal-shell py-12">
      <div className="mx-auto w-full max-w-[480px] rounded-xl border border-[var(--stitch-primary)]/10 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900/50">
        <div className="mb-8 flex flex-col gap-2 text-center sm:text-left">
          <h1 className="text-3xl font-black leading-tight text-slate-900 dark:text-slate-100">Create Account</h1>
          <p className="text-base text-slate-500 dark:text-slate-400">Join ALiang Gateway to get started</p>
        </div>

        <form className="flex flex-col gap-5" onSubmit={handleSubmit}>
          <div className="flex flex-col gap-2">
            <label className="text-sm font-semibold text-slate-900 dark:text-slate-100" htmlFor="register-username">
              Username
            </label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">
                <MaterialIcon name="person" size={20} />
              </span>
              <input
                id="register-username"
                className="h-12 w-full rounded border border-slate-200 bg-white pl-10 pr-4 text-base text-slate-900 placeholder:text-slate-400 focus:outline-0 focus:ring-2 focus:ring-[var(--stitch-primary)]/50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                placeholder="Enter your username"
                type="text"
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                required
              />
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-sm font-semibold text-slate-900 dark:text-slate-100" htmlFor="register-email">
              Email Address
            </label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">
                <MaterialIcon name="mail" size={20} />
              </span>
              <input
                id="register-email"
                className="h-12 w-full rounded border border-slate-200 bg-white pl-10 pr-4 text-base text-slate-900 placeholder:text-slate-400 focus:outline-0 focus:ring-2 focus:ring-[var(--stitch-primary)]/50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                placeholder="name@example.com"
                type="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                required
              />
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-sm font-semibold text-slate-900 dark:text-slate-100" htmlFor="register-password">
              Password
            </label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">
                <MaterialIcon name="lock" size={20} />
              </span>
              <input
                id="register-password"
                className="h-12 w-full rounded border border-slate-200 bg-white pl-10 pr-4 text-base text-slate-900 placeholder:text-slate-400 focus:outline-0 focus:ring-2 focus:ring-[var(--stitch-primary)]/50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                placeholder="Create a password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <label className="text-sm font-semibold text-slate-900 dark:text-slate-100" htmlFor="register-confirm-password">
              Confirm Password
            </label>
            <div className="relative">
              <span className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400">
                <MaterialIcon name="verified_user" size={20} />
              </span>
              <input
                id="register-confirm-password"
                className="h-12 w-full rounded border border-slate-200 bg-white pl-10 pr-4 text-base text-slate-900 placeholder:text-slate-400 focus:outline-0 focus:ring-2 focus:ring-[var(--stitch-primary)]/50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                placeholder="Confirm your password"
                type="password"
                value={confirmPassword}
                onChange={(event) => setConfirmPassword(event.target.value)}
                required
              />
            </div>
          </div>

          <div className="flex items-center gap-3 py-2">
            <input
              className="size-5 rounded border-slate-300 bg-white text-[var(--stitch-primary)] focus:ring-[var(--stitch-primary)] dark:border-slate-700 dark:bg-slate-800"
              id="register-terms"
              type="checkbox"
              checked={agreeTerms}
              onChange={(event) => setAgreeTerms(event.target.checked)}
            />
            <label className="text-sm text-slate-600 dark:text-slate-400" htmlFor="register-terms">
              I agree to the
              <a className="mx-1 text-[var(--stitch-primary)] hover:underline" href="/docs">
                Terms and Conditions
              </a>
              and
              <a className="ml-1 text-[var(--stitch-primary)] hover:underline" href="/docs">
                Privacy Policy
              </a>
            </label>
          </div>

          {error ? <p className="text-sm text-red-600 dark:text-red-400">{error}</p> : null}
          {success ? <p className="text-sm text-emerald-600 dark:text-emerald-400">{success}</p> : null}

          <button
            className="mt-2 w-full rounded bg-[var(--stitch-primary)] py-3.5 font-bold text-white shadow-lg shadow-[var(--stitch-primary)]/20 transition-colors hover:bg-[var(--stitch-primary)]/90 disabled:cursor-not-allowed disabled:opacity-60"
            type="submit"
            disabled={isSubmitting}
          >
            {isSubmitting ? "Creating account..." : "Create Account"}
          </button>
        </form>

        <p className="mt-8 text-center text-sm text-slate-600 dark:text-slate-400">
          Already have an account?
          <Link className="ml-1 font-bold text-[var(--stitch-primary)] hover:underline" href="/login">
            Log in
          </Link>
        </p>

        <form className="mt-6 flex flex-col gap-3 border-t border-slate-200 pt-5 dark:border-slate-700" onSubmit={handleVerifyEmail}>
          <p className="text-sm font-semibold text-slate-700 dark:text-slate-300">Email Verification</p>
          <input
            className="h-11 w-full rounded border border-slate-200 bg-white px-3 text-sm text-slate-900 placeholder:text-slate-400 focus:outline-0 focus:ring-2 focus:ring-[var(--stitch-primary)]/50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
            placeholder="Enter verification code"
            value={verificationCode}
            onChange={(event) => setVerificationCode(event.target.value)}
          />
          <button
            className="w-fit rounded bg-slate-900 px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60 dark:bg-slate-100 dark:text-slate-900 dark:hover:bg-slate-200"
            type="submit"
            disabled={isVerifying}
          >
            {isVerifying ? "Verifying..." : "Verify Email"}
          </button>
        </form>
      </div>
    </section>
  );
}
