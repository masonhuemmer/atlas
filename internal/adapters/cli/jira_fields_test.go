package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian"
	"github.com/masonhuemmer/atlas/internal/domain"
)

func runCreate(t *testing.T, d Deps, args ...string) (int, string, string) {
	t.Helper()
	out, errw := d.Stdout.(*bytes.Buffer), d.Stderr.(*bytes.Buffer)
	out.Reset()
	errw.Reset()
	code := Run(append([]string{"atlas", "jira", "create"}, args...), d)
	return code, out.String(), errw.String()
}

func TestJiraCreateFieldsMergesIntoIssue(t *testing.T) {
	d, _, _ := testDeps()
	code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "with custom data",
		"--fields", `{"customfield_10538":"first\n\nsecond","customfield_10481":{"accountId":"acct-alex","displayName":"Alex Example"}}`)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var created domain.Issue
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatal(err, out)
	}
	if created.Key != "SDO-2" || created.Summary != "with custom data" {
		t.Fatalf("%+v", created)
	}
	got, _ := runGet(t, d, created.Key, "--fields", "custom")
	want := []domain.CustomField{
		{ID: "customfield_10481", Name: "Peer Reviewer", Type: "userpicker", Text: "Alex Example"},
		{ID: "customfield_10538", Name: "Notification of Changes", Type: "textarea", Text: "first\nsecond"},
	}
	if !reflect.DeepEqual(got.CustomFields, want) {
		t.Fatalf("%+v", got.CustomFields)
	}
}

func TestJiraCreateFieldsMergesPlainFieldsToo(t *testing.T) {
	d, _, _ := testDeps()
	code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "s", "--fields", `{"priority":"High","description":"from fields"}`)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var created domain.Issue
	_ = json.Unmarshal([]byte(out), &created)
	if created.Priority != "High" || created.Description != "from fields" {
		t.Fatalf("%+v", created)
	}
}

func TestJiraCreateFieldsReservedKeysAreUsage(t *testing.T) {
	for _, fields := range []string{`{"summary":"x"}`, `{"project":{"key":"SES"}}`, `{"issuetype":{"name":"Story"}}`, `{"Summary":"x"}`, `{"customfield_10538":"ok","project":"x"}`} {
		d, _, _ := testDeps()
		mem := d.Jira.(atlassian.JiraAPI).Memory
		before := mem.IssueCount()
		code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "s", "--fields", fields)
		if code != domain.ExitUsage || out != "" || mem.IssueCount() != before {
			t.Errorf("%s: exit %d, stdout %q, stderr %s", fields, code, out, errw)
		}
		if !strings.Contains(errw, "--project, --type, and --summary") {
			t.Errorf("%s: hint should name the flags: %s", fields, errw)
		}
	}
}

func TestJiraCreateFieldsConflictWithFlagsIsUsage(t *testing.T) {
	cases := []struct {
		name   string
		flags  []string
		fields string
	}{
		{"description", []string{"--description", "d"}, `{"description":"other"}`},
		{"parent", []string{"--type", "Sub-task", "--project", "SES", "--parent", "SES-1"}, `{"parent":{"key":"SES-1"}}`},
		{"labels", []string{"--labels", "a,b"}, `{"labels":["c"]}`},
		{"assignee", []string{"--assignee", "acct-alex"}, `{"assignee":{"accountId":"acct-avery"}}`},
	}
	for _, tc := range cases {
		d, _, _ := testDeps()
		mem := d.Jira.(atlassian.JiraAPI).Memory
		before := mem.IssueCount()
		args := append([]string{"--project", "SDO", "--type", "Task", "--summary", "s"}, tc.flags...)
		args = append(args, "--fields", tc.fields)
		code, _, errw := runCreate(t, d, args...)
		if code != domain.ExitUsage || mem.IssueCount() != before {
			t.Errorf("%s: exit %d: %s", tc.name, code, errw)
		}
		if !strings.Contains(errw, tc.name) || !strings.Contains(errw, "both") {
			t.Errorf("%s: message should name the clash: %s", tc.name, errw)
		}
	}
}

func TestJiraCreateFieldsAssigneeSkipsDefault(t *testing.T) {
	d, _, _ := testDeps()
	code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "s", "--fields", `{"assignee":{"accountId":"acct-avery"}}`)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var created domain.Issue
	_ = json.Unmarshal([]byte(out), &created)
	if created.Assignee != "acct-avery" {
		t.Fatalf("assignee %q, the default must not replace --fields", created.Assignee)
	}
	d, _, _ = testDeps()
	code, out, _ = runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "s", "--fields", `{"customfield_10538":"x"}`)
	_ = json.Unmarshal([]byte(out), &created)
	if code != domain.ExitOK || created.Assignee != domain.DefaultAssigneeAccountID() {
		t.Fatalf("without assignee in --fields the default applies: %q", created.Assignee)
	}
}

func TestJiraCreateFieldsRejectsNonObject(t *testing.T) {
	for _, fields := range []string{`not json`, `[]`, `null`, `"x"`, `{`} {
		d, _, _ := testDeps()
		code, _, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "s", "--fields", fields)
		if code != domain.ExitUsage {
			t.Errorf("%s: exit %d %s", fields, code, errw)
		}
		if strings.Contains(errw, "edit requires") {
			t.Errorf("%s: create reported an edit message: %s", fields, errw)
		}
	}
}

