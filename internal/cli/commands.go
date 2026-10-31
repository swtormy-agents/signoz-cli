package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/swtormy-agents/signoz-cli/internal/signoz"
)

func (a *app) healthCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Verify SIGNOZ_URL and API key (GET /api/v1/version)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			path := "/api/v1/version"
			if a.dryRun {
				return a.emitValue(map[string]any{
					"request": signoz.DryRun(http.MethodGet, c.BaseURL, path, nil, nil),
					"mode":    "read-only",
				})
			}
			raw, _, err := c.Get(ctx, path, nil)
			if err != nil {
				return a.wrapAPI(err)
			}
			var server json.RawMessage = raw
			return a.emitValue(map[string]any{"ok": true, "url": c.BaseURL, "server": server})
		},
	}
}

func (a *app) serverVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "SigNoz server version (GET /api/v1/version)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.simpleGet("/api/v1/version", nil)(cmd, args)
		},
	}
}

func (a *app) simpleGet(path string, query map[string]string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		c, err := a.client(ctx)
		if err != nil {
			return err
		}
		if a.dryRun {
			return a.emitValue(signoz.DryRun(http.MethodGet, c.BaseURL, path, urlValues(query), nil))
		}
		raw, _, err := c.Get(ctx, path, urlValues(query))
		if err != nil {
			return a.wrapAPI(err)
		}
		return a.emit(raw)
	}
}

func (a *app) logsCmd() *cobra.Command {
	logs := &cobra.Command{Use: "logs", Short: "Logs queries (POST /api/v5/query_range)"}
	var filter string
	var startMs, endMs int64
	var limit, offset int
	query := &cobra.Command{
		Use:   "query",
		Short: "Search raw log records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runBuilderQuery(cmd, "logs", filter, startMs, endMs, limit, offset)
		},
	}
	query.Flags().StringVar(&filter, "filter", "", "filter expression (example: severity_text = 'ERROR')")
	query.Flags().Int64Var(&startMs, "start", 0, "start time Unix milliseconds")
	query.Flags().Int64Var(&endMs, "end", 0, "end time Unix milliseconds")
	query.Flags().IntVar(&limit, "limit", 50, "max rows")
	query.Flags().IntVar(&offset, "offset", 0, "offset")
	logs.AddCommand(query)
	return logs
}

func (a *app) tracesCmd() *cobra.Command {
	traces := &cobra.Command{Use: "traces", Short: "Trace queries (POST /api/v5/query_range)"}
	var filter string
	var startMs, endMs int64
	var limit, offset int
	query := &cobra.Command{
		Use:   "query",
		Short: "Search raw span records",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.runBuilderQuery(cmd, "traces", filter, startMs, endMs, limit, offset)
		},
	}
	query.Flags().StringVar(&filter, "filter", "", "filter expression (example: service.name = 'frontend')")
	query.Flags().Int64Var(&startMs, "start", 0, "start time Unix milliseconds")
	query.Flags().Int64Var(&endMs, "end", 0, "end time Unix milliseconds")
	query.Flags().IntVar(&limit, "limit", 50, "max rows")
	query.Flags().IntVar(&offset, "offset", 0, "offset")
	traces.AddCommand(query)
	return traces
}

func (a *app) runBuilderQuery(cmd *cobra.Command, signal, filter string, startMs, endMs int64, limit, offset int) error {
	ctx := cmd.Context()
	c, err := a.client(ctx)
	if err != nil {
		return err
	}
	start, end, err := resolveRange(startMs, endMs)
	if err != nil {
		return usageErr("%v", err)
	}
	body := signoz.BuilderRawQuery(signal, filter, start, end, limit, offset)
	path := "/api/v5/query_range"
	if a.dryRun {
		return a.emitValue(signoz.DryRun(http.MethodPost, c.BaseURL, path, nil, body))
	}
	raw, _, err := c.JSON(ctx, http.MethodPost, path, body)
	if err != nil {
		return a.wrapAPI(err)
	}
	return a.emit(raw)
}

func resolveRange(startMs, endMs int64) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	if endMs == 0 {
		endMs = now.UnixMilli()
	}
	if startMs == 0 {
		startMs = endMs - 15*60*1000
	}
	if startMs >= endMs {
		return time.Time{}, time.Time{}, fmt.Errorf("--start must be before --end")
	}
	return time.UnixMilli(startMs).UTC(), time.UnixMilli(endMs).UTC(), nil
}

