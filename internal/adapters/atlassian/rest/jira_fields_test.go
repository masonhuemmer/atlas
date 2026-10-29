package rest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func staticCatalog(calls *int32) FieldCatalog {
	return func() (map[string]FieldMeta, error) {
		atomic.AddInt32(calls, 1)
		return map[string]FieldMeta{
			"customfield_10538": {Name: "Notification of Changes", Type: "textarea"},
			"customfield_10010": {Name: "Notes", Type: "textfield"},
			"customfield_10602": {Name: "Environments", Type: "multiselect"},
			"customfield_10700": {Name: "Release Notes", Type: "textarea"},
		}, nil
	}
}

func TestConvertCustomFields(t *testing.T) {
	var calls int32
	in := map[string]any{
		"customfield_10538": "line one\n\nline two",
		"customfield_10700": "**bold** note",
		"customfield_10010": "plain text stays",
		"customfield_10602": []any{map[string]any{"value": "PROD"}},
		"customfield_10481": map[string]any{"accountId": "a1"},
		"customfield_99999": "not in the catalog",
		"description":       "left for the caller",
		"labels":            []any{"a"},
	}
	snapshot, _ := json.Marshal(in)
	out, err := ConvertCustomFields(in, staticCatalog(&calls))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("catalog loaded %d times, want exactly once", calls)
	}
	again, _ := json.Marshal(in)
	if string(again) != string(snapshot) {
		t.Fatalf("input map was modified: %s", again)
	}
	doc := asMap(out["customfield_10538"])
	if doc["type"] != "doc" || len(asList(doc["content"])) != 2 {
		t.Fatalf("textarea string not converted to a two-paragraph document: %v", out["customfield_10538"])
	}
	if flattenField(out["customfield_10538"]) != "line one\nline two" {
		t.Fatalf("round trip %q", flattenField(out["customfield_10538"]))
	}
	if !strings.Contains(mustMarshal(t, out["customfield_10700"]), `"type":"strong"`) {
		t.Fatalf("markdown not rendered: %v", out["customfield_10700"])
	}
	for _, key := range []string{"customfield_10010", "customfield_99999", "description", "customfield_10602", "customfield_10481", "labels"} {
		if !reflect.DeepEqual(out[key], in[key]) {
			t.Errorf("%s changed: %v -> %v", key, in[key], out[key])
		}
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConvertCustomFieldsSkipsCatalogWithoutStringValues(t *testing.T) {
	var calls int32
	for _, in := range []map[string]any{
		nil,
		{},
		{"summary": "x", "description": "markdown"},
		{"customfield_10538": adfDoc(adfPara(adfTextNode("already ADF")))},
		{"customfield_10602": []any{"a"}, "customfield_10700": nil},
	} {
		out, err := ConvertCustomFields(in, staticCatalog(&calls))
		if err != nil || len(out) != len(in) {
			t.Fatalf("%v -> %v, %v", in, out, err)
		}
	}
	if calls != 0 {
		t.Fatalf("catalog loaded %d times for fields that need no schema", calls)
	}
}

func TestConvertCustomFieldsEmptyTextareaClearsField(t *testing.T) {
	var calls int32
	out, err := ConvertCustomFields(map[string]any{"customfield_10538": ""}, staticCatalog(&calls))
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := out["customfield_10538"]; !ok || value != nil {
		t.Fatalf("empty Markdown should send null, got %#v", out)
	}
	if mustMarshal(t, out) != `{"customfield_10538":null}` {
		t.Fatal(mustMarshal(t, out))
	}
}

func TestConvertCustomFieldsCatalogErrorFailsTheWrite(t *testing.T) {
	boom := domain.Auth("not authorized for this jira site")
	_, err := ConvertCustomFields(map[string]any{"customfield_10538": "x"}, func() (map[string]FieldMeta, error) { return nil, boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
}

func TestCreateFieldsMergesFlagsAndFields(t *testing.T) {
	var calls int32
	got, err := CreateFields(domain.CreateIssue{
		Project: "ABC", IssueType: "Task", Summary: "s", Parent: "ABC-9", Labels: []string{"x"}, Assignee: "acct-1",
		Description: "**d**",
		Fields: map[string]any{
			"customfield_10538": "note",
			"priority":          map[string]any{"name": "High"},
			// A caller that skipped CLI validation still cannot override what the flags own.
			"summary": "overridden", "project": map[string]any{"key": "ZZZ"},
		},
	}, staticCatalog(&calls))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("catalog calls %d", calls)
	}
	if got["summary"] != "s" || asMap(got["project"])["key"] != "ABC" || asMap(got["issuetype"])["name"] != "Task" {
		t.Fatalf("flags must win: %v", got)
	}
	if asMap(got["parent"])["key"] != "ABC-9" || asMap(got["assignee"])["accountId"] != "acct-1" || !reflect.DeepEqual(got["labels"], []string{"x"}) {
		t.Fatalf("%v", got)
	}
	if asMap(got["description"])["type"] != "doc" || asMap(got["customfield_10538"])["type"] != "doc" {
		t.Fatalf("Markdown not converted: %v", got)
	}
	if asMap(got["priority"])["name"] != "High" {
		t.Fatalf("plain field lost: %v", got)
	}
}

func TestCreateFieldsDoesNotInventAssignee(t *testing.T) {
	got, err := CreateFields(domain.CreateIssue{
		Project: "ABC", IssueType: "Task", Summary: "s",
		Fields: map[string]any{"assignee": map[string]any{"accountId": "from-fields"}, "description": "# Title"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if asMap(got["assignee"])["accountId"] != "from-fields" {
		t.Fatalf("%v", got["assignee"])
	}
	if asMap(got["description"])["type"] != "doc" {
		t.Fatalf("a description string in --fields converts like the --description flag: %v", got["description"])
	}
	bare, _ := CreateFields(domain.CreateIssue{Project: "ABC", IssueType: "Task", Summary: "s"}, nil)
	if _, ok := bare["assignee"]; ok {
		t.Fatalf("no assignee given, none sent: %v", bare)
	}
}

type fieldWriteServer struct {
	t          *testing.T
	fieldCalls int32
	posted     map[string]any
	put        map[string]any
	writes     int32
}

func (s *fieldWriteServer) handler(createdKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field":
			atomic.AddInt32(&s.fieldCalls, 1)
			_, _ = w.Write([]byte(fieldCatalogJSON))
		case r.Method == http.MethodPost && r.URL.Path == "/rest/api/3/issue":
			atomic.AddInt32(&s.writes, 1)
			_ = json.NewDecoder(r.Body).Decode(&s.posted)
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"key":"`+createdKey+`"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/rest/api/3/issue/ABC-1":
			atomic.AddInt32(&s.writes, 1)
			_ = json.NewDecoder(r.Body).Decode(&s.put)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/api/3/issue/"):
			_ = json.NewEncoder(w).Encode(map[string]any{"key": strings.TrimPrefix(r.URL.Path, "/rest/api/3/issue/"), "fields": map[string]any{"summary": "s", "project": map[string]any{"key": "ABC"}}})
		default:
			s.t.Errorf("unexpected %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	})
}

func TestJiraCreateSendsMergedFieldsWithTextareaAsADF(t *testing.T) {
	srv := &fieldWriteServer{t: t}
	c, _ := liveClient(t, srv.handler("ABC-2"))
	created, err := (Jira{Client: c}).Create(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{
		Project: "ABC", IssueType: "Task", Summary: "s", Assignee: "acct-1",
		Fields: map[string]any{
			"customfield_10538": "Ada Lovelace ; Grace Hopper\n\nsecond paragraph",
			"customfield_10010": "not a textarea",
			"customfield_10602": []any{map[string]any{"value": "PROD"}},
		},
	}, false)
	if err != nil || created.Key != "ABC-2" {
		t.Fatalf("%+v %v", created, err)
	}
	if srv.fieldCalls != 1 {
		t.Fatalf("GET /field %d times, want once", srv.fieldCalls)
	}
	fields := asMap(srv.posted["fields"])
	doc := asMap(fields["customfield_10538"])
	if doc["type"] != "doc" || doc["version"] != float64(1) || len(asList(doc["content"])) != 2 {
		t.Fatalf("textarea not sent as ADF: %v", fields["customfield_10538"])
	}
	if fields["customfield_10010"] != "not a textarea" {
		t.Fatalf("textfield changed: %v", fields["customfield_10010"])
	}
	if !reflect.DeepEqual(fields["customfield_10602"], []any{map[string]any{"value": "PROD"}}) {
		t.Fatalf("select changed: %v", fields["customfield_10602"])
	}
	if fields["summary"] != "s" || asMap(fields["project"])["key"] != "ABC" || asMap(fields["assignee"])["accountId"] != "acct-1" {
		t.Fatalf("%v", fields)
	}
}

func TestJiraCreateWithoutCustomStringsSkipsFieldLookup(t *testing.T) {
	srv := &fieldWriteServer{t: t}
	c, _ := liveClient(t, srv.handler("ABC-2"))
	for _, extra := range []map[string]any{
		nil,
		{"priority": map[string]any{"name": "High"}},
		{"customfield_10602": []any{map[string]any{"value": "PROD"}}},
		{"customfield_10538": adfDoc(adfPara(adfTextNode("ADF given")))},
	} {
		if _, err := (Jira{Client: c}).Create(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{Project: "ABC", IssueType: "Task", Summary: "s", Fields: extra}, false); err != nil {
			t.Fatal(err)
		}
	}
	if srv.fieldCalls != 0 {
		t.Fatalf("GET /field %d times for fields that need no schema", srv.fieldCalls)
	}
}

func TestJiraCreateFieldsPreviewReadsButNeverWrites(t *testing.T) {
	srv := &fieldWriteServer{t: t}
	c, _ := liveClient(t, srv.handler("ABC-2"))
	got, err := (Jira{Client: c}).CreateFields(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{
		Project: "ABC", IssueType: "Task", Summary: "s", Fields: map[string]any{"customfield_10538": "preview me"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if asMap(got["customfield_10538"])["type"] != "doc" || got["summary"] != "s" {
		t.Fatalf("%v", got)
	}
	if srv.writes != 0 || srv.fieldCalls != 1 {
		t.Fatalf("writes %d, field calls %d", srv.writes, srv.fieldCalls)
	}
}

func TestJiraCreateFieldsPreviewWithoutStringsMakesNoRequest(t *testing.T) {
	c := &Client{} // no store, no server: any request or credential lookup would fail
	got, err := (Jira{Client: c}).CreateFields(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{Project: "ABC", IssueType: "Task", Summary: "s"})
	if err != nil || got["summary"] != "s" {
		t.Fatalf("%v %v", got, err)
	}
}

func TestJiraEditConvertsTextareaLikeCreate(t *testing.T) {
	srv := &fieldWriteServer{t: t}
	c, _ := liveClient(t, srv.handler("ABC-2"))
	_, err := (Jira{Client: c}).Edit(context.Background(), "dev.example.atlassian.net", "ABC-1", map[string]any{
		"customfield_10538": "first\n\nsecond",
		"customfield_10010": "plain",
		"description":       "## Heading",
		"summary":           "renamed",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if srv.fieldCalls != 1 {
		t.Fatalf("GET /field %d times, want once", srv.fieldCalls)
	}
	fields := asMap(srv.put["fields"])
	if asMap(fields["customfield_10538"])["type"] != "doc" || len(asList(asMap(fields["customfield_10538"])["content"])) != 2 {
		t.Fatalf("textarea not sent as ADF: %v", fields["customfield_10538"])
	}
	if asMap(fields["description"])["type"] != "doc" {
		t.Fatalf("description conversion lost: %v", fields["description"])
	}
	if fields["customfield_10010"] != "plain" || fields["summary"] != "renamed" {
		t.Fatalf("%v", fields)
	}
}

func TestJiraEditWithoutCustomStringsSkipsFieldLookup(t *testing.T) {
	srv := &fieldWriteServer{t: t}
	c, _ := liveClient(t, srv.handler("ABC-2"))
	if _, err := (Jira{Client: c}).Edit(context.Background(), "dev.example.atlassian.net", "ABC-1", map[string]any{"summary": "x", "description": "d"}, false); err != nil {
		t.Fatal(err)
	}
	if srv.fieldCalls != 0 {
		t.Fatalf("GET /field %d times", srv.fieldCalls)
	}
}

func TestJiraEditDryRunStillOnlyReads(t *testing.T) {
	srv := &fieldWriteServer{t: t}
	c, _ := liveClient(t, srv.handler("ABC-2"))
	if _, err := (Jira{Client: c}).Edit(context.Background(), "dev.example.atlassian.net", "ABC-1", map[string]any{"customfield_10538": "x"}, true); err != nil {
		t.Fatal(err)
	}
	if srv.writes != 0 {
		t.Fatal("dry-run wrote")
	}
}

func validationServer(status int, body string, writes *int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/rest/api/3/field" {
			_, _ = w.Write([]byte(fieldCatalogJSON))
			return
		}
		atomic.AddInt32(writes, 1)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	})
}

func TestJiraCreateSurfacesValidationErrors(t *testing.T) {
	var writes int32
	c, _ := liveClient(t, validationServer(http.StatusBadRequest,
		`{"errorMessages":["Field 'x' cannot be set."],"errors":{"summary":"You must specify a summary of the issue.","customfield_10538":"Operation value must be an Atlassian Document"}}`, &writes))
	_, err := (Jira{Client: c}).Create(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{Project: "ABC", IssueType: "Task", Summary: "s", Fields: map[string]any{"customfield_10538": 5}}, false)
	if domain.ClassOf(err) != domain.ClassService || domain.ExitOf(err) != domain.ExitService {
		t.Fatalf("a 400 stays a service error: %v", err)
	}
	msg := err.Error()
	for _, want := range []string{"Field 'x' cannot be set.", "customfield_10538: Operation value must be an Atlassian Document", "summary: You must specify a summary of the issue."} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
	// errorMessages first, then errors by key.
	if strings.Index(msg, "Field 'x'") > strings.Index(msg, "customfield_10538") || strings.Index(msg, "customfield_10538") > strings.Index(msg, "summary: You") {
		t.Errorf("unstable order: %q", msg)
	}
}

func TestJiraEditSurfacesValidationErrors(t *testing.T) {
	var writes int32
	c, _ := liveClient(t, validationServer(http.StatusBadRequest, `{"errorMessages":[],"errors":{"customfield_10481":"Specify a valid user"}}`, &writes))
	_, err := (Jira{Client: c}).Edit(context.Background(), "dev.example.atlassian.net", "ABC-1", map[string]any{"customfield_10481": "nobody"}, false)
	if domain.ClassOf(err) != domain.ClassService || !strings.Contains(err.Error(), "customfield_10481: Specify a valid user") {
		t.Fatalf("%v", err)
	}
}

func TestJiraWriteValidationMessageIsCapped(t *testing.T) {
	var writes int32
	c, _ := liveClient(t, validationServer(http.StatusBadRequest, `{"errorMessages":["`+strings.Repeat("é", 2000)+`"]}`, &writes))
	_, err := (Jira{Client: c}).Create(context.Background(), "dev.example.atlassian.net", domain.CreateIssue{Project: "ABC", IssueType: "Task", Summary: "s"}, false)
	if domain.ClassOf(err) != domain.ClassService {
		t.Fatal(err)
	}
	if n := len([]rune(err.Error())); n > 600 || n < 400 {
		t.Fatalf("message is %d runes, want about 500 at most", n)
	}
}

func TestJiraWriteOtherFailuresKeepGenericMessage(t *testing.T) {
	for name, tc := range map[string]struct {
		status int
		body   string
		want   error
	}{
		"400 without a body":       {http.StatusBadRequest, ``, MapStatus(http.StatusBadRequest)},
		"400 with other JSON":      {http.StatusBadRequest, `{"message":"x"}`, MapStatus(http.StatusBadRequest)},
		"400 with HTML":            {http.StatusBadRequest, `<html>bad</html>`, MapStatus(http.StatusBadRequest)},
		"400 with empty error set": {http.StatusBadRequest, `{"errorMessages":[],"errors":{}}`, MapStatus(http.StatusBadRequest)},
		"500 with errors":          {http.StatusInternalServerError, `{"errors":{"summary":"nope"}}`, MapStatus(http.StatusInternalServerError)},
		"403 with errorMessages":   {http.StatusForbidden, `{"errorMessages":["no"]}`, MapStatus(http.StatusForbidden)},
		"404 with errorMessages":   {http.StatusNotFound, `{"errorMessages":["Issue does not exist"]}`, MapStatus(http.StatusNotFound)},
	} {
		t.Run(name, func(t *testing.T) {
			var writes int32
			c, _ := liveClient(t, validationServer(tc.status, tc.body, &writes))
			_, err := (Jira{Client: c}).Edit(context.Background(), "dev.example.atlassian.net", "ABC-1", map[string]any{"summary": "x"}, false)
			if err == nil || err.Error() != tc.want.Error() || domain.ClassOf(err) != domain.ClassOf(tc.want) {
				t.Fatalf("got %v (%s), want %v (%s)", err, domain.ClassOf(err), tc.want, domain.ClassOf(tc.want))
			}
		})
	}
}
