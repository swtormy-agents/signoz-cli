package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/swtormy-agents/signoz-cli/internal/signoz"
)

const (
	exitRuntime = 1
	exitUsage   = 2
	exitAuth    = 3
	exitBlocked = 4
)

type exitError struct {
	code int
	kind string
	err  error
	api  *signoz.APIError
}

func (e *exitError) Error() string { return e.err.Error() }

func usageErr(format string, a ...any) error {
	return &exitError{code: exitUsage, kind: "usage", err: fmt.Errorf(format, a...)}
}

func runtimeErr(err error) error {
	return &exitError{code: exitRuntime, kind: "runtime", err: err}
}

func authErr(err error) error {
	return &exitError{code: exitAuth, kind: "auth", err: err}
}

func configErr(err error) error {
	return &exitError{code: exitUsage, kind: "config", err: err}
}

func blockedErr(err error) error {
	return &exitError{code: exitBlocked, kind: "blocked", err: err}
}

var Version = "dev"

func versionString() string {
	v := Version
	if v != "dev" {
		return v
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		rev, dirty := "", ""
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					dirty = "+dirty"
				}
			}
		}
		if len(rev) > 7 {
			rev = rev[:7]
		}
		if rev != "" {
			return "dev-" + rev + dirty
		}
	}
	return v
}

func staleHint(err error) string {
	m := err.Error()
	if strings.Contains(m, "unknown flag") || strings.Contains(m, "unknown command") || strings.Contains(m, "unknown shorthand") {
		return fmt.Sprintf(" (this is signoz %s; rebuild or `go install github.com/swtormy-agents/signoz-cli/cmd/signoz@feat/v0.1`)", versionString())
	}
	return ""
}

type app struct {
	in       io.Reader
	out      io.Writer
	errOut   io.Writer
	getenv   func(string) string
	dryRun   bool
	jq       string
	pretty   bool
	output   string
	baseURL  string
	timeout  time.Duration
	apiKey   string
}

func Execute(args []string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string) int {
	a := &app{in: stdin, out: stdout, errOut: stderr, getenv: getenv, output: "json"}
	root := a.rootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		if _, ok := err.(*exitError); !ok {
			err = usageErr("%s%s", err, staleHint(err))
		}
		return a.reportError(err)
	}
	return 0
}

func (a *app) reportError(err error) int {
	ee, ok := err.(*exitError)
	if !ok {
		ee = &exitError{code: exitUsage, kind: "usage", err: err}
	}
	detail := map[string]any{"type": ee.kind, "message": a.redact(ee.err.Error())}
	if ee.api != nil {
		detail["status"] = ee.api.Status
		if ee.api.Message != "" {
			detail["message"] = a.redact(ee.api.Message)
		}
	}
	b, _ := marshal(map[string]any{"ok": false, "error": detail})
	fmt.Fprintln(a.errOut, string(b))
	return ee.code
}

func (a *app) redact(s string) string {
	return signoz.StripURLUserinfo((&signoz.Client{APIKey: a.apiKey}).Redact(s))
}

func (a *app) validateAPIKey() error {
	if len(a.apiKey) < signoz.MinAPIKeyLength {
		return configErr(fmt.Errorf(
			"SIGNOZ_API_KEY must be at least %d characters (shorter values are rejected because they cannot be redacted safely from logs)",
			signoz.MinAPIKeyLength,
		))
	}
	return nil
}

func (a *app) resolveAPIKey() error {
	if a.apiKey != "" {
		return a.validateAPIKey()
	}
	a.apiKey = strings.TrimSpace(a.getenv("SIGNOZ_API_KEY"))
	if a.apiKey != "" {
		return a.validateAPIKey()
	}
	rd := bufio.NewReader(a.in)
	line, err := rd.ReadString('\n')
	if err != nil && line == "" {
		return authErr(errors.New("SIGNOZ_API_KEY is not set; export it or pipe the API key on stdin"))
	}
	a.apiKey = strings.TrimSpace(line)
	if a.apiKey == "" {
		return authErr(errors.New("empty API key on stdin"))
	}
	return a.validateAPIKey()
}

func (a *app) client(ctx context.Context) (*signoz.Client, error) {
	if err := a.resolveAPIKey(); err != nil {
		return nil, err
	}
	base := a.baseURL
	if base == "" {
		base = strings.TrimSpace(a.getenv("SIGNOZ_URL"))
	}
	if base == "" {
		return nil, authErr(errors.New("SIGNOZ_URL is not set (example: https://your-instance.signoz.cloud)"))
	}
	normalized, err := signoz.ValidateBaseURL(base)
	if err != nil {
		return nil, configErr(err)
	}
	return &signoz.Client{
		BaseURL: normalized,
		APIKey:  a.apiKey,
		HTTP:    &http.Client{Timeout: a.timeout},
	}, nil
}

func (a *app) wrapAPI(err error) error {
	var api *signoz.APIError
	if errors.As(err, &api) {
		code, kind := exitRuntime, "api"
		if api.IsAuth() {
			code, kind = exitAuth, "auth"
		}
		return &exitError{code: code, kind: kind, err: api, api: api}
	}
	return runtimeErr(err)
}

func (a *app) rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "signoz",
		Short:         "SigNoz REST API from the shell (unofficial, community-built)",
		Long:          rootLong,
		Version:       versionString(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageErr("%s%s", err, staleHint(err))
	})
	pf := root.PersistentFlags()
	pf.BoolVar(&a.dryRun, "dry-run", false, "print the HTTP request instead of sending it")
	pf.StringVar(&a.jq, "jq", "", "filter JSON output with a jq expression")
	pf.BoolVar(&a.pretty, "pretty", false, "indent JSON output")
	pf.StringVar(&a.output, "output", "json", "output format: json or table")
	pf.StringVar(&a.baseURL, "url", "", "SigNoz base URL (default $SIGNOZ_URL)")
	pf.DurationVar(&a.timeout, "http-timeout", 60*time.Second, "HTTP timeout per request")

	root.AddCommand(
		a.healthCmd(),
		a.serverVersionCmd(),
		a.logsCmd(),
		a.tracesCmd(),
		a.alertsCmd(),
		a.servicesCmd(),
		a.apiCmd(),
		a.skillCmd(),
	)
	return root
}

const rootLong = `signoz calls the SigNoz REST API from the shell. JSON on stdout by default; errors on stderr.

Unofficial, community-built — not affiliated with SigNoz.

Setup
  export SIGNOZ_URL=https://your-instance.signoz.cloud
  export SIGNOZ_API_KEY=...          service account API key (never pass on the command line)

Mutating api calls need --write, an interactive terminal, and typing "write" when prompted.

Exit codes: 0 ok, 1 runtime/API, 2 usage or config, 3 auth, 4 blocked.`
