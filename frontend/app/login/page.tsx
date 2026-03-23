"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { MaterialIcon } from "@/components/ui/MaterialIcon";

type LoginResponse = {
  user: {
    id: number;
    email: string;
    name: string;
    role: "user" | "admin";
  };
  session_token: string;
};

const SESSION_TOKEN_STORAGE_KEY = "session_token";

export default function LoginPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

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

      const payload = (await response.json()) as LoginResponse | { error?: string };
      if (!response.ok) {
        throw new Error((payload as { error?: string }).error ?? "Login failed");
      }

      const sessionToken = (payload as LoginResponse).session_token;
      localStorage.setItem(SESSION_TOKEN_STORAGE_KEY, sessionToken);

      const role = (payload as LoginResponse).user.role;
      router.replace(role === "admin" ? "/admin" : "/dashboard");
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : "Login failed");
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <section className="portal-shell py-12">
      <div className="mx-auto w-full max-w-[440px] rounded-xl border border-slate-100 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900/50">
        <div className="mb-8 text-center">
          <h1 className="mb-2 text-3xl font-bold leading-tight text-slate-900 dark:text-white">Welcome Back</h1>
          <p className="text-sm text-slate-500 dark:text-slate-400">Login to your ALiang Gateway account</p>
        </div>

        <form className="space-y-5" onSubmit={handleSubmit}>
          <div className="flex flex-col gap-2">
            <label className="text-sm font-medium text-slate-700 dark:text-slate-300" htmlFor="login-email">
              Email
            </label>
            <input
              id="login-email"
              className="h-12 w-full rounded-lg border border-slate-200 bg-white px-4 text-slate-900 placeholder:text-slate-400 focus:border-[var(--stitch-primary)] focus:ring-1 focus:ring-[var(--stitch-primary)] dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
              placeholder="name@company.com"
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
            />
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <label className="text-sm font-medium text-slate-700 dark:text-slate-300" htmlFor="login-password">
                Password
              </label>
            </div>
            <div className="relative flex items-center">
              <input
                id="login-password"
                className="h-12 w-full rounded-lg border border-slate-200 bg-white pl-4 pr-12 text-slate-900 placeholder:text-slate-400 focus:border-[var(--stitch-primary)] focus:ring-1 focus:ring-[var(--stitch-primary)] dark:border-slate-700 dark:bg-slate-800 dark:text-slate-100"
                placeholder="Enter your password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
              />
              <span className="absolute right-4 flex items-center text-slate-400">
                <MaterialIcon name="lock" size={20} />
              </span>
            </div>
          </div>

          {error ? <p className="text-sm text-red-600 dark:text-red-400">{error}</p> : null}

          <button
            className="h-12 w-full rounded-lg bg-[var(--stitch-primary)] font-bold text-white shadow-sm transition-colors hover:bg-[var(--stitch-primary)]/90 disabled:cursor-not-allowed disabled:opacity-60"
            type="submit"
            disabled={isSubmitting}
          >
            {isSubmitting ? "Logging in..." : "Login"}
          </button>
        </form>

        <div className="mt-8 text-center">
          <p className="text-sm text-slate-600 dark:text-slate-400">
            Don&apos;t have an account?
            <Link className="ml-1 font-semibold text-[var(--stitch-primary)] hover:underline" href="/register">
              Create an account
            </Link>
          </p>
        </div>
      </div>
    </section>
  );
}
