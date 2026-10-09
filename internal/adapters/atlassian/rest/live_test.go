package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/adapters/keychain"
	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestMain(m *testing.M) {
	domain.SetCatalog(domain.Catalog{
		Sites: []domain.Site{
			{Alias: "dev", Hostname: "dev.example.atlassian.net", UUID: "u1", Role: domain.RoleLicensed},
			{Alias: "helpdesk", Hostname: "helpdesk.example.atlassian.net", UUID: "u2", Role: domain.RoleJSMCustomer},
		},
		Projects: map[string]string{"ABC": "dev"},
		Spaces:   map[string]string{"DOCS": "dev"},
		Defaults: domain.Defaults{Workspace: "ws", JSMSite: "helpdesk"},
	})
	m.Run()
}

func liveClient(t *testing.T, h http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	store := &keychain.Fake{}
	_ = auth.PutSite(store, "dev", auth.Cred{Email: "a@b.c", Token: "tok"})
	_ = auth.PutSite(store, "helpdesk", auth.Cred{Email: "a@b.c", Token: "tok"})
	c := &Client{HTTP: srv.Client(), Store: store, BaseURL: srv.URL, BBBase: srv.URL}
	return c, srv
}

type handlerTransport struct{ handler http.Handler }

func (h handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	w := httptest.NewRecorder()
	h.handler.ServeHTTP(w, r)
	return w.Result(), nil
}

func noSocketClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	store := &keychain.Fake{}
	if err := auth.PutSite(store, "dev", auth.Cred{Email: "a@b.c", Token: "tok"}); err != nil {
		t.Fatal(err)
	}
	return &Client{HTTP: &http.Client{Transport: handlerTransport{handler: h}}, Store: store, BaseURL: "https://dev.example.atlassian.net"}
}

func TestKeepBasicAuthOnSameHostRedirect(t *testing.T) {
	sawAuth := false
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("/from", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/to", http.StatusFound)
	})
	mux.HandleFunc("/to", func(w http.ResponseWriter, r *http.Request) {
		_, p, ok := r.BasicAuth()
		sawAuth = ok && p == "tok"
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	store := &keychain.Fake{}
	_ = auth.PutSite(store, "dev", auth.Cred{Email: "a@b.c", Token: "tok"})
	c := &Client{Store: store, BaseURL: srv.URL}
	code, _, err := c.doJSON(context.Background(), http.MethodGet, srv.URL+"/from", auth.Cred{Email: "a@b.c", Token: "tok"}, nil, nil)
	if err != nil || code != http.StatusOK {
		t.Fatalf("%d %v", code, err)
	}
	if !sawAuth {
		t.Fatal("authorization dropped on redirect")
	}
}

func TestJiraGetLive(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/ABC-1" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if u, p, ok := r.BasicAuth(); !ok || u != "a@b.c" || p != "tok" {
			t.Fatal("basic")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"key": "ABC-1",
			"fields": map[string]any{
				"summary":   "hello",
				"status":    map[string]any{"name": "To Do"},
				"issuetype": map[string]any{"name": "Task"},
				"project":   map[string]any{"key": "ABC"},
			},
		})
	}))
	iss, err := Jira{Client: c}.Get(context.Background(), "dev.example.atlassian.net", "ABC-1")
	if err != nil {
		t.Fatal(err)
	}
	if iss.Key != "ABC-1" || iss.Summary != "hello" || iss.Site != "dev.example.atlassian.net" {
		t.Fatalf("%+v", iss)
	}
	if iss.BrowseURL != "https://dev.example.atlassian.net/browse/ABC-1" {
		t.Fatal(iss.BrowseURL)
	}
}

