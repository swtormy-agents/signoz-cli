package signoz

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidateBaseURL parses a SigNoz instance URL: http/https only, host required, no userinfo.
// Returns a normalized base URL string safe to log (no credentials).
func ValidateBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("SIGNOZ_URL is empty")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid SIGNOZ_URL: %w", err)
	}
	if u.User != nil {
		return "", fmt.Errorf("SIGNOZ_URL must not contain userinfo (credentials embedded in the URL are not allowed)")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", fmt.Errorf("SIGNOZ_URL scheme must be http or https, not %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("SIGNOZ_URL must include a host")
	}
	u.User = nil
	u.Fragment = ""
	u.RawQuery = ""
	out := strings.TrimRight(u.String(), "/")
	return out, nil
}

// StripURLUserinfo removes embedded credentials from URLs in text (for errors and dry-run).
func StripURLUserinfo(s string) string {
	u, err := url.Parse(s)
	if err != nil {
		return redactUserinfoSubstring(s)
	}
	if u.User != nil {
		withUser := u.String()
		u.User = nil
		without := strings.TrimRight(u.String(), "/")
		out := strings.ReplaceAll(s, withUser, without)
		return redactUserinfoSubstring(out)
	}
	return redactUserinfoSubstring(s)
}

func redactUserinfoSubstring(s string) string {
	// Catch userinfo when the whole string is not parseable as a single URL.
	if i := strings.Index(s, "://"); i >= 0 {
		rest := s[i+3:]
		if at := strings.Index(rest, "@"); at > 0 && strings.Contains(rest[:at], ":") {
			return s[:i+3] + rest[at+1:]
		}
	}
	return s
}
