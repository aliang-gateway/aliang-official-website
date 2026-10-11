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

  it("开启时必须填写邮箱验证码", () => {
    expect(validateRegisterSubmit({ ...base, emailCodeRequired: true })).toMatch(
      /verification code/i,
    );
    expect(
      validateRegisterSubmit({
        ...base,
        emailCodeRequired: true,
        emailCode: "  ",
      }),
    ).toMatch(/verification code/i);
    expect(
      validateRegisterSubmit({
        ...base,
        emailCodeRequired: true,
        emailCode: "ABC12345",
      }),
    ).toBeNull();
  });

  it("关闭时不要求邮箱验证码", () => {
    expect(validateRegisterSubmit({ ...base, emailCodeRequired: false })).toBeNull();
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
    expect(body.captcha_key).toBe("cap-key");
    expect(body.captcha_x).toBe(118);
    expect(body.captcha_y).toBe(92);
    expect(
      "captcha_key" in buildRegisterBody({ email: "a@b.co", name: "n", password: "pw" }),
    ).toBe(false);
  });

  it("carries email_code only when present", () => {
    const body = buildRegisterBody({
      email: "a@b.co",
      name: "n",
      password: "pw",
      emailCode: "ABC12345",
    });
    expect(body.email_code).toBe("ABC12345");
    expect(
      "email_code" in buildRegisterBody({ email: "a@b.co", name: "n", password: "pw" }),
    ).toBe(false);
  });
});