func TestJiraSearchUsersUsesAssignableScopeAndReturnsAccountIDs(t *testing.T) {
	for _, tc := range []struct{ project, issue, path, parameter, value string }{
		{"SDO", "", "/rest/api/3/user/assignable/search", "project", "SDO"},
		{"", "SDO-588", "/rest/api/3/user/assignable/search", "issueKey", "SDO-588"},
		{"", "", "/rest/api/3/user/search", "", ""},
	} {
		t.Run(tc.path+tc.parameter, func(t *testing.T) {
			c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tc.path || r.URL.Query().Get("query") != "Alex Example" {
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
				}
				if tc.parameter != "" && r.URL.Query().Get(tc.parameter) != tc.value {
					t.Fatalf("scope %s", r.URL.RawQuery)
				}
				_, _ = w.Write([]byte(`[{"accountId":"acct-alex","displayName":"Alex Example","active":true},{"accountId":"acct-hidden","displayName":"Hidden Email","active":true}]`))
			}))
			result, err := (Jira{Client: c}).SearchUsers(context.Background(), "dev.example.atlassian.net", "Alex Example", tc.project, tc.issue)
			if err != nil || result.Count != 2 || result.Users[0].AccountID != "acct-alex" || result.Users[1].Email != "" {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func TestJiraEditAssigneeSendsAccountID(t *testing.T) {
	c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if id := str(asMap(asMap(payload["fields"])["assignee"]), "accountId"); id != "acct-alex" {
				t.Fatalf("account ID %q", id)
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "ABC-1", "fields": map[string]any{
			"project": map[string]any{"key": "ABC"}, "assignee": map[string]any{"accountId": "acct-alex"},
		}})
	}))
	_, err := (Jira{Client: c}).Edit(context.Background(), "dev.example.atlassian.net", "ABC-1", map[string]any{"assignee": map[string]any{"accountId": "acct-alex"}}, false)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJiraGetUnauthorized(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	_, err := Jira{Client: c}.Get(context.Background(), "dev.example.atlassian.net", "ABC-1")
	if domain.ClassOf(err) != domain.ClassAuth {
		t.Fatal(err)
	}
}

func TestJiraGetNotFound(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	_, err := Jira{Client: c}.Get(context.Background(), "dev.example.atlassian.net", "ABC-9")
	if domain.ClassOf(err) != domain.ClassNotFound {
		t.Fatal(err)
	}
}

func TestJiraSearchPostsJQLAndFields(t *testing.T) {
	var method, path, rawQuery string
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, rawQuery = r.Method, r.URL.Path, r.URL.RawQuery
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issues": []any{map[string]any{
				"key": "ABC-1",
				"fields": map[string]any{
					"summary":   "hello",
					"status":    map[string]any{"name": "To Do"},
					"issuetype": map[string]any{"name": "Task"},
					"project":   map[string]any{"key": "ABC"},
				},
			}},
		})
	}))
	page, err := Jira{Client: c}.Search(context.Background(), "dev.example.atlassian.net", "project = ABC")
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodPost || path != "/rest/api/3/search/jql" {
		t.Fatalf("%s %s %s", method, path, rawQuery)
	}
	if page.Count != 1 || page.Items[0].Key != "ABC-1" || page.Items[0].Summary != "hello" {
		t.Fatalf("%+v", page)
	}
}

func TestJiraSearchEmptyGETFallsBackToPOST(t *testing.T) {
	var methods []string
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method+" "+r.URL.Path)
		if r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/search/jql" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issues": []any{map[string]any{
					"key": "ABC-1",
					"fields": map[string]any{
						"summary":   "from-post",
						"status":    map[string]any{"name": "To Do"},
						"issuetype": map[string]any{"name": "Task"},
						"project":   map[string]any{"key": "ABC"},
					},
				}},
			})
			return
		}
		t.Fatal(r.Method, r.URL.Path)
	}))
	page, err := Jira{Client: c}.Search(context.Background(), "dev.example.atlassian.net", "project = ABC")
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 1 || page.Items[0].Summary != "from-post" {
		t.Fatalf("%v %+v", methods, page)
	}
}

func TestJiraSearchHydratesIDOnlyHits(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/rest/api/3/search/jql":
			_ = json.NewEncoder(w).Encode(map[string]any{"issues": []any{map[string]any{"id": "10001"}}})
		case strings.Contains(r.URL.Path, "/rest/api/3/issue/"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"key": "ABC-1",
				"fields": map[string]any{
					"summary":   "hello",
					"status":    map[string]any{"name": "To Do"},
					"issuetype": map[string]any{"name": "Task"},
					"project":   map[string]any{"key": "ABC"},
				},
			})
		default:
			t.Fatal(r.URL.Path)
		}
	}))
	page, err := Jira{Client: c}.Search(context.Background(), "dev.example.atlassian.net", "project = ABC")
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 1 || page.Items[0].Key != "ABC-1" || page.Items[0].Summary != "hello" {
		t.Fatalf("%+v", page)
	}
}

