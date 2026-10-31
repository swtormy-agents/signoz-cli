package signoz

import (
	"strings"
	"testing"
)

func TestValidateBaseURL(t *testing.T) {
	ok, err := ValidateBaseURL("https://instance.signoz.cloud/")
	if err != nil || ok != "https://instance.signoz.cloud" {
		t.Fatalf("ok=%q err=%v", ok, err)
	}
	for _, raw := range []string{
		"ftp://example.com",
		"https://",
		"https:///path",
		"https://user:pass@host.example",
		"://no-scheme",
	} {
		if _, err := ValidateBaseURL(raw); err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestStripURLUserinfo(t *testing.T) {
	in := "https://user:pass@host.example/api"
	out := StripURLUserinfo(in)
	if strings.Contains(out, "user") || strings.Contains(out, "pass") {
		t.Fatalf("userinfo leaked: %q", out)
	}
	if !strings.Contains(out, "host.example") {
		t.Fatalf("host missing: %q", out)
	}
}