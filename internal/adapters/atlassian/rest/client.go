package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/domain"
)

const bitbucketAPI = "https://api.bitbucket.org"

// Client is shared HTTP + Basic auth for Jira, Confluence, Bitbucket, and JSM.
// BaseURL overrides the site origin in tests (httptest.Server).
type Client struct {
	HTTP     *http.Client
	Store    auth.Store
	BaseURL  string
	BBBase   string
	AuthSite string // optional alias override for tests
}

func (c *Client) httpc() *http.Client {
	if c != nil && c.HTTP != nil {
		if c.HTTP.CheckRedirect == nil {
			c.HTTP.CheckRedirect = keepBasicAuthOnRedirect
		}
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: keepBasicAuthOnRedirect}
}

// keepBasicAuthOnRedirect copies Authorization onto same-host redirects.
// net/http strips it, which makes /search/jql return an empty anonymous page
// while /issue/{key} still works (often no extra hop).
func keepBasicAuthOnRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return http.ErrUseLastResponse
	}
	if len(via) == 0 {
		return nil
	}
	prev := via[len(via)-1]
	if req.URL.Host != prev.URL.Host {
		return nil
	}
	if req.Header.Get("Authorization") == "" {
		if auth := prev.Header.Get("Authorization"); auth != "" {
			req.Header.Set("Authorization", auth)
		}
	}
	return nil
}

func (c *Client) origin(hostname string) string {
	if c != nil && strings.TrimSpace(c.BaseURL) != "" {
		return strings.TrimRight(c.BaseURL, "/")
	}
	return "https://" + strings.TrimSpace(hostname)
}

func (c *Client) bitbucketOrigin() string {
	if c != nil && strings.TrimSpace(c.BBBase) != "" {
		return strings.TrimRight(c.BBBase, "/")
	}
	return bitbucketAPI
}

func (c *Client) credForHost(hostname string) (auth.Cred, error) {
	if c == nil || c.Store == nil {
		return auth.Cred{}, domain.Auth("not signed in").WithHint("atlas auth login --site ALIAS --email EMAIL --token TOKEN")
	}
	id := strings.TrimSpace(c.AuthSite)
	if id == "" {
		id = hostname
	}
	site, err := domain.Lookup(id)
	if err != nil {
		return auth.Cred{}, err
	}
	return c.credForAlias(site.Alias)
}

func (c *Client) credForAlias(alias string) (auth.Cred, error) {
	if c == nil || c.Store == nil {
		return auth.Cred{}, domain.Auth("not signed in").WithHint("atlas auth login --site ALIAS --email EMAIL --token TOKEN")
	}
	b, ok, err := c.Store.Get()
	if err != nil {
		return auth.Cred{}, err
	}
	if !ok || b.Sites == nil {
		return auth.Cred{}, domain.Auth("not signed in").WithHint("atlas auth login --site ALIAS --email EMAIL --token TOKEN")
	}
	cred, ok := b.Sites[alias]
	if !ok || cred.Email == "" || cred.Token == "" {
		return auth.Cred{}, domain.Auth("not signed in for " + alias).WithHint("atlas auth login --site " + alias + " --email EMAIL --token TOKEN")
	}
	return cred, nil
}

func (c *Client) workspaceCred(workspace string) (auth.Cred, error) {
	if c == nil {
		return auth.WorkspaceCred(nil, workspace)
	}
	return auth.WorkspaceCred(c.Store, workspace)
}

// maxJSONBody caps what doJSON reads from a response.
const maxJSONBody = 1 << 20

func (c *Client) doJSON(ctx context.Context, method, rawURL string, cred auth.Cred, extra http.Header, body any) (int, []byte, error) {
	code, b, _, err := c.doJSONLimit(ctx, method, rawURL, cred, extra, body, maxJSONBody)
	return code, b, err
}

// doJSONLimit is doJSON with a caller-chosen cap. complete is false when the
// body was longer than limit or the connection broke while reading it, so a
// caller that must not act on a partial body can refuse it.
func (c *Client) doJSONLimit(ctx context.Context, method, rawURL string, cred auth.Cred, extra http.Header, body any, limit int64) (code int, b []byte, complete bool, err error) {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, false, domain.Usage(err.Error())
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
	if err != nil {
		return 0, nil, false, domain.Usage(err.Error())
	}
	req.SetBasicAuth(cred.Email, cred.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	res, err := c.httpc().Do(req)
	if err != nil {
		return 0, nil, false, domain.Service(err.Error())
	}
	defer res.Body.Close()
	b, readErr := io.ReadAll(io.LimitReader(res.Body, limit+1))
	complete = readErr == nil && int64(len(b)) <= limit
	if int64(len(b)) > limit {
		b = b[:limit]
	}
	return res.StatusCode, b, complete, nil
}

func (c *Client) doBytes(ctx context.Context, method, rawURL string, cred auth.Cred, extra http.Header) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return 0, nil, domain.Usage(err.Error())
	}
	req.SetBasicAuth(cred.Email, cred.Token)
	req.Header.Set("Accept", "text/plain, application/json")
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}
	res, err := c.httpc().Do(req)
	if err != nil {
		return 0, nil, domain.Service(err.Error())
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	return res.StatusCode, b, nil
}

func classify(code int, kind, notFound string) error {
	switch code {
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.Auth("not authorized for this " + kind + " site")
	case http.StatusNotFound:
		return domain.NotFound(notFound)
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable:
		return domain.Service(kind + " service error")
	default:
		return domain.Service(kind + " service error")
	}
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

func q(raw string) string { return url.QueryEscape(raw) }

func textFromADF(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		if s, ok := t["text"].(string); ok {
			return s
		}
		var parts []string
		if c, ok := t["content"].([]any); ok {
			for _, e := range c {
				if s := textFromADF(e); s != "" {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "")
	case []any:
		var parts []string
		for _, e := range t {
			if s := textFromADF(e); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}

func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return s
}

func nestedName(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	n, _ := m[key].(map[string]any)
	return str(n, "name")
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asList(v any) []any {
	s, _ := v.([]any)
	return s
}

func mustJSON(b []byte) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if m == nil {
		m = map[string]any{}
	}
	return m
}
