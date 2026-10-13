// Package applehttp provides authenticated HTTP transport shared by Apple APIs.
package applehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxResponseBytes = 32 << 20

type ErrorDetail struct {
	ID     string          `json:"id,omitempty"`
	Status string          `json:"status,omitempty"`
	Code   string          `json:"code,omitempty"`
	Title  string          `json:"title,omitempty"`
	Detail string          `json:"detail,omitempty"`
	Source json.RawMessage `json:"source,omitempty"`
	Meta   json.RawMessage `json:"meta,omitempty"`
}

type APIError struct {
	HTTPStatusCode int
	ErrorCode      int64
	ErrorMessage   string
	Errors         []ErrorDetail
	Headers        http.Header
	RetryAfter     time.Time
	Body           []byte
}

func (e *APIError) Error() string {
	message := e.ErrorMessage
	if message == "" && len(e.Errors) > 0 {
		message = e.Errors[0].Title + ": " + e.Errors[0].Detail
	}
	return fmt.Sprintf("Apple API: HTTP %d, code %d: %s", e.HTTPStatusCode, e.ErrorCode, message)
}

type Client struct {
	HTTP  *http.Client
	Base  *url.URL
	Token func(context.Context) (string, error)
}

func New(base string, client *http.Client, token func(context.Context) (string, error)) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid API base URL")
	}
	if token == nil {
		return nil, errors.New("token provider is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	// Credentials must not follow redirects, even to another Apple API origin.
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{HTTP: &copyClient, Base: u, Token: token}, nil
}

// Resolve permits pagination URLs only within this client's configured origin.
func (c *Client) Resolve(path string, query url.Values) (*url.URL, error) {
	u, err := url.Parse(path)
	if err != nil || u.User != nil || u.Fragment != "" {
		return nil, errors.New("invalid API path")
	}
	if u.IsAbs() || u.Host != "" {
		if u.Scheme != c.Base.Scheme || u.Host != c.Base.Host {
			return nil, errors.New("API URL must belong to the configured origin")
		}
	} else if !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(path, "//") {
		return nil, errors.New("API path must be absolute")
	}
	u = c.Base.ResolveReference(u)
	q := u.Query()
	for key, values := range query {
		q[key] = values
	}
	u.RawQuery = q.Encode()
	return u, nil
}

func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, contentType string, output any) error {
	u, err := c.Resolve(path, query)
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		if raw, ok := body.([]byte); ok {
			reader = bytes.NewReader(raw)
		} else {
			data, err := json.Marshal(body)
			if err != nil {
				return fmt.Errorf("encode request: %w", err)
			}
			reader = bytes.NewReader(data)
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return err
	}
	token, err := c.Token(ctx)
	if err != nil {
		return fmt.Errorf("sign request: %w", err)
	}
	if token == "" {
		return errors.New("token provider returned an empty token")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(data) > maxResponseBytes {
		return errors.New("API response exceeds 32 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var payload struct {
			ErrorCode    int64         `json:"errorCode"`
			ErrorMessage string        `json:"errorMessage"`
			Errors       []ErrorDetail `json:"errors"`
		}
		_ = json.Unmarshal(data, &payload)
		e := &APIError{HTTPStatusCode: resp.StatusCode, ErrorCode: payload.ErrorCode, ErrorMessage: payload.ErrorMessage, Errors: payload.Errors, Headers: resp.Header.Clone(), Body: data}
		if raw := resp.Header.Get("Retry-After"); raw != "" {
			if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
				if n > 100000000000 {
					e.RetryAfter = time.UnixMilli(n)
				} else if n >= 0 {
					e.RetryAfter = time.Now().Add(time.Duration(n) * time.Second)
				}
			} else if t, err := http.ParseTime(raw); err == nil {
				e.RetryAfter = t
			}
		}
		return e
	}
	if output == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(data, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
