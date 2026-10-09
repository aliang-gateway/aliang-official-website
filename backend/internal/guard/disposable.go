package guard

import "strings"

// DefaultDisposableDomains is a curated blocklist of well-known disposable /
// throwaway email providers. Batch-registration bots rely on these to mint
// accounts at scale; the list favors breadth on the most abused providers
// over completeness and can be extended without code changes elsewhere.
var DefaultDisposableDomains = []string{
	"mailinator.com", "guerrillamail.com", "guerrillamail.net", "sharklasers.com",
	"10minutemail.com", "10minutemail.net", "temp-mail.org", "temp-mail.io",
	"tempmail.com", "tempmail.plus", "yopmail.com", "yopmail.fr",
	"trashmail.com", "trashmail.de", "throwawaymail.com", "getnada.com",
	"maildrop.cc", "mailnesia.com", "fakeinbox.com", "dispostable.com",
	"maildrop.com", "tempr.email", "discard.email", "mail-temporaire.fr",
	"mytemp.email", "mohmal.com", "emailondeck.com", "33mail.com",
	"spamgourmet.com", "burnermail.io", "minuteinbox.com", "instantemailaddress.com",
	"linshiyouxiang.net", "moakt.com", "moakt.co", "tmpmail.org",
	"tmpmail.net", "tmpmail.dev", "email-fake.com", "fakemail.net",
	"tempinbox.com", "disposablemail.com", "tempmailo.com", "mail-temp.com",
	"1secmail.com", "1secmail.net", "snapmail.cc", "mintemail.com",
	"cust.in", "mailsac.com",
}

// DisposableChecker reports whether an email domain (or a bare email
// address) resolves to a blocked domain. Matching is case-insensitive and
// covers subdomains of listed domains. An empty list blocks nothing.
type DisposableChecker struct {
	blocked map[string]struct{}
}

func NewDisposableChecker(domains []string) *DisposableChecker {
	blocked := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain != "" {
			blocked[domain] = struct{}{}
		}
	}
	return &DisposableChecker{blocked: blocked}
}

// NewDefaultDisposableChecker blocks the built-in disposable provider list.
func NewDefaultDisposableChecker() *DisposableChecker {
	return NewDisposableChecker(DefaultDisposableDomains)
}

func (c *DisposableChecker) IsBlocked(value string) bool {
	if len(c.blocked) == 0 {
		return false
	}

	domain := strings.ToLower(strings.TrimSpace(value))
	if idx := strings.LastIndex(domain, "@"); idx >= 0 {
		domain = domain[idx+1:]
	}
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return false
	}

	for {
		if _, ok := c.blocked[domain]; ok {
			return true
		}
		idx := strings.Index(domain, ".")
		if idx < 0 {
			return false
		}
		domain = domain[idx+1:]
	}
}
