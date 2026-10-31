// Package signoz is a minimal SigNoz REST API client (community-built; not affiliated with SigNoz).
package signoz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const MinAPIKeyLength = 16

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

type APIError struct {
	Status  int
	Message string
	Body    json.RawMessage
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("signoz HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("signoz HTTP %d", e.Status)
}

func (e *APIError) IsAuth() bool { return e.Status == 401 || e.Status == 403 }

func (c *Client) base() string {
	return strings.TrimRight(c.BaseURL, "/")
}

func (c *Client) http() *http.Client {
	if c.HTTP == nil {
		return http.DefaultClient
	}
	return c.HTTP
}

func (c *Client) Redact(s string) string {
	if c.APIKey == "" || len(c.APIKey) < MinAPIKeyLength {
		return s
	}
	return strings.ReplaceAll(s, c.APIKey, "<API_KEY>")
}

func (c *Client) newRequest(ctx context.Context, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	u := c.base() + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, errors.New(c.Redact(err.Error()))
	}
	req.Header.Set("SIGNOZ-API-KEY", c.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body io.Reader) (json.RawMessage, http.Header, error) {
	req, err := c.newRequest(ctx, method, path, query, body)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request %s %s failed: %s", method, path, c.Redact(err.Error()))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("read response: %s", c.Redact(err.Error()))
	}
	if resp.StatusCode >= 400 {
		msg := c.Redact(parseMessage(data))
		return nil, resp.Header, &APIError{Status: resp.StatusCode, Message: msg, Body: data}
	}
	if len(data) == 0 {
		return json.RawMessage("null"), resp.Header, nil
	}
	if !json.Valid(data) {
		return nil, resp.Header, &APIError{Status: resp.StatusCode, Message: "non-JSON response"}
	}
	return json.RawMessage(data), resp.Header, nil
}

func parseMessage(data []byte) string {
	var wrap struct {
		Status string `json:"status"`
		Data   json.RawMessage
		Errors []struct {
			Msg string `json:"msg"`
		} `json:"errors"`
	}
	if json.Unmarshal(data, &wrap) == nil {
		if len(wrap.Errors) > 0 && wrap.Errors[0].Msg != "" {
			return wrap.Errors[0].Msg
		}
	}
	var m struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &m) == nil {
		if m.Message != "" {
			return m.Message
		}
		if m.Error != "" {
			return m.Error
		}
	}
	return strings.TrimSpace(string(data))
}

func (c *Client) Get(ctx context.Context, path string, query url.Values) (json.RawMessage, http.Header, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil)
}

func (c *Client) JSON(ctx context.Context, method, path string, v any) (json.RawMessage, http.Header, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, nil, err
	}
	return c.Do(ctx, method, path, nil, bytes.NewReader(b))
}

func DryRun(method, baseURL, path string, query url.Values, body any) map[string]any {
	u := strings.TrimRight(baseURL, "/") + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	out := map[string]any{
		"method": method,
		"url":    u,
		"header": map[string]string{"SIGNOZ-API-KEY": "<API_KEY>"},
	}
	if body != nil {
		out["body"] = body
	}
	return out
}
