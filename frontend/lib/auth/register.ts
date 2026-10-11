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
  // Self-hosted slider captcha. sliderRequired is true only when the
  // backend actually issued a challenge (it 404s when the slider is off).
  sliderRequired?: boolean;
  sliderSolved?: boolean;
  // 注册邮箱验证码（portal 自有）。emailCodeRequired 来自运行时开关探测。
  emailCodeRequired?: boolean;
  emailCode?: string;
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
  if (input.sliderRequired && !input.sliderSolved) {
    return "Please complete the slider verification";
  }
  if (input.emailCodeRequired && !(input.emailCode ?? "").trim()) {
    return "Please enter the email verification code";
  }
  return null;
}

// buildRegisterBody assembles the POST /api/auth/register payload, trimming
// user input and including captcha fields only when present so the upstream
// contract stays unchanged when the captchas are disabled.
export function buildRegisterBody(input: {
  email: string;
  name: string;
  password: string;
  turnstileToken?: string;
  slider?: { key: string; x: number; y: number } | null;
  emailCode?: string;
}): Record<string, string | number> {
  const body: Record<string, string | number> = {
    email: input.email.trim(),
    name: input.name.trim(),
    password: input.password,
  };
  const token = (input.turnstileToken ?? "").trim();
  if (token) {
    body["turnstile_token"] = token;
  }
  if (input.slider && input.slider.key) {
    body["captcha_key"] = input.slider.key;
    body["captcha_x"] = input.slider.x;
    body["captcha_y"] = input.slider.y;
  }
  const emailCode = (input.emailCode ?? "").trim();
  if (emailCode) {
    body["email_code"] = emailCode;
  }
  return body;
}
