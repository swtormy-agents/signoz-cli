package signoz

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	key := "signoz-api-key-16" // 16 chars // gitleaks:allow
	c := &Client{APIKey: key}
	in := "error: invalid " + key + " token"
	out := c.Redact(in)
	if out == in {
		t.Fatal("expected redaction")
	}
	if strings.Contains(out, key) {
		t.Fatal("key still present")
	}
}
