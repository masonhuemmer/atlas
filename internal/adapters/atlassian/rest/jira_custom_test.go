package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func adfDoc(blocks ...any) map[string]any {
	return map[string]any{"type": "doc", "version": 1, "content": blocks}
}

func adfPara(inlines ...any) map[string]any {
	return map[string]any{"type": "paragraph", "content": inlines}
}

func adfTextNode(text string) map[string]any {
	return map[string]any{"type": "text", "text": text}
}

func TestFlattenFieldValue(t *testing.T) {
	linked := map[string]any{"type": "text", "text": "docs", "marks": []any{
		map[string]any{"type": "strong"},
		map[string]any{"type": "link", "attrs": map[string]any{"href": "https://example.com/a"}},
	}}
	bareLink := map[string]any{"type": "text", "text": "https://example.com/b", "marks": []any{
		map[string]any{"type": "link", "attrs": map[string]any{"href": "https://example.com/b"}},
	}}
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"nil", nil, ""},
		{"string as is", "plain text", "plain text"},
		{"empty string", "", ""},
		{"blank string", "  \n ", ""},
		{"date as is", "2026-10-23T13:58:56.925+0000", "2026-10-23T13:58:56.925+0000"},
		{"integer", float64(3), "3"},
		{"decimal", 3.5, "3.5"},
		{"large integer", float64(10538), "10538"},
		{"bool", true, "true"},
		{"user display name", map[string]any{"accountId": "a1", "displayName": "Ada Lovelace"}, "Ada Lovelace"},
		{"user account id fallback", map[string]any{"accountId": "a1"}, "a1"},
		{"option value", map[string]any{"id": "1", "value": "High"}, "High"},
		{"cascading select", map[string]any{"value": "EMEA", "child": map[string]any{"value": "Frankfurt"}}, "EMEA / Frankfurt"},
		{"cascading parent only", map[string]any{"value": "EMEA", "child": nil}, "EMEA"},
		{"named object", map[string]any{"id": "7", "name": "1.0"}, "1.0"},
		{"unknown object is compact JSON", map[string]any{"b": float64(1), "a": "x"}, `{"a":"x","b":1}`},
		{"empty object", map[string]any{}, ""},
		{"empty array", []any{}, ""},
		{"users joined", []any{
			map[string]any{"displayName": "Ada Lovelace"},
			map[string]any{"displayName": "Grace Hopper"},
		}, "Ada Lovelace; Grace Hopper"},
		{"array skips null and empty", []any{nil, "", map[string]any{"value": "A"}, map[string]any{"value": ""}, "B"}, "A; B"},
		{"strings joined", []any{"a", "b", "c"}, "a; b; c"},
		{"empty doc", adfDoc(), ""},
		{"empty paragraph", adfDoc(adfPara()), ""},
		{"paragraphs split by newline", adfDoc(adfPara(adfTextNode("first")), adfPara(adfTextNode("second"))), "first\nsecond"},
		{"inline nodes concatenate", adfDoc(adfPara(adfTextNode("a"), adfTextNode("b"))), "ab"},
		{"hard break", adfDoc(adfPara(adfTextNode("a"), map[string]any{"type": "hardBreak"}, adfTextNode("b"))), "a\nb"},
		{"mention uses attrs.text", adfDoc(adfPara(adfTextNode("Ping "), map[string]any{"type": "mention", "attrs": map[string]any{"id": "a1", "text": "@Ada Lovelace"}}, adfTextNode(" now"))), "Ping @Ada Lovelace now"},
		{"emoji uses attrs.text", adfDoc(adfPara(adfTextNode("ok "), map[string]any{"type": "emoji", "attrs": map[string]any{"shortName": ":thumbsup:", "text": "👍"}})), "ok 👍"},
		{"inlineCard uses attrs.url", adfDoc(adfPara(adfTextNode("see "), map[string]any{"type": "inlineCard", "attrs": map[string]any{"url": "https://example.com/c"}})), "see https://example.com/c"},
		{"link mark kept as text <href>", adfDoc(adfPara(linked)), "docs <https://example.com/a>"},
		{"link text equal to href is not doubled", adfDoc(adfPara(bareLink)), "https://example.com/b"},
		{"bullet list items are blocks", adfDoc(map[string]any{"type": "bulletList", "content": []any{
			map[string]any{"type": "listItem", "content": []any{adfPara(adfTextNode("one"))}},
			map[string]any{"type": "listItem", "content": []any{adfPara(adfTextNode("two"))}},
		}}), "one\ntwo"},
		{"code block", adfDoc(map[string]any{"type": "codeBlock", "content": []any{adfTextNode("x := 1")}}), "x := 1"},
		{"adf inside array", []any{adfDoc(adfPara(adfTextNode("x")))}, "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flattenField(tc.in); got != tc.want {
				t.Fatalf("flattenField(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFieldTypeFromSchema(t *testing.T) {
	cases := []struct {
		schema map[string]any
		want   string
	}{
		{map[string]any{"type": "string", "custom": "com.atlassian.jira.plugin.system.customfieldtypes:textarea"}, "textarea"},
		{map[string]any{"type": "array", "items": "user", "custom": "com.atlassian.jira.plugin.system.customfieldtypes:multiuserpicker"}, "multiuserpicker"},
		{map[string]any{"type": "string"}, "string"},
		{map[string]any{"type": "option", "custom": ""}, "option"},
		{map[string]any{"custom": "com.example.plugin:my-field"}, "my-field"},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := fieldTypeFromSchema(tc.schema); got != tc.want {
			t.Errorf("fieldTypeFromSchema(%v) = %q, want %q", tc.schema, got, tc.want)
		}
	}
}

func TestBuildCustomFields(t *testing.T) {
	catalog := map[string]FieldMeta{
		"customfield_10481": {Name: "Peer Reviewer", Type: "userpicker"},
		"customfield_10538": {Name: "Notification of Changes", Type: "textarea"},
		"customfield_9":     {Name: "Small", Type: "textfield"},
	}
	values := map[string]any{
		"summary":           "not custom",
		"customfield_10538": adfDoc(adfPara(adfTextNode("Ada Lovelace ; Grace Hopper"))),
		"customfield_10481": map[string]any{"displayName": "Ada Lovelace"},
		"customfield_9":     "nine",
		"customfield_10602": []any{},
		"customfield_10603": nil,
		"customfield_77":    "no catalog entry",
	}
	all := BuildCustomFields(values, catalog, nil, true)
	want := []domain.CustomField{
		{ID: "customfield_9", Name: "Small", Type: "textfield", Text: "nine"},
		{ID: "customfield_77", Name: "customfield_77", Text: "no catalog entry"},
		{ID: "customfield_10481", Name: "Peer Reviewer", Type: "userpicker", Text: "Ada Lovelace"},
		{ID: "customfield_10538", Name: "Notification of Changes", Type: "textarea", Text: "Ada Lovelace ; Grace Hopper"},
	}
	if !reflect.DeepEqual(all, want) {
		t.Fatalf("all fields (numeric id order)\n got %+v\nwant %+v", all, want)
	}
	picked := BuildCustomFields(values, catalog, []string{"customfield_10538", "customfield_10602", "customfield_9", "customfield_10538", "customfield_404"}, false)
	wantPicked := []domain.CustomField{want[3], want[0]}
	if !reflect.DeepEqual(picked, wantPicked) {
		t.Fatalf("picked fields keep request order, drop empty/absent/duplicate\n got %+v\nwant %+v", picked, wantPicked)
	}
	if none := BuildCustomFields(values, catalog, nil, false); len(none) != 0 {
		t.Fatalf("no ids and not all returned %+v", none)
	}
}

const fieldCatalogJSON = `[
  {"id":"summary","name":"Summary","custom":false,"schema":{"type":"string","system":"summary"}},
  {"id":"customfield_10538","name":"Notification of Changes","custom":true,"schema":{"type":"string","custom":"com.atlassian.jira.plugin.system.customfieldtypes:textarea","customId":10538}},
  {"id":"customfield_10481","name":"Peer Reviewer","custom":true,"schema":{"type":"user","custom":"com.atlassian.jira.plugin.system.customfieldtypes:userpicker","customId":10481}},
  {"id":"customfield_10602","name":"Environments","custom":true,"schema":{"type":"array","items":"option","custom":"com.atlassian.jira.plugin.system.customfieldtypes:multiselect","customId":10602}}
]`

func issueWithCustomFields() map[string]any {
	return map[string]any{
		"key": "ABC-1",
		"fields": map[string]any{
			"summary":           "hello",
			"status":            map[string]any{"name": "To Do"},
			"customfield_10538": adfDoc(adfPara(adfTextNode("Ada Lovelace ; Grace Hopper"))),
			"customfield_10481": map[string]any{"accountId": "a1", "displayName": "Ada Lovelace"},
			"customfield_10602": []any{},
			"customfield_10603": nil,
		},
	}
}

func TestJiraGetCustomAllFields(t *testing.T) {
	var fieldCalls, issueCalls int32
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("%s %s: custom reads must not write", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/rest/api/3/field":
			atomic.AddInt32(&fieldCalls, 1)
			_, _ = w.Write([]byte(fieldCatalogJSON))
		case "/rest/api/3/issue/ABC-1":
			atomic.AddInt32(&issueCalls, 1)
			if got := r.URL.Query().Get("fields"); !strings.HasPrefix(got, "*all") {
				t.Errorf("fields query %q, want *all", got)
			}
			_ = json.NewEncoder(w).Encode(issueWithCustomFields())
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	got, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "abc-1", nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.CustomField{
		{ID: "customfield_10481", Name: "Peer Reviewer", Type: "userpicker", Text: "Ada Lovelace"},
		{ID: "customfield_10538", Name: "Notification of Changes", Type: "textarea", Text: "Ada Lovelace ; Grace Hopper"},
	}
	if !reflect.DeepEqual(got.CustomFields, want) {
		t.Fatalf("got %+v\nwant %+v", got.CustomFields, want)
	}
	if got.StatusHistory != nil {
		t.Fatalf("history requested=false but got %+v", got.StatusHistory)
	}
	if fieldCalls != 1 || issueCalls != 1 {
		t.Fatalf("field calls %d, issue calls %d, want one each", fieldCalls, issueCalls)
	}
}

func TestJiraGetCustomSelectedIDsRequestOnlyThoseFields(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/field":
			_, _ = w.Write([]byte(fieldCatalogJSON))
		case "/rest/api/3/issue/ABC-1":
			if got := r.URL.Query().Get("fields"); got != "customfield_10481,customfield_10602" {
				t.Errorf("fields query %q", got)
			}
			_ = json.NewEncoder(w).Encode(issueWithCustomFields())
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	got, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", []string{"customfield_10481", "customfield_10602"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CustomFields) != 1 || got.CustomFields[0].ID != "customfield_10481" || got.CustomFields[0].Text != "Ada Lovelace" {
		t.Fatalf("empty customfield_10602 must be omitted: %+v", got.CustomFields)
	}
}

func TestJiraGetCustomRejectsMalformedFieldID(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s", r.URL)
	}))
	_, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", []string{"customfield_1&expand=changelog"}, false, false)
	if domain.ClassOf(err) != domain.ClassUsage {
		t.Fatalf("err %v", err)
	}
}

