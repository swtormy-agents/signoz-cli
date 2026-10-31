package cli

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const testAPIKey = "signoz-test-key16" // 16 chars // gitleaks:allow

func testEnv(url string) func(string) string {
	return func(k string) string {
		switch k {
		case "SIGNOZ_URL":
			return url
		case "SIGNOZ_API_KEY":
			return testAPIKey
		}
		return ""
	}
}

func runCLI(t *testing.T, srv *httptest.Server, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = Execute(args, strings.NewReader(""), &out, &errOut, testEnv(srv.URL))
	return code, out.String(), errOut.String()
}

func TestHealthAndVersion(t *testing.T) {
	mux := http.NewServeMux()
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("SIGNOZ-API-KEY") != testAPIKey {
			http.Error(w, "nope", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"version":"0.99.0","setupCompleted":true}`))
	}
	mux.HandleFunc("/api/v1/version", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	code, out, _ := runCLI(t, srv, "health")
	if code != 0 || !strings.Contains(out, `"ok":true`) {
		t.Fatalf("health code=%d out=%s", code, out)
	}
	code, out, _ = runCLI(t, srv, "version")
	if code != 0 || !strings.Contains(out, "0.99.0") {
		t.Fatalf("version code=%d out=%s", code, out)
	}
}

func TestLogsQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/query_range", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("SIGNOZ-API-KEY") != testAPIKey {
			http.Error(w, "auth", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"status":"success","data":{"result":[]}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, out, _ := runCLI(t, srv, "logs", "query", "--filter", "severity_text = 'ERROR'")
	if code != 0 || !strings.Contains(out, "success") {
		t.Fatalf("logs query code=%d out=%s", code, out)
	}
}

func TestTracesQuery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v5/query_range", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"success"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, _, _ := runCLI(t, srv, "traces", "query")
	if code != 0 {
		t.Fatalf("traces query exit %d", code)
	}
}

func TestAlertsList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/rules", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"success","data":{"rules":[{"id":"1","name":"cpu"}]}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, out, _ := runCLI(t, srv, "alerts", "list")
	if code != 0 || !strings.Contains(out, "cpu") {
		t.Fatalf("alerts list code=%d out=%s", code, out)
	}
}

func TestServicesList(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/services/list", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`["frontend","api"]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, out, _ := runCLI(t, srv, "services", "list")
	if code != 0 || !strings.Contains(out, "frontend") {
		t.Fatalf("services list code=%d out=%s", code, out)
	}
}

func TestAPICall(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/rules", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"success"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, _, _ := runCLI(t, srv, "api", "GET", "/api/v3/rules")
	if code != 0 {
		t.Fatalf("api exit %d", code)
	}
}

func TestAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Unauthorized"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	code, _, errOut := runCLI(t, srv, "version")
	if code != 3 {
		t.Fatalf("want exit 3 got %d stderr=%s", code, errOut)
	}
}