func TestJiraEditFieldsInvalidJSONKeepsEditHint(t *testing.T) {
	d, _, errw := testDeps()
	if code := Run([]string{"atlas", "jira", "edit", "SDO-1", "--fields", "not json"}, d); code != domain.ExitUsage {
		t.Fatal(code)
	}
	if !strings.Contains(errw.String(), "--fields") || !strings.Contains(errw.String(), "atlas jira edit") {
		t.Fatalf("edit keeps its own hint: %s", errw.String())
	}
}

func TestJiraCreateDryRunEchoesFinalFields(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.Jira.(atlassian.JiraAPI).Memory
	before := mem.IssueCount()
	code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "preview", "--description", "**d**",
		"--fields", `{"customfield_10538":"first\n\nsecond","customfield_10601":{"value":"EMEA","child":{"value":"Frankfurt"}}}`, "--dry-run")
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	if mem.IssueCount() != before {
		t.Fatal("dry-run created an issue")
	}
	var preview struct {
		DryRun  bool           `json:"dry_run"`
		Verb    string         `json:"verb"`
		Project string         `json:"project"`
		Summary string         `json:"summary"`
		Fields  map[string]any `json:"fields"`
	}
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatal(err, out)
	}
	if !preview.DryRun || preview.Verb != "create" || preview.Project != "SDO" || preview.Summary != "preview" {
		t.Fatalf("existing keys changed: %s", out)
	}
	f := preview.Fields
	doc, _ := f["customfield_10538"].(map[string]any)
	if doc["type"] != "doc" || len(doc["content"].([]any)) != 2 {
		t.Fatalf("textarea string not previewed as ADF: %v", f["customfield_10538"])
	}
	if !reflect.DeepEqual(f["customfield_10601"], map[string]any{"value": "EMEA", "child": map[string]any{"value": "Frankfurt"}}) {
		t.Fatalf("select changed: %v", f["customfield_10601"])
	}
	if f["summary"] != "preview" || f["project"].(map[string]any)["key"] != "SDO" || f["issuetype"].(map[string]any)["name"] != "Task" {
		t.Fatalf("flag fields missing from the merged payload: %v", f)
	}
	if desc, _ := f["description"].(map[string]any); desc["type"] != "doc" {
		t.Fatalf("description should preview as ADF: %v", f["description"])
	}
	if _, ok := f["assignee"].(map[string]any); !ok && domain.DefaultAssigneeAccountID() != "" {
		t.Fatalf("default assignee missing: %v", f)
	}
}

func TestJiraCreateDryRunWithoutFieldsKeepsOldShape(t *testing.T) {
	d, _, _ := testDeps()
	code, out, errw := runCreate(t, d, "--project", "SDO", "--type", "Task", "--summary", "s", "--dry-run")
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var preview map[string]any
	_ = json.Unmarshal([]byte(out), &preview)
	if _, ok := preview["fields"]; ok {
		t.Fatalf("fields echoed although --fields was not used: %s", out)
	}
}

func TestJiraEditFieldsTextareaParity(t *testing.T) {
	d, _, errw := testDeps()
	if code := Run([]string{"atlas", "jira", "edit", "SDO-1", "--fields", `{"customfield_10538":"new\n\ntext"}`, "--dry-run"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	got, _ := runGet(t, d, "SDO-1", "--fields", "customfield_10538")
	if got.CustomFields[0].Text != "Alex Example ; Avery Example" {
		t.Fatalf("dry-run edited the issue: %+v", got.CustomFields)
	}
	if code := Run([]string{"atlas", "jira", "edit", "SDO-1", "--fields", `{"customfield_10538":"new\n\ntext"}`}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	got, _ = runGet(t, d, "SDO-1", "--fields", "customfield_10538")
	if len(got.CustomFields) != 1 || got.CustomFields[0].Text != "new\ntext" {
		t.Fatalf("%+v", got.CustomFields)
	}
}

func TestJiraEditFieldsKeepsADFAndOtherTypes(t *testing.T) {
	d, _, errw := testDeps()
	fields := `{"customfield_10538":{"type":"doc","version":1,"content":[{"type":"paragraph","content":[{"type":"text","text":"given as ADF"}]}]},"customfield_10602":[{"value":"PROD"},{"value":"QA"}]}`
	if code := Run([]string{"atlas", "jira", "edit", "SDO-1", "--fields", fields}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	got, _ := runGet(t, d, "SDO-1", "--fields", "customfield_10538,customfield_10602")
	if len(got.CustomFields) != 2 || got.CustomFields[0].Text != "given as ADF" || got.CustomFields[1].Text != "PROD; QA" {
		t.Fatalf("%+v", got.CustomFields)
	}
}

func TestJiraEditStillRejectsUnknownPlainField(t *testing.T) {
	d, _, _ := testDeps()
	if code := Run([]string{"atlas", "jira", "edit", "SDO-1", "--fields", `{"nonsense":"x"}`}, d); code != domain.ExitUsage {
		t.Fatal(code)
	}
}

func TestJiraHelpAndSkillDescribeFields(t *testing.T) {
	for _, want := range []string{"--fields", "customfield_", "history"} {
		if !strings.Contains(jiraHelp, want) {
			t.Errorf("jira help missing %q", want)
		}
	}
	for _, want := range []string{"customfield_", "JSON string", "status_history"} {
		if !strings.Contains(skillMarkdown, want) {
			t.Errorf("skill.md missing %q", want)
		}
	}
}