func changelogPage(startAt, total int, isLast bool, values ...any) map[string]any {
	return map[string]any{"startAt": startAt, "maxResults": 2, "total": total, "isLast": isLast, "values": values}
}

func changelogEntry(created, author string, items ...any) map[string]any {
	entry := map[string]any{"created": created, "items": items}
	if author != "" {
		entry["author"] = map[string]any{"accountId": "acct-" + author, "displayName": author}
	}
	return entry
}

func changeItem(field, from, to string) map[string]any {
	return map[string]any{"field": field, "fromString": from, "toString": to}
}

func TestJiraGetCustomHistoryPagesThroughWholeChangelog(t *testing.T) {
	var starts []string
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/ABC-1/changelog" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		starts = append(starts, r.URL.Query().Get("startAt"))
		if r.URL.Query().Get("maxResults") == "" {
			t.Error("maxResults not requested")
		}
		start, _ := strconv.Atoi(r.URL.Query().Get("startAt"))
		// The server serves two entries per page whatever maxResults says.
		pages := map[int]map[string]any{
			0: changelogPage(0, 5, false,
				changelogEntry("2026-10-01T10:00:00.000+0000", "Ada Lovelace", changeItem("status", "Open", "In Progress")),
				changelogEntry("2026-10-02T10:00:00.000+0000", "Grace Hopper", changeItem("assignee", "", "Grace Hopper"))),
			2: changelogPage(2, 5, false,
				changelogEntry("2026-10-03T10:00:00.000+0000", "Ada Lovelace", changeItem("priority", "Low", "High"), changeItem("status", "In Progress", "In QA")),
				changelogEntry("2026-10-04T10:00:00.000+0000", "", changeItem("status", "In QA", "Done"))),
			4: changelogPage(4, 5, true,
				changelogEntry("2026-10-05T10:00:00.000+0000", "Grace Hopper", changeItem("Status Category", "x", "y"))),
		}
		page, ok := pages[start]
		if !ok {
			t.Errorf("unexpected startAt %d", start)
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	got, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, false, true)
	if err != nil {
		t.Fatal(err)
	}
	want := []domain.StatusChange{
		{Created: "2026-10-01T10:00:00.000+0000", Author: "Ada Lovelace", From: "Open", To: "In Progress"},
		{Created: "2026-10-03T10:00:00.000+0000", Author: "Ada Lovelace", From: "In Progress", To: "In QA"},
		{Created: "2026-10-04T10:00:00.000+0000", From: "In QA", To: "Done"},
	}
	if !reflect.DeepEqual(got.StatusHistory, want) {
		t.Fatalf("got %+v\nwant %+v", got.StatusHistory, want)
	}
	if strings.Join(starts, ",") != "0,2,4" {
		t.Fatalf("startAt sequence %v, want 0,2,4", starts)
	}
	if got.CustomFields != nil {
		t.Fatalf("custom fields not requested but got %+v", got.CustomFields)
	}
}

