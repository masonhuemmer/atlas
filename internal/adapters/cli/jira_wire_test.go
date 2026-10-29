package cli

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian/rest"
	"github.com/masonhuemmer/atlas/internal/adapters/keychain"
	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/domain"
)

const wireFieldCatalog = `[
  {"id":"customfield_10538","name":"Notification of Changes","custom":true,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea","customId":10538}},
  {"id":"customfield_10481","name":"Peer Reviewer","custom":true,"schema":{"type":"user","custom":"com.atlassian.jira.plugin.system.customfieldtypes:userpicker","customId":10481}}
]`

// wireDeps runs the CLI against the live REST adapter, with a local test
// server standing in for the site. Nothing leaves the machine.
func wireDeps(t *testing.T, h http.Handler) Deps {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	store := &keychain.Fake{}
	if err := auth.PutSite(store, "sesamidevel", auth.Cred{Email: "a@b.c", Token: "tok"}); err != nil {
		t.Fatal(err)
	}
	d, _, _ := testDeps()
	d.Jira = rest.Jira{Client: &rest.Client{HTTP: srv.Client(), Store: store, BaseURL: srv.URL}}
	return d
}

func TestWireCreateDryRunReadsFieldSchemaOnceAndNeverWrites(t *testing.T) {
	var reads, others int32
	d := wireDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field" {
			atomic.AddInt32(&reads, 1)
			_, _ = io.WriteString(w, wireFieldCatalog)
			return
		}
		atomic.AddInt32(&others, 1)
		http.Error(w, "unexpected", http.StatusTeapot)
	}))
	code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "preview",
		"--fields", `{"customfield_10538":"one\n\ntwo","customfield_10481":{"accountId":"acct-1"}}`, "--dry-run")
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	if reads != 1 || others != 0 {
		t.Fatalf("GET /field %d times, other requests %d", reads, others)
	}
	for _, want := range []string{`"dry_run":true`, `"fields":{`, `"customfield_10538":{"content":[`, `"customfield_10481":{"accountId":"acct-1"}`, `"summary":"preview"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
}

func TestWireCreateDryRunWithoutSchemaNeedsNoRequest(t *testing.T) {
	var requests int32
	d := wireDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
	}))
	code, _, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "preview", "--fields", `{"customfield_10481":{"accountId":"acct-1"}}`, "--dry-run")
	if code != domain.ExitOK || requests != 0 {
		t.Fatalf("exit %d, %d requests: %s", code, requests, errw)
	}
}

func TestWireCreateShowsJiraValidationText(t *testing.T) {
	d := wireDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/field" {
			_, _ = io.WriteString(w, wireFieldCatalog)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"errorMessages":[],"errors":{"customfield_10481":"Specify a valid user"}}`)
	}))
	code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "s", "--fields", `{"customfield_10481":"nobody"}`)
	if code != domain.ExitService || out != "" {
		t.Fatalf("exit %d, stdout %q", code, out)
	}
	if !strings.Contains(errw, `"class":"service"`) || !strings.Contains(errw, "customfield_10481: Specify a valid user") {
		t.Fatal(errw)
	}
}

func TestWireGetCustomAndHistory(t *testing.T) {
	d := wireDeps(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/SDO-1":
			if strings.HasPrefix(r.URL.Query().Get("fields"), "*all") {
				_, _ = io.WriteString(w, `{"key":"SDO-1","fields":{"customfield_10481":{"accountId":"a1","displayName":"Ada Lovelace"},"customfield_10538":null}}`)
				return
			}
			_, _ = io.WriteString(w, `{"key":"SDO-1","fields":{"summary":"hello","status":{"name":"In QA"},"issuetype":{"name":"Task"},"project":{"key":"SDO"}}}`)
		case "/rest/api/3/field":
			_, _ = io.WriteString(w, wireFieldCatalog)
		case "/rest/api/3/issue/SDO-1/changelog":
			_, _ = io.WriteString(w, `{"isLast":true,"total":1,"values":[{"created":"2026-10-23T19:10:14.066+0000","author":{"displayName":"Ada Lovelace"},"items":[{"field":"status","fromString":"Open","toString":"In QA"}]}]}`)
		default:
			t.Errorf("unexpected %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	got, raw := runGet(t, d, "SDO-1", "--fields", "key,status,custom,history")
	if got.Key != "SDO-1" || got.Status != "In QA" {
		t.Fatal(raw)
	}
	if len(got.CustomFields) != 1 || got.CustomFields[0] != (domain.CustomField{ID: "customfield_10481", Name: "Peer Reviewer", Type: "userpicker", Text: "Ada Lovelace"}) {
		t.Fatal(raw)
	}
	if len(got.StatusHistory) != 1 || got.StatusHistory[0] != (domain.StatusChange{Created: "2026-10-23T19:10:14.066+0000", Author: "Ada Lovelace", From: "Open", To: "In QA"}) {
		t.Fatal(raw)
	}
}
