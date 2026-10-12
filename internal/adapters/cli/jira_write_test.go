package cli

import (
	"encoding/json"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestJiraCreatePersistsAndGetFindsKey(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "create", "--project", "SDO", "--type", "Task", "--summary", "Phase 3 write"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var created domain.Issue
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		t.Fatal(err, out.String())
	}
	if created.Key != "SDO-2" {
		t.Fatalf("key %q", created.Key)
	}
	if created.Site != "sesamidevel.atlassian.net" {
		t.Fatalf("site %q", created.Site)
	}
	if created.Assignee != domain.DefaultAssigneeAccountID() {
		t.Fatalf("assignee %q", created.Assignee)
	}
	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "jira", "get", created.Key}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var got domain.Issue
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err, out.String())
	}
	if got.Summary != "Phase 3 write" || got.Key != created.Key {
		t.Fatalf("%+v", got)
	}
}

func TestJiraCommentDryRunDoesNotAppend(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "comment", "SDO-1", "--body", "do not persist", "--dry-run"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err, out.String())
	}
	if preview["dry_run"] != true {
		t.Fatalf("%v", preview)
	}
	if preview["namespace"] != "jira" || preview["verb"] != "comment" {
		t.Fatalf("%v", preview)
	}
	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "jira", "get", "SDO-1"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err, out.String())
	}
	if len(iss.Comments) != 0 {
		t.Fatalf("comments %v", iss.Comments)
	}
}

func TestJiraCommentPersists(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "comment", "SDO-1", "--body", "ship it"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "jira", "get", "SDO-1"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err, out.String())
	}
	if len(iss.Comments) != 1 || iss.Comments[0] != "ship it" {
		t.Fatalf("comments %v", iss.Comments)
	}
}