func TestJiraGetCustomHistoryStopsOnEmptyPage(t *testing.T) {
	calls := 0
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 3 {
			t.Error("kept paging after an empty page")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"isLast": false, "total": 99, "values": []any{}})
	}))
	got, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, false, true)
	if err != nil || len(got.StatusHistory) != 0 || calls != 1 {
		t.Fatalf("history %+v, err %v, calls %d", got.StatusHistory, err, calls)
	}
}

func TestJiraGetCustomFieldsAndHistoryTogether(t *testing.T) {
	var paths []string
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/rest/api/3/field":
			_, _ = w.Write([]byte(fieldCatalogJSON))
		case "/rest/api/3/issue/ABC-1":
			_ = json.NewEncoder(w).Encode(issueWithCustomFields())
		case "/rest/api/3/issue/ABC-1/changelog":
			_ = json.NewEncoder(w).Encode(changelogPage(0, 1, true, changelogEntry("2026-10-01T10:00:00.000+0000", "Ada Lovelace", changeItem("status", "Open", "Done"))))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	got, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, true, true)
	if err != nil || len(got.CustomFields) != 2 || len(got.StatusHistory) != 1 {
		t.Fatalf("%+v, %v", got, err)
	}
	if len(paths) != 3 {
		t.Fatalf("paths %v", paths)
	}
}

