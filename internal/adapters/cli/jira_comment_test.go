package cli

import (
	"encoding/json"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestJiraCommentEditDeleteFlow(t *testing.T) {
	d, out, errw := testDeps()
	run := func(args ...string) map[string]any {
		t.Helper()
		out.Reset()
		errw.Reset()
		if code := Run(append([]string{"atlas"}, args...), d); code != domain.ExitOK {
			t.Fatalf("%v: exit=%d error=%s", args, code, errw.String())
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err, out.String())
		}
		return result
	}
	run("jira", "comment", "SDO-1", "--body", "original")
	listed := run("jira", "comments", "SDO-1")
	comments := listed["comments"].([]any)
	if len(comments) != 1 {
		t.Fatal(listed)
	}
	id := comments[0].(map[string]any)["id"].(string)
	preview := run("jira", "comment", "edit", "SDO-1", "--id", id, "--body", "replacement", "--dry-run")
	if preview["dry_run"] != true || preview["body"] != "replacement" || preview["current"].(map[string]any)["body"] != "original" {
		t.Fatal(preview)
	}
	if got := run("jira", "comments", "SDO-1")["comments"].([]any)[0].(map[string]any)["body"]; got != "original" {
		t.Fatalf("dry-run changed comment: %v", got)
	}
	run("jira", "comment", "edit", "SDO-1", "--id", id, "--body", "replacement")
	if got := run("jira", "comments", "SDO-1")["comments"].([]any)[0].(map[string]any)["body"]; got != "replacement" {
		t.Fatalf("edit did not persist: %v", got)
	}
	preview = run("jira", "comment", "delete", "SDO-1", "--id", id, "--dry-run")
	if preview["dry_run"] != true || preview["comment"].(map[string]any)["body"] != "replacement" {
		t.Fatal(preview)
	}
	if got := run("jira", "comments", "SDO-1")["total"]; got != float64(1) {
		t.Fatalf("dry-run removed comment: %v", got)
	}
	run("jira", "comment", "delete", "SDO-1", "--id", id)
	if got := run("jira", "comments", "SDO-1")["total"]; got != float64(0) {
		t.Fatalf("delete did not persist: %v", got)
	}
	out.Reset()
	errw.Reset()
	if code := Run([]string{"atlas", "jira", "comment", "edit", "SDO-1", "--id", id, "--body", "again"}, d); code != domain.ExitNotFound {
		t.Fatalf("deleted ID edit exit=%d err=%s", code, errw.String())
	}
}

func TestJiraCommentMCPGate(t *testing.T) {
	if !isRead("jira", "comments") || isRead("jira", "comment") || !isWrite("jira", "comment") {
		t.Fatal("incorrect MCP classification")
	}
	args, err := buildWriteArgs("jira", "comment", []string{"delete", "SDO-1"}, map[string]any{"id": "123"}, false)
	if err != nil || args[len(args)-1] != "--id=123" {
		t.Fatalf("MCP args=%v err=%v", args, err)
	}
	foundDryRun := false
	for _, arg := range args {
		if arg == "--dry-run=true" {
			foundDryRun = true
		}
	}
	if !foundDryRun {
		t.Fatalf("MCP delete bypassed dry-run: %v", args)
	}
}
