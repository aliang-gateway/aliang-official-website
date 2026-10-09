import { describe, it, expect } from "vitest";
import { validateRegisterSubmit, buildRegisterBody } from "../register";

describe("validateRegisterSubmit", () => {
  const base = {
    email: "someone@example.com",
    password: "pw123456",
    confirmPassword: "pw123456",
    agreeTerms: true,
  };

  it("accepts a fully valid submission", () => {
    expect(validateRegisterSubmit(base)).toBeNull();
  });

  it("rejects a malformed email address", () => {
    expect(validateRegisterSubmit({ ...base, email: "not-an-email" })).toMatch(
      /valid email/i,
    );
    expect(validateRegisterSubmit({ ...base, email: "a@b" })).toMatch(
      /valid email/i,
    );
  });

  it("keeps the password-mismatch error", () => {
    expect(
      validateRegisterSubmit({ ...base, confirmPassword: "different" }),
    ).toMatch(/do not match/i);
  });

  it("keeps the terms checkbox error", () => {
    expect(
      validateRegisterSubmit({ ...base, agreeTerms: false }),
    ).toMatch(/agree to the terms/i);
  });

  it("requires a captcha token when turnstile is enabled", () => {
    expect(
      validateRegisterSubmit({ ...base, turnstileEnabled: true, turnstileToken: "" }),
    ).toMatch(/captcha/i);
    expect(
      validateRegisterSubmit({
        ...base,
        turnstileEnabled: true,
        turnstileToken: "tok",
      }),
    ).toBeNull();
  });

  it("does not require a captcha token when turnstile is disabled", () => {
    expect(
      validateRegisterSubmit({ ...base, turnstileEnabled: false }),
    ).toBeNull();
  });
});

describe("buildRegisterBody", () => {
  it("includes turnstile_token only when present", () => {
    const withToken = buildRegisterBody({
      email: " a@b.co ",
      name: " n ",
      password: "pw",
      turnstileToken: "tok",
    });
    expect(withToken).toEqual({
      email: "a@b.co",
      name: "n",
      password: "pw",
      turnstile_token: "tok",
    });

    const withoutToken = buildRegisterBody({
      email: "a@b.co",
      name: "n",
      password: "pw",
    });
    expect(withoutToken).toEqual({ email: "a@b.co", name: "n", password: "pw" });
    expect("turnstile_token" in withoutToken).toBe(false);
  });
});