func (a *app) alertsCmd() *cobra.Command {
	alerts := &cobra.Command{Use: "alerts", Short: "Alert rules"}
	var limit, offset int
	var query, sort, order string
	list := &cobra.Command{
		Use:   "list",
		Short: "List alert rules (GET /api/v3/rules)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			q := map[string]string{}
			if query != "" {
				q["query"] = query
			}
			if sort != "" {
				q["sort"] = sort
			}
			if order != "" {
				q["order"] = order
			}
			if cmd.Flags().Changed("limit") {
				q["limit"] = fmt.Sprintf("%d", limit)
			}
			if cmd.Flags().Changed("offset") {
				q["offset"] = fmt.Sprintf("%d", offset)
			}
			return a.simpleGet("/api/v3/rules", q)(cmd, args)
		},
	}
	list.Flags().StringVar(&query, "query", "", "filter DSL")
	list.Flags().StringVar(&sort, "sort", "", "sort field (updated_at, created_at, name, state, severity)")
	list.Flags().StringVar(&order, "order", "", "asc or desc")
	list.Flags().IntVar(&limit, "limit", 50, "page size")
	list.Flags().IntVar(&offset, "offset", 0, "offset")
	alerts.AddCommand(list)
	return alerts
}

func (a *app) servicesCmd() *cobra.Command {
	services := &cobra.Command{Use: "services", Short: "APM services"}
	list := &cobra.Command{
		Use:   "list",
		Short: "List service names (GET /api/v1/services/list)",
		Args:  cobra.NoArgs,
		RunE:  a.simpleGet("/api/v1/services/list", nil),
	}
	services.AddCommand(list)
	return services
}

func (a *app) apiCmd() *cobra.Command {
	var bodyJSON string
	var writeFlag bool
	cmd := &cobra.Command{
		Use:   "api <METHOD> <path>",
		Short: "Call a SigNoz API path (escape hatch)",
		Long:  "Paths must start with /api/. Mutating methods need --write and interactive confirmation.",
		Example: `  signoz api GET /api/v3/rules?limit=5
  signoz logs query --filter "severity_text = 'ERROR'"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			pathOnly, q, err := normalizeAPIPath(args[1])
			if err != nil {
				return err
			}
			if err := checkAPIAllowed(method, pathOnly); err != nil {
				return err
			}
			if apiNeedsWrite(method) {
				if err := a.requireHumanWrite(cmd, "api "+method+" "+pathOnly); err != nil {
					return err
				}
			}
			ctx := cmd.Context()
			c, err := a.client(ctx)
			if err != nil {
				return err
			}
			if a.dryRun {
				var body any
				if bodyJSON != "" {
					_ = json.Unmarshal([]byte(bodyJSON), &body)
				}
				return a.emitValue(signoz.DryRun(method, c.BaseURL, pathOnly, q, body))
			}
			var raw json.RawMessage
			if bodyJSON != "" {
				var body any
				if err := json.Unmarshal([]byte(bodyJSON), &body); err != nil {
					return usageErr("--body: %v", err)
				}
				raw, _, err = c.JSON(ctx, method, pathOnly, body)
			} else {
				raw, _, err = c.Do(ctx, method, pathOnly, q, nil)
			}
			if err != nil {
				return a.wrapAPI(err)
			}
			return a.emit(raw)
		},
	}
	cmd.Flags().StringVar(&bodyJSON, "body", "", "JSON body for POST/PUT/PATCH")
	bindWriteFlag(cmd, &writeFlag)
	return cmd
}

func normalizeAPIPath(raw string) (string, url.Values, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, usageErr("path is required")
	}
	if raw[0] != '/' {
		raw = "/" + raw
	}
	pathPart := raw
	var q url.Values
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		pathPart = u.Path
		q = u.Query()
	} else if i := strings.Index(raw, "?"); i >= 0 {
		pathPart = raw[:i]
		if u, err := url.Parse("http://dummy" + raw); err == nil {
			q = u.Query()
		}
	}
	if strings.Contains(pathPart, "..") {
		return "", nil, usageErr("path must not contain ..")
	}
	cleaned := path.Clean(pathPart)
	if !strings.HasPrefix(cleaned, "/api/") {
		return "", nil, usageErr("path must start with /api/")
	}
	return cleaned, q, nil
}

func checkAPIAllowed(method, pathOnly string) error {
	p := strings.ToLower(pathOnly)
	if method == http.MethodDelete && strings.Contains(p, "/rules/") {
		return blockedErr(fmt.Errorf("api %s %s is blocked in v0.1; confirm manually in the UI", method, pathOnly))
	}
	return nil
}

func apiNeedsWrite(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

var isTerminal = func(r ioReader) bool {
	f, ok := r.(*os.File)
	return ok && isTerminalFile(f)
}

type ioReader interface {
	Read(p []byte) (n int, err error)
}
