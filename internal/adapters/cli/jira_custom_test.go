package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

type getOut struct {
	Key           string                `json:"key"`
	Status        string                `json:"status"`
	IssueType     string                `json:"issuetype"`
	Created       string                `json:"created"`
	CustomFields  []domain.CustomField  `json:"custom_fields"`
	StatusHistory []domain.StatusChange `json:"status_history"`
}

func runGet(t *testing.T, d Deps, args ...string) (getOut, string) {
	t.Helper()
	out, errw := d.Stdout.(*bytes.Buffer), d.Stderr.(*bytes.Buffer)
	code := Run(append([]string{"atlas", "jira", "get"}, args...), d)
	if code != domain.ExitOK {
		t.Fatalf("exit %d: %s", code, errw.String())
	}
	var got getOut
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatal(err, out.String())
	}
	return got, out.String()
}

var seedCustomFields = []domain.CustomField{
	{ID: "customfield_10481", Name: "Peer Reviewer", Type: "userpicker", Text: "Avery Example"},
	{ID: "customfield_10538", Name: "Notification of Changes", Type: "textarea", Text: "Alex Example ; Avery Example"},
	{ID: "customfield_10601", Name: "Region", Type: "cascadingselect", Text: "EMEA / Frankfurt"},
}

func TestJiraGetFieldsCustom(t *testing.T) {
	d, _, _ := testDeps()
	got, raw := runGet(t, d, "SDO-1", "--fields", "custom")
	if got.Key != "SDO-1" {
		t.Fatalf("key %q", got.Key)
	}
	if !reflect.DeepEqual(got.CustomFields, seedCustomFields) {
		t.Fatalf("custom_fields %+v", got.CustomFields)
	}
	if got.StatusHistory != nil {
		t.Fatalf("history not requested: %+v", got.StatusHistory)
	}
	// The key names and order of one item are the contract the factory plugin parses.
	wantItem := `{"id":"customfield_10481","name":"Peer Reviewer","type":"userpicker","text":"Avery Example"}`
	if !strings.Contains(raw, `"custom_fields":[`+wantItem+`,`) {
		t.Fatalf("contract shape missing in %s", raw)
	}
	if strings.Contains(raw, "status_history") || strings.Contains(raw, `"summary"`) {
		t.Fatalf("only the requested fields belong in %s", raw)
	}
}

func TestJiraGetFieldsSingleCustomField(t *testing.T) {
	d, _, _ := testDeps()
	got, _ := runGet(t, d, "SDO-1", "--fields", "customfield_10538")
	if !reflect.DeepEqual(got.CustomFields, []domain.CustomField{seedCustomFields[1]}) {
		t.Fatalf("%+v", got.CustomFields)
	}
	// Token matching ignores case and surrounding space, and repeats collapse.
	d, _, _ = testDeps()
	got, _ = runGet(t, d, "SDO-1", "--fields", " CustomField_10601 , customfield_10481,customfield_10601")
	if !reflect.DeepEqual(got.CustomFields, []domain.CustomField{seedCustomFields[2], seedCustomFields[0]}) {
		t.Fatalf("%+v", got.CustomFields)
	}
}

func TestJiraGetFieldsEmptyOrAbsentCustomFieldIsOmitted(t *testing.T) {
	for _, field := range []string{"customfield_10602", "customfield_99999"} {
		d, _, _ := testDeps()
		got, raw := runGet(t, d, "SDO-1", "--fields", field)
		if got.Key != "SDO-1" || got.CustomFields != nil || strings.Contains(raw, "custom_fields") {
			t.Fatalf("%s: %s", field, raw)
		}
	}
}

func TestJiraGetFieldsHistory(t *testing.T) {
	d, _, _ := testDeps()
	got, raw := runGet(t, d, "SDO-1", "--fields", "history")
	want := []domain.StatusChange{
		{Created: "2026-09-01T09:30:00.000+0000", Author: "Alex Example", From: "To Do", To: "In Progress"},
		{Created: "2026-09-02T14:05:00.000+0000", Author: "Avery Example", From: "In Progress", To: "To Do"},
	}
	if !reflect.DeepEqual(got.StatusHistory, want) {
		t.Fatalf("%+v", got.StatusHistory)
	}
	if !strings.Contains(raw, `"status_history":[{"created":"2026-09-01T09:30:00.000+0000","author":"Alex Example","from":"To Do","to":"In Progress"}`) {
		t.Fatalf("contract shape missing in %s", raw)
	}
	if got.CustomFields != nil {
		t.Fatalf("custom not requested: %+v", got.CustomFields)
	}
}

