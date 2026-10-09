// Pure helpers behind the registration form, kept out of the page component
// so they stay unit-testable (mirrors lib/auth/scan-login.ts).

export type RegisterSubmitInput = {
  email: string;
  password: string;
  confirmPassword: string;
  agreeTerms: boolean;
  // Cloudflare Turnstile. The backend only enforces the captcha when it is
  // configured with a secret key; the client mirrors that with the site key.
  turnstileEnabled?: boolean;
  turnstileToken?: string;
};

// validateRegisterSubmit returns the first user-facing error message, or
// null when the submission may proceed.
export function validateRegisterSubmit(
  input: RegisterSubmitInput,
): string | null {
  if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(input.email.trim())) {
    return "Please enter a valid email address";
  }
  if (input.password !== input.confirmPassword) {
    return "Passwords do not match";
  }
  if (!input.agreeTerms) {
    return "Please agree to the terms first";
  }
  if (input.turnstileEnabled && !(input.turnstileToken ?? "").trim()) {
    return "Please complete the captcha verification";
  }
  return null;
}

// buildRegisterBody assembles the POST /api/auth/register payload, trimming
// user input and including turnstile_token only when a token exists so the
// upstream contract stays unchanged when the captcha is disabled.
export function buildRegisterBody(input: {
  email: string;
  name: string;
  password: string;
  turnstileToken?: string;
}): Record<string, string> {
  const body: Record<string, string> = {
    email: input.email.trim(),
    name: input.name.trim(),
    password: input.password,
  };
  const token = (input.turnstileToken ?? "").trim();
  if (token) {
    body["turnstile_token"] = token;
  }
  return body;
}
