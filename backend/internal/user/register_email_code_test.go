package user

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// capturingMailSender 记录每次发信，供断言验证码邮件内容。
type capturingMailSender struct {
	mu    sync.Mutex
	calls []MailCall
}

type MailCall struct {
	ToEmail string
	Subject string
	Body    string
}

func (s *capturingMailSender) Send(_ context.Context, toEmail, subject, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, MailCall{ToEmail: toEmail, Subject: subject, Body: body})
	return nil
}

func (s *capturingMailSender) last() MailCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls[len(s.calls)-1]
}

func newRegisterCodeService(t *testing.T) (*Service, *capturingMailSender) {
	t.Helper()
	sender := &capturingMailSender{}
	svc := NewServiceWithOptions(setupTestDB(t), ServiceOptions{MailSender: sender, SQLDialect: "sqlite"})
	return svc, sender
}

func TestIssueRegisterEmailCode_SendsAndStores(t *testing.T) {
	t.Parallel()

	svc, sender := newRegisterCodeService(t)
	if err := svc.IssueRegisterEmailCode(context.Background(), " NewUser@Example.com "); err != nil {
		t.Fatalf("IssueRegisterEmailCode() error = %v", err)
	}

	call := sender.last()
	if call.ToEmail != "newuser@example.com" {
		t.Fatalf("sent to %q, want normalized email", call.ToEmail)
	}
	if !strings.Contains(call.Body, "code") {
		t.Fatalf("email body should contain the code: %q", call.Body)
	}

	// 数据库里的码必须与邮件中的一致（从 body 提取最后一个词粗校验存在性）
	var stored string
	if err := svc.db.QueryRow(`SELECT code FROM als_register_email_codes WHERE email = ?`, "newuser@example.com").Scan(&stored); err != nil {
		t.Fatalf("query stored code: %v", err)
	}
	if !strings.Contains(call.Body, stored) {
		t.Fatalf("email body %q does not contain stored code %q", call.Body, stored)
	}
	if len(stored) < 6 {
		t.Fatalf("code %q suspiciously short", stored)
	}
}

func TestIssueRegisterEmailCode_OverwritesPreviousCode(t *testing.T) {
	t.Parallel()

	svc, _ := newRegisterCodeService(t)
	ctx := context.Background()
	email := "overwrite@example.com"

	if err := svc.IssueRegisterEmailCode(ctx, email); err != nil {
		t.Fatalf("first issue: %v", err)
	}
	var firstCode string
	if err := svc.db.QueryRow(`SELECT code FROM als_register_email_codes WHERE email = ?`, email).Scan(&firstCode); err != nil {
		t.Fatalf("query first code: %v", err)
	}

	if err := svc.IssueRegisterEmailCode(ctx, email); err != nil {
		t.Fatalf("second issue: %v", err)
	}
	var secondCode string
	if err := svc.db.QueryRow(`SELECT code FROM als_register_email_codes WHERE email = ?`, email).Scan(&secondCode); err != nil {
		t.Fatalf("query second code: %v", err)
	}
	if firstCode == secondCode {
		t.Fatal("reissue must generate a fresh code")
	}

	// 旧码已失效、新码可用
	if err := svc.VerifyRegisterEmailCode(ctx, email, firstCode); err == nil {
		t.Fatal("old code must be invalidated by reissue")
	}
	if err := svc.VerifyRegisterEmailCode(ctx, email, secondCode); err != nil {
		t.Fatalf("new code should verify: %v", err)
	}
}

func TestIssueRegisterEmailCode_ValidatesEmail(t *testing.T) {
	t.Parallel()

	svc, _ := newRegisterCodeService(t)
	if err := svc.IssueRegisterEmailCode(context.Background(), "not-an-email"); err == nil {
		t.Fatal("expected invalid email to be rejected")
	}

	restricted := NewServiceWithOptions(setupTestDB(t), ServiceOptions{
		MailSender:          &capturingMailSender{},
		AllowedEmailDomains: []string{"aliang.one"},
		SQLDialect:          "sqlite",
	})
	if err := restricted.IssueRegisterEmailCode(context.Background(), "a@gmail.com"); err == nil {
		t.Fatal("expected disallowed domain to be rejected")
	}
}

func TestVerifyRegisterEmailCode_ConsumesOnce(t *testing.T) {
	t.Parallel()

	svc, _ := newRegisterCodeService(t)
	ctx := context.Background()
	email := "once@example.com"
	if err := svc.IssueRegisterEmailCode(ctx, email); err != nil {
		t.Fatalf("issue: %v", err)
	}

	var code string
	if err := svc.db.QueryRow(`SELECT code FROM als_register_email_codes WHERE email = ?`, email).Scan(&code); err != nil {
		t.Fatalf("query code: %v", err)
	}

	if err := svc.VerifyRegisterEmailCode(ctx, email, strings.ToLower(code)); err != nil {
		t.Fatalf("first verify should pass (case-insensitive): %v", err)
	}
	if err := svc.VerifyRegisterEmailCode(ctx, email, code); err == nil {
		t.Fatal("second verify must fail (one-time consumption)")
	}
}

func TestVerifyRegisterEmailCode_WrongCodeLocksAfterAttempts(t *testing.T) {
	t.Parallel()

	svc, _ := newRegisterCodeService(t)
	ctx := context.Background()
	email := "bruteforce@example.com"
	if err := svc.IssueRegisterEmailCode(ctx, email); err != nil {
		t.Fatalf("issue: %v", err)
	}

	var realCode string
	if err := svc.db.QueryRow(`SELECT code FROM als_register_email_codes WHERE email = ?`, email).Scan(&realCode); err != nil {
		t.Fatalf("query code: %v", err)
	}

	for i := 0; i < registerCodeMaxAttempts; i++ {
		if err := svc.VerifyRegisterEmailCode(ctx, email, "WRONG"); err == nil {
			t.Fatalf("wrong code %d must fail", i+1)
		}
	}
	// 连错达到上限后，正确码也被作废（防拖库爆破）
	if err := svc.VerifyRegisterEmailCode(ctx, email, realCode); err == nil {
		t.Fatal("correct code must be invalidated after max wrong attempts")
	}
}

func TestVerifyRegisterEmailCode_Expired(t *testing.T) {
	t.Parallel()

	svc, _ := newRegisterCodeService(t)
	ctx := context.Background()
	email := "expired@example.com"
	if err := svc.IssueRegisterEmailCode(ctx, email); err != nil {
		t.Fatalf("issue: %v", err)
	}
	// 手动把过期时间拨到过去
	if _, err := svc.db.Exec(`UPDATE als_register_email_codes SET expires_at = ? WHERE email = ?`, "2000-01-01 00:00:00", email); err != nil {
		t.Fatalf("expire code: %v", err)
	}
	if err := svc.VerifyRegisterEmailCode(ctx, email, "ANYTHING"); err == nil {
		t.Fatal("expired code must be rejected")
	}
}

func TestVerifyRegisterEmailCode_UnknownEmail(t *testing.T) {
	t.Parallel()

	svc, _ := newRegisterCodeService(t)
	if err := svc.VerifyRegisterEmailCode(context.Background(), "ghost@example.com", "12345678"); err == nil {
		t.Fatal("unknown email must be rejected")
	}
}