func TestJiraCommentReadBackWithFields(t *testing.T) {
	d, out, errw := testDeps()
	if code := Run([]string{"atlas", "jira", "comment", "SDO-1", "--body", "benchmark verification"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	out.Reset()
	errw.Reset()
	if code := Run([]string{"atlas", "jira", "get", "SDO-1", "--fields", "comment"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var got struct {
		Key      string   `json:"key"`
		Comments []string `json:"comments"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || got.Key != "SDO-1" || len(got.Comments) != 1 || got.Comments[0] != "benchmark verification" {
		t.Fatalf("output=%s err=%v", out.String(), err)
	}
}

func TestJiraInternalCommentDryRun(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "comment", "SDO-1", "--body", "private update", "--internal", "--dry-run"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview["internal"] != true || preview["dry_run"] != true {
		t.Fatalf("%v", preview)
	}
	out.Reset()
	if code := Run([]string{"atlas", "jira", "get", "SDO-1"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err)
	}
	if len(iss.Comments) != 0 {
		t.Fatalf("comments %v", iss.Comments)
	}
}

func TestJiraInternalCommentRequiresCustomerRequest(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "comment", "SDO-1", "--body", "private update", "--internal"}, d)
	if code != domain.ExitNotFound {
		t.Fatal(code, errw.String())
	}
	if code := Run([]string{"atlas", "jira", "get", "SDO-1"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err)
	}
	if len(iss.Comments) != 0 {
		t.Fatalf("public comment created: %v", iss.Comments)
	}
}

func TestJiraTransitionSDO1Done(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "transition", "SDO-1", "--name", "Done"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err, out.String())
	}
	if iss.Status != "Done" {
		t.Fatalf("status %q", iss.Status)
	}
	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "jira", "get", "SDO-1"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err, out.String())
	}
	if iss.Status != "Done" {
		t.Fatalf("status %q", iss.Status)
	}
}

func TestJiraTransitionUnknownNameIsUsage(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "jira", "transition", "SDO-1", "--name", "Yeet"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
	var e map[string]any
	if err := json.Unmarshal(errw.Bytes(), &e); err != nil {
		t.Fatal(errw.String())
	}
	if e["class"] != "usage" {
		t.Fatalf("%v", e)
	}
}

func TestJiraCreateSDPStoryPersists(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "create", "--project", "SDP", "--type", "Story", "--summary", "allowed"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var created domain.Issue
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		t.Fatal(err, out.String())
	}
	if created.IssueType != "Story" || created.Project != "SDP" {
		t.Fatalf("%+v", created)
	}
}

func TestJiraEditFields(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "edit", "SDO-1", "--fields", `{"summary":"renamed"}`}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err, out.String())
	}
	if iss.Summary != "renamed" {
		t.Fatalf("%+v", iss)
	}
}

func TestJiraSubtaskCreateAndReparent(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "create", "--project", "SES", "--type", "Sub-task", "--summary", "Investigate", "--parent", "SES-1", "--dry-run"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview["dry_run"] != true || preview["parent"] != "SES-1" {
		t.Fatalf("%v", preview)
	}
	out.Reset()
	code = Run([]string{"atlas", "jira", "create", "--project", "SES", "--type", "Sub-task", "--summary", "Investigate", "--parent", "SES-1"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var subtask domain.Issue
	if err := json.Unmarshal(out.Bytes(), &subtask); err != nil {
		t.Fatal(err)
	}
	if subtask.Key != "SES-2" || subtask.Parent != "SES-1" || subtask.IssueType != "Sub-task" {
		t.Fatalf("%+v", subtask)
	}
	out.Reset()
	code = Run([]string{"atlas", "jira", "create", "--project", "SES", "--type", "Story", "--summary", "Other story"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var story domain.Issue
	if err := json.Unmarshal(out.Bytes(), &story); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code = Run([]string{"atlas", "jira", "edit", subtask.Key, "--parent", story.Key}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	out.Reset()
	code = Run([]string{"atlas", "jira", "get", subtask.Key}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	if err := json.Unmarshal(out.Bytes(), &subtask); err != nil {
		t.Fatal(err)
	}
	if subtask.Parent != story.Key {
		t.Fatalf("parent %q, want %q", subtask.Parent, story.Key)
	}
}

func TestJiraSubtaskParentMustShareProject(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "jira", "create", "--project", "SES", "--type", "Sub-task", "--summary", "Investigate", "--parent", "CAB-1"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
}

func hasRelatesLink(iss domain.Issue, inward, outward string) bool {
	for _, l := range iss.Links {
		if l.Type == domain.DefaultLinkType && l.Inward == inward && l.Outward == outward {
			return true
		}
	}
	return false
}

func TestJiraLinkSDO1SDP2CreatesRelates(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "link", "SDO-1", "SDP-2"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err, out.String())
	}
	if res["type"] != domain.DefaultLinkType {
		t.Fatalf("default type %v", res["type"])
	}

	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "jira", "get", "SDO-1"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var inward domain.Issue
	if err := json.Unmarshal(out.Bytes(), &inward); err != nil {
		t.Fatal(err, out.String())
	}
	if !hasRelatesLink(inward, "SDO-1", "SDP-2") {
		t.Fatalf("inward links %+v", inward.Links)
	}

	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "jira", "get", "SDP-2"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var outward domain.Issue
	if err := json.Unmarshal(out.Bytes(), &outward); err != nil {
		t.Fatal(err, out.String())
	}
	if !hasRelatesLink(outward, "SDO-1", "SDP-2") {
		t.Fatalf("outward links %+v", outward.Links)
	}
}

func TestJiraLinkSDO1CAB1IsUsage(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "link", "SDO-1", "CAB-1"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
	var e map[string]any
	if err := json.Unmarshal(errw.Bytes(), &e); err != nil {
		t.Fatal(errw.String())
	}
	if e["class"] != "usage" {
		t.Fatalf("%v", e)
	}

	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "jira", "get", "SDO-1"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err, out.String())
	}
	if len(iss.Links) != 0 {
		t.Fatalf("cross-cloud link persisted %+v", iss.Links)
	}
}

func TestJiraLinkDefaultTypeIsRelates(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "link", "SDO-1", "SDP-2"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var res map[string]any
	if err := json.Unmarshal(out.Bytes(), &res); err != nil {
		t.Fatal(err, out.String())
	}
	if res["type"] != "Relates" {
		t.Fatalf("type %v", res["type"])
	}
	inward, ok := res["inward"].(map[string]any)
	if !ok {
		t.Fatalf("%v", res)
	}
	links, ok := inward["links"].([]any)
	if !ok || len(links) == 0 {
		t.Fatalf("links %v", inward["links"])
	}
	first, ok := links[0].(map[string]any)
	if !ok {
		t.Fatalf("%v", links[0])
	}
	if first["type"] != "Relates" {
		t.Fatalf("%v", first)
	}
}
