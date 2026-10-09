package guard

import "testing"

func TestDisposableCheckerBlocksListedDomains(t *testing.T) {
	t.Parallel()

	c := NewDisposableChecker([]string{"mailinator.com", "yopmail.com"})
	if !c.IsBlocked("mailinator.com") {
		t.Fatal("expected listed domain to be blocked")
	}
	if !c.IsBlocked("YOPMAIL.COM") {
		t.Fatal("expected match to be case-insensitive")
	}
}

func TestDisposableCheckerBlocksSubdomainsOfListedDomains(t *testing.T) {
	t.Parallel()

	c := NewDisposableChecker([]string{"mailinator.com"})
	if !c.IsBlocked("spam.mailinator.com") {
		t.Fatal("expected subdomain of a listed domain to be blocked")
	}
}

func TestDisposableCheckerAllowsUnknownAndSimilarDomains(t *testing.T) {
	t.Parallel()

	c := NewDisposableChecker([]string{"mailinator.com"})
	for _, domain := range []string{"gmail.com", "aliang.one", "notmailinator.com", "mailinator.com.evil.example"} {
		if c.IsBlocked(domain) {
			t.Fatalf("did not expect %q to be blocked", domain)
		}
	}
}

func TestDisposableCheckerEmptyListBlocksNothing(t *testing.T) {
	t.Parallel()

	c := NewDisposableChecker(nil)
	if c.IsBlocked("mailinator.com") {
		t.Fatal("empty list must block nothing")
	}
}

func TestDisposableCheckerHandlesEmailAddressesAndWhitespace(t *testing.T) {
	t.Parallel()

	c := NewDisposableChecker([]string{"mailinator.com"})
	if !c.IsBlocked(" someone@mailinator.com ") {
		t.Fatal("expected bare email input to resolve to its domain")
	}
}
