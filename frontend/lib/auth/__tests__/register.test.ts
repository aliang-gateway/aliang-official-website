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

  it("requires a solved slider when the slider captcha is loaded", () => {
    expect(
      validateRegisterSubmit({ ...base, sliderRequired: true }),
    ).toMatch(/slider/i);
    expect(
      validateRegisterSubmit({
        ...base,
        sliderRequired: true,
        sliderSolved: true,
      }),
    ).toBeNull();
  });

  it("does not require a slider when no challenge was issued", () => {
    expect(validateRegisterSubmit({ ...base, sliderRequired: false })).toBeNull();
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

  it("carries slider captcha fields when solved", () => {
    const body = buildRegisterBody({
      email: "a@b.co",
      name: "n",
      password: "pw",
      slider: { key: "cap-key", x: 118, y: 92 },
    });
    expect(body).toEqual({
      email: "a@b.co",
      name: "n",
      password: "pw",
      captcha_key: "cap-key",
      captcha_x: 118,
      captcha_y: 92,
    });
    expect(
      "captcha_key" in
        buildRegisterBody({ email: "a@b.co", name: "n", password: "pw" }),
    ).toBe(false);
  });
});