func TestAuth401KeyNotInStderr(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("SIGNOZ-API-KEY")
		http.Error(w, `{"message":"invalid key `+key+`"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	code, _, errOut := runCLI(t, srv, "version")
	if code != 3 {
		t.Fatalf("want exit 3 got %d", code)
	}
	if strings.Contains(errOut, testAPIKey) {
		t.Fatalf("API key leaked to stderr: %s", errOut)
	}
	if !strings.Contains(errOut, "<API_KEY>") {
		t.Fatalf("expected redacted placeholder in stderr: %s", errOut)
	}
}

func TestDryRunOutputRedactsAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	code, out, _ := runCLI(t, srv, "health", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out, testAPIKey) {
		t.Fatalf("dry-run leaked key: %s", out)
	}
	if !strings.Contains(out, "<API_KEY>") {
		t.Fatalf("expected dry-run placeholder: %s", out)
	}
}

func TestReportErrorRedactsAPIKey(t *testing.T) {
	var errOut bytes.Buffer
	a := &app{apiKey: testAPIKey, errOut: &errOut}
	code := a.reportError(runtimeErr(fmt.Errorf("failure mentioning %s", testAPIKey)))
	if code != 1 {
		t.Fatalf("code=%d", code)
	}
	out := errOut.String()
	if strings.Contains(out, testAPIKey) {
		t.Fatalf("key leaked in reportError output: %s", out)
	}
}

func TestShortAPIKeyRejected(t *testing.T) {
	shortKey := "short-key-15c" // 15 characters // gitleaks:allow
	getenv := func(k string) string {
		switch k {
		case "SIGNOZ_URL":
			return "http://127.0.0.1:1"
		case "SIGNOZ_API_KEY":
			return shortKey
		}
		return ""
	}
	var errOut bytes.Buffer
	code := Execute([]string{"version"}, strings.NewReader(""), io.Discard, &errOut, getenv)
	if code != 2 {
		t.Fatalf("want exit 2 got %d: %s", code, errOut.String())
	}
}

func TestSkillPrint(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	code, out, _ := runCLI(t, srv, "skill", "print")
	if code != 0 || !strings.Contains(out, "name: signoz") {
		t.Fatalf("code=%d out=%s", code, out)
	}
}

func TestWriteRequiresWriteFlag(t *testing.T) {
	var errOut bytes.Buffer
	code := Execute([]string{"api", "POST", "/api/v2/rules", "--body", "{}"},
		strings.NewReader(""), io.Discard, &errOut, testEnv("http://example"))
	if code != 3 {
		t.Fatalf("want 3 got %d: %s", code, errOut.String())
	}
}

func TestAlertsListTableOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/rules", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"success","data":{"rules":[{"id":"1","name":"cpu","state":"inactive"}]}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	code, out, _ := runCLI(t, srv, "alerts", "list", "--output", "table")
	if code != 0 || !strings.Contains(out, "cpu") {
		t.Fatalf("table code=%d out=%s", code, out)
	}
}

func TestInvalidBaseURLs(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	cases := []struct {
		url  string
		code int
	}{
		{"ftp://example.com", 2},
		{"https://", 2},
		{"https://user:pass@host.example", 2},
	}
	for _, tc := range cases {
		getenv := func(k string) string {
			switch k {
			case "SIGNOZ_URL":
				return tc.url
			case "SIGNOZ_API_KEY":
				return testAPIKey
			}
			return ""
		}
		var errOut bytes.Buffer
		code := Execute([]string{"version"}, strings.NewReader(""), io.Discard, &errOut, getenv)
		if code != tc.code {
			t.Fatalf("url %q want exit %d got %d: %s", tc.url, tc.code, code, errOut.String())
		}
		errText := errOut.String()
		if strings.Contains(errText, "user:pass") || strings.Contains(errText, "pass@") {
			t.Fatalf("userinfo leaked in error for %q: %s", tc.url, errText)
		}
	}
}

func TestDryRunStripsURLUserinfo(t *testing.T) {
	getenv := func(k string) string {
		switch k {
		case "SIGNOZ_URL":
			return "https://user:secret@host.example"
		case "SIGNOZ_API_KEY":
			return testAPIKey
		}
		return ""
	}
	var out bytes.Buffer
	code := Execute([]string{"health", "--dry-run"}, strings.NewReader(""), &out, io.Discard, getenv)
	if code != 2 {
		t.Fatalf("want config error for userinfo URL, got %d", code)
	}
}

func TestDryRunNoUserinfoOnValidURL(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	getenv := func(k string) string {
		switch k {
		case "SIGNOZ_URL":
			return srv.URL
		case "SIGNOZ_API_KEY":
			return testAPIKey
		}
		return ""
	}
	var out bytes.Buffer
	code := Execute([]string{"health", "--dry-run"}, strings.NewReader(""), &out, io.Discard, getenv)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(out.String(), "@") {
		t.Fatalf("unexpected userinfo in dry-run: %s", out.String())
	}
}

func TestAPIPathValidation(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	for _, args := range [][]string{
		{"api", "GET", "/apix/foo"},
		{"api", "GET", "/api/../etc/passwd"},
	} {
		code, _, errOut := runCLI(t, srv, args...)
		if code != 2 {
			t.Fatalf("args %v want exit 2 got %d stderr=%s", args, code, errOut)
		}
	}
}

func TestSkillMarkdownIdentical(t *testing.T) {
	a, err := os.ReadFile("SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile("../../skills/signoz/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("SKILL.md files differ; run scripts/sync-skill-md.sh")
	}
	if string(a) != skillMD {
		t.Fatalf("embedded SKILL.md out of sync with internal/cli/SKILL.md")
	}
}