func TestJiraCreateDryRunDoesNotPOST(t *testing.T) {
	hit := false
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusTeapot)
	}))
	iss, err := Jira{Client: c}.Create(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{
		Project: "ABC", IssueType: "Task", Summary: "n",
	}, true)
	if err != nil || hit || iss.Summary != "n" {
		t.Fatalf("%v %v %+v", err, hit, iss)
	}
}

func TestJiraSubtaskCreateAndEditSendParent(t *testing.T) {
	var writes []string
	c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			parent := str(asMap(asMap(payload["fields"])["parent"]), "key")
			writes = append(writes, r.Method+" "+parent)
			if r.Method == http.MethodPost {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"key":"ABC-2"}`))
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		case http.MethodGet:
			if !strings.Contains(r.URL.Query().Get("fields"), "parent") {
				t.Fatal("parent not requested")
			}
			parent := "ABC-1"
			if len(writes) == 2 {
				parent = "ABC-3"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"key": "ABC-2", "fields": map[string]any{
				"summary": "Investigate", "issuetype": map[string]any{"name": "Sub-task"},
				"project": map[string]any{"key": "ABC"}, "parent": map[string]any{"key": parent},
			}})
		default:
			t.Fatal(r.Method)
		}
	}))
	j := Jira{Client: c}
	created, err := j.Create(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{
		Project: "ABC", IssueType: "Sub-task", Summary: "Investigate", Parent: "ABC-1",
	}, false)
	if err != nil || created.Parent != "ABC-1" {
		t.Fatalf("%+v %v", created, err)
	}
	edited, err := j.Edit(context.Background(), "dev.example.atlassian.net", "ABC-2", map[string]any{"parent": map[string]any{"key": "ABC-3"}}, false)
	if err != nil || edited.Parent != "ABC-3" {
		t.Fatalf("%+v %v", edited, err)
	}
	if len(writes) != 2 || writes[0] != "POST ABC-1" || writes[1] != "PUT ABC-3" {
		t.Fatalf("writes %v", writes)
	}
}

func TestJiraSubtaskReparentDetectsUnchangedParent(t *testing.T) {
	c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"key": "ABC-2", "fields": map[string]any{
			"project": map[string]any{"key": "ABC"}, "parent": map[string]any{"key": "ABC-1"},
		}})
	}))
	_, err := (Jira{Client: c}).Edit(context.Background(), "dev.example.atlassian.net", "ABC-2", map[string]any{"parent": map[string]any{"key": "ABC-3"}}, false)
	if domain.ClassOf(err) != domain.ClassService {
		t.Fatalf("expected service error for unchanged parent, got %v", err)
	}
}

func TestJiraCommentPostsADF(t *testing.T) {
	var payload map[string]any
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatal(r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{}`))
	}))
	if err := (Jira{Client: c}).Comment(context.Background(), "dev.example.atlassian.net", "ABC-1", "hi", false); err != nil {
		t.Fatal(err)
	}
	if asMap(payload["body"])["type"] != "doc" {
		t.Fatalf("%v", payload)
	}
}

func TestJiraInternalCommentUsesJSMRequestAPI(t *testing.T) {
	var calls []string
	var payload map[string]any
	c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case "GET /rest/servicedeskapi/request/ABC-1":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"issueKey":"ABC-1"}`))
		case "POST /rest/servicedeskapi/request/ABC-1/comment":
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"42","public":false}`))
		default:
			t.Fatal(r.Method, r.URL.Path)
		}
	}))
	if err := (Jira{Client: c}).CommentInternal(context.Background(), "dev.example.atlassian.net", "ABC-1", "private update", false); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != "GET /rest/servicedeskapi/request/ABC-1" || calls[1] != "POST /rest/servicedeskapi/request/ABC-1/comment" {
		t.Fatalf("calls %v", calls)
	}
	if payload["public"] != false || payload["body"] != "private update" {
		t.Fatalf("payload %v", payload)
	}
}

func TestJiraInternalCommentPreflightFailureDoesNotPost(t *testing.T) {
	for _, tc := range []struct {
		name  string
		code  int
		class string
	}{
		{"not a customer request", http.StatusNotFound, domain.ClassNotFound},
		{"no access", http.StatusForbidden, domain.ClassAuth},
	} {
		t.Run(tc.name, func(t *testing.T) {
			posts := 0
			c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
				}
				w.WriteHeader(tc.code)
			}))
			err := (Jira{Client: c}).CommentInternal(context.Background(), "dev.example.atlassian.net", "ABC-1", "private update", false)
			if domain.ClassOf(err) != tc.class || posts != 0 {
				t.Fatalf("error %v, posts %d", err, posts)
			}
		})
	}
}