func TestJiraGetFieldsCombinesWithIssueFields(t *testing.T) {
	d, _, _ := testDeps()
	got, raw := runGet(t, d, "SDO-1", "--fields", "summary,status,issuetype,created,custom,history,comment")
	if got.Status != "To Do" || got.IssueType != "Task" || got.Created != "2026-09-01T00:00:00Z" {
		t.Fatalf("%s", raw)
	}
	if len(got.CustomFields) != 3 || len(got.StatusHistory) != 2 {
		t.Fatalf("%s", raw)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(raw), &m)
	for _, key := range []string{"key", "summary", "status", "issuetype", "created", "custom_fields", "status_history"} {
		if _, ok := m[key]; !ok {
			t.Errorf("missing %q in %s", key, raw)
		}
	}
}

func TestJiraGetFieldsKeyTokenIsAccepted(t *testing.T) {
	d, _, _ := testDeps()
	got, raw := runGet(t, d, "SDO-1", "--fields", "key,status,issuetype,created,custom,history")
	if got.Key != "SDO-1" || got.Status != "To Do" || len(got.CustomFields) != 3 || len(got.StatusHistory) != 2 {
		t.Fatal(raw)
	}
}

func TestJiraGetFieldsHumanFormat(t *testing.T) {
	d, out, errw := testDeps()
	if code := Run([]string{"atlas", "--human", "jira", "get", "SDO-1", "--fields", "custom"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	if !strings.Contains(out.String(), `"Peer Reviewer"`) {
		t.Fatal(out.String())
	}
}

func TestJiraGetFieldsUnknownTokenIsUsage(t *testing.T) {
	for _, fields := range []string{"bogus", "custom,bogus", "customfield_", "customfield_abc", "customfield_12x", "customfield_1 2", "history,customfields", "customfield_10538;customfield_1"} {
		d, out, errw := testDeps()
		code := Run([]string{"atlas", "jira", "get", "SDO-1", "--fields", fields}, d)
		if code != domain.ExitUsage {
			t.Errorf("--fields %q: exit %d, stderr %s", fields, code, errw.String())
		}
		if out.Len() != 0 {
			t.Errorf("--fields %q wrote %s", fields, out.String())
		}
		var e map[string]any
		if err := json.Unmarshal(errw.Bytes(), &e); err != nil || e["class"] != "usage" {
			t.Errorf("--fields %q: %s", fields, errw.String())
		}
	}
}

func TestJiraGetDefaultOutputUnchanged(t *testing.T) {
	d, out, errw := testDeps()
	iss, err := d.Jira.Get(ctx(), "sesamidevel.atlassian.net", "SDO-1")
	if err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"atlas", "jira", "get", "SDO-1"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var got, want map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	wantJSON, _ := json.Marshal(iss)
	_ = json.Unmarshal(wantJSON, &want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("default output changed:\n got %v\nwant %v", got, want)
	}
	for _, key := range []string{"custom_fields", "status_history"} {
		if _, ok := got[key]; ok {
			t.Fatalf("default output has %q", key)
		}
	}
}

func TestJiraGetNonCustomFieldsDoNotCallCustomPath(t *testing.T) {
	d, out, errw := testDeps()
	if code := Run([]string{"atlas", "jira", "get", "SDO-1", "--fields", "summary,status"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var m map[string]any
	_ = json.Unmarshal(out.Bytes(), &m)
	if len(m) != 5 {
		t.Fatalf("expected key, site, browse_url, summary, status only: %v", m)
	}
}

func TestJiraGetFieldsUnknownKeyIsNotFound(t *testing.T) {
	d, _, errw := testDeps()
	if code := Run([]string{"atlas", "jira", "get", "SDO-999", "--fields", "custom,history"}, d); code != domain.ExitNotFound {
		t.Fatal(code, errw.String())
	}
}