func TestJiraGetCustomTruncatedIssueIsServiceError(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/field" {
			_, _ = w.Write([]byte(fieldCatalogJSON))
			return
		}
		// Valid JSON, but past the 1 MiB read cap: a cut body must not decode as an empty issue.
		_, _ = io.WriteString(w, `{"key":"ABC-1","fields":{"customfield_10538":"`)
		_, _ = io.WriteString(w, strings.Repeat("x", 1<<20+100))
		_, _ = io.WriteString(w, `"}}`)
	}))
	got, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, true, false)
	if domain.ClassOf(err) != domain.ClassService || !strings.Contains(err.Error(), "1 MiB") {
		t.Fatalf("got %+v, err %v", got, err)
	}
}

func TestJiraGetCustomTruncatedChangelogIsServiceError(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"values":[{"created":"x","items":[{"field":"status","fromString":"`)
		_, _ = io.WriteString(w, strings.Repeat("x", 1<<20+100))
		_, _ = io.WriteString(w, `"}]}]}`)
	}))
	_, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, false, true)
	if domain.ClassOf(err) != domain.ClassService {
		t.Fatalf("err %v", err)
	}
}

func TestJiraGetCustomBrokenConnectionIsServiceError(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/field" {
			_, _ = w.Write([]byte(fieldCatalogJSON))
			return
		}
		// The bytes that arrive are a complete, valid issue, but fewer than promised.
		w.Header().Set("Content-Length", "500")
		_, _ = io.WriteString(w, `{"key":"ABC-1","fields":{}}`)
	}))
	got, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, true, false)
	if domain.ClassOf(err) != domain.ClassService {
		t.Fatalf("a body cut mid-stream looked like success: %+v, %v", got, err)
	}
}

func TestJiraGetCustomInvalidJSONIsServiceError(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/field" {
			_, _ = io.WriteString(w, `<html>gateway</html>`)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	_, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, true, false)
	if domain.ClassOf(err) != domain.ClassService {
		t.Fatalf("err %v", err)
	}
}

func TestJiraGetCustomStatusMapping(t *testing.T) {
	for code, class := range map[int]string{
		http.StatusNotFound:     domain.ClassNotFound,
		http.StatusForbidden:    domain.ClassAuth,
		http.StatusUnauthorized: domain.ClassAuth,
		http.StatusBadGateway:   domain.ClassService,
	} {
		c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/rest/api/3/field" {
				_, _ = w.Write([]byte(fieldCatalogJSON))
				return
			}
			w.WriteHeader(code)
		}))
		for _, history := range []bool{false, true} {
			_, err := (Jira{Client: c}).GetCustom(context.Background(), "dev.example.atlassian.net", "ABC-1", nil, !history, history)
			if domain.ClassOf(err) != class {
				t.Errorf("status %d history=%v: class %q, want %q (%v)", code, history, domain.ClassOf(err), class, err)
			}
		}
	}
}

func TestJiraGetStillRequestsFixedFields(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/issue/ABC-1" || r.URL.Query().Get("fields") != jiraFields {
			t.Errorf("Get widened: %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(issueWithCustomFields())
	}))
	if _, err := (Jira{Client: c}).Get(context.Background(), "dev.example.atlassian.net", "ABC-1"); err != nil {
		t.Fatal(err)
	}
}