func TestJiraInternalCommentDetectsVisibilityMismatch(t *testing.T) {
	for _, response := range []string{`{"public":true}`, `{}`} {
		t.Run(response, func(t *testing.T) {
			c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodGet {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(response))
			}))
			err := (Jira{Client: c}).CommentInternal(context.Background(), "dev.example.atlassian.net", "ABC-1", "private update", false)
			if domain.ClassOf(err) != domain.ClassService || !strings.Contains(err.Error(), "visibility") && !strings.Contains(err.Error(), "public") {
				t.Fatal(err)
			}
		})
	}
}

func TestJiraInternalCommentDryRunDoesNotCallSite(t *testing.T) {
	hit := false
	c := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
	}))
	if err := (Jira{Client: c}).CommentInternal(context.Background(), "dev.example.atlassian.net", "ABC-1", "private update", true); err != nil || hit {
		t.Fatalf("error %v, site called %v", err, hit)
	}
}

func TestConfluenceSearchLive(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/wiki/rest/api/content/search") {
			t.Fatal(r.URL.Path)
		}
		if !strings.Contains(r.URL.Query().Get("expand"), "ancestors") {
			t.Fatal(r.URL.String())
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []any{map[string]any{"id": "100", "title": "Doc", "space": map[string]any{"key": "DOCS"}, "ancestors": []any{map[string]any{"id": "50"}}}},
		})
	}))
	page, err := Confluence{Client: c}.Search(context.Background(), "dev.example.atlassian.net", `space = DOCS`)
	if err != nil || page.Count != 1 || page.Items[0].Title != "Doc" || page.Items[0].ParentID != "50" {
		t.Fatalf("%v %+v", err, page)
	}
}

func TestBitbucketOperationsUseSelectedWorkspaceCred(t *testing.T) {
	tests := []struct {
		name string
		call func(Bitbucket) error
	}{
		{name: "get", call: func(b Bitbucket) error {
			_, err := b.Get(context.Background(), "workspace-b", "atlas", 1)
			return err
		}},
		{name: "list", call: func(b Bitbucket) error {
			_, err := b.List(context.Background(), "workspace-b", "atlas")
			return err
		}},
		{name: "create", call: func(b Bitbucket) error {
			_, err := b.Create(context.Background(), "workspace-b", "atlas", domain.CreatePullRequest{Title: "pr", Source: "feature"}, false)
			return err
		}},
		{name: "edit", call: func(b Bitbucket) error {
			description := "updated"
			_, err := b.Edit(context.Background(), "workspace-b", "atlas", 1, domain.EditPullRequest{Description: &description}, false)
			return err
		}},
		{name: "comment", call: func(b Bitbucket) error {
			return b.Comment(context.Background(), "workspace-b", "atlas", 1, "body", false)
		}},
		{name: "merge", call: func(b Bitbucket) error {
			_, err := b.Merge(context.Background(), "workspace-b", "atlas", 1, false)
			return err
		}},
		{name: "diff", call: func(b Bitbucket) error {
			_, err := b.Diff(context.Background(), "workspace-b", "atlas", 1)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if user, token, ok := r.BasicAuth(); !ok || user != "workspace-b@example.com" || token != "workspace-b-token" {
					t.Fatalf("wrong credential: user=%q token=%q present=%v", user, token, ok)
				}
				if !strings.Contains(r.URL.Path, "/repositories/workspace-b/atlas/pullrequests") {
					t.Fatal(r.URL.Path)
				}
				switch tt.name {
				case "list":
					_ = json.NewEncoder(w).Encode(map[string]any{"values": []any{}})
				case "comment":
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{}`))
				case "create":
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(map[string]any{"id": 2, "title": "pr", "state": "OPEN"})
				case "diff":
					_, _ = w.Write([]byte("diff --git a/file b/file"))
				default:
					_ = json.NewEncoder(w).Encode(map[string]any{
						"id": 1, "title": "pr", "state": "OPEN",
						"source":      map[string]any{"branch": map[string]any{"name": "feat"}},
						"destination": map[string]any{"branch": map[string]any{"name": "main"}},
					})
				}
			}))
			t.Cleanup(srv.Close)
			store := &keychain.Fake{}
			_ = auth.PutSite(store, "dev", auth.Cred{Email: "site@example.com", Token: "site-token"})
			_ = auth.PutWorkspace(store, "workspace-a", auth.Cred{Email: "workspace-a@example.com", Token: "workspace-a-token"})
			_ = auth.PutWorkspace(store, "workspace-b", auth.Cred{Email: "workspace-b@example.com", Token: "workspace-b-token"})
			client := &Client{HTTP: srv.Client(), Store: store, BBBase: srv.URL}
			if err := tt.call(Bitbucket{Client: client}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBitbucketEditUpdatesDescriptionWithoutReplacingTitle(t *testing.T) {
	putCount := 0
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/ws/atlas/pullrequests/1" {
			t.Fatal(r.URL.Path)
		}
		if r.Method == http.MethodPut {
			putCount++
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if len(payload) != 2 || payload["title"] != "Old title" || payload["description"] != "## Revised\n\nSummary" {
				t.Fatalf("payload %#v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "state": "OPEN", "title": payload["title"], "description": payload["description"]})
			return
		}
		if r.Method != http.MethodGet {
			t.Fatal(r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "state": "OPEN", "title": "Old title", "description": "Old description"})
	}))
	if err := auth.PutWorkspace(c.Store, "ws", auth.Cred{Email: "ws@example.com", Token: "ws-token"}); err != nil {
		t.Fatal(err)
	}
	description := "## Revised\n\nSummary"
	b := Bitbucket{Client: c}
	preview, err := b.Edit(context.Background(), "ws", "atlas", 1, domain.EditPullRequest{Description: &description}, true)
	if err != nil || preview.Title != "Old title" || preview.Description != description || putCount != 0 {
		t.Fatalf("preview %v %+v put count %d", err, preview, putCount)
	}
	edited, err := b.Edit(context.Background(), "ws", "atlas", 1, domain.EditPullRequest{Description: &description}, false)
	if err != nil || edited.Title != "Old title" || edited.Description != description || putCount != 1 {
		t.Fatalf("edit %v %+v put count %d", err, edited, putCount)
	}
}

func TestBitbucketDoesNotFallBackToLicensedSiteCred(t *testing.T) {
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	t.Cleanup(srv.Close)
	store := &keychain.Fake{}
	_ = auth.PutSite(store, "dev", auth.Cred{Email: "site@example.com", Token: "site-token"})
	client := &Client{HTTP: srv.Client(), Store: store, BBBase: srv.URL}
	_, err := (Bitbucket{Client: client}).Get(context.Background(), "missing-workspace", "atlas", 1)
	de, ok := err.(*domain.Error)
	if !ok || de.Class != domain.ClassAuth || !strings.Contains(de.Hint, "auth login --workspace missing-workspace") {
		t.Fatalf("unexpected error: %#v", err)
	}
	if hit {
		t.Fatal("Bitbucket request sent with a licensed-site credential")
	}
}

func TestJSMDesksAndPublicComment(t *testing.T) {
	var comment map[string]any
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/servicedesk"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"values": []any{map[string]any{"id": "3", "projectKey": "EOS", "projectName": "EOS"}},
			})
		case strings.Contains(r.URL.Path, "/comment"):
			b, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(b, &comment)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Fatal(r.URL.Path)
		}
	}))
	desks, err := JSM{Client: c}.Desks(context.Background(), "helpdesk.example.atlassian.net")
	if err != nil || desks.Count != 1 || desks.Items[0].Key != "EOS" {
		t.Fatalf("%v %+v", err, desks)
	}
	if err := (JSM{Client: c}).Comment(context.Background(), "helpdesk.example.atlassian.net", "EOS-1", "hi", false); err != nil {
		t.Fatal(err)
	}
	if comment["public"] != true {
		t.Fatalf("%v", comment)
	}
}

func TestJSMCreateOmitsRaiseOnBehalfOf(t *testing.T) {
	var payload map[string]any
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &payload)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"issueKey": "EOS-2", "serviceDeskId": "3", "currentStatus": map[string]any{"status": "OPEN"}})
	}))
	got, err := JSM{Client: c}.Create(context.Background(), "helpdesk.example.atlassian.net", domain.CreateRequest{
		DeskID: "3", TypeID: "10", Summary: "s",
	}, false)
	if err != nil || got.Key != "EOS-2" {
		t.Fatalf("%v %+v", err, got)
	}
	if _, ok := payload["raiseOnBehalfOf"]; ok {
		t.Fatal(payload)
	}
}
