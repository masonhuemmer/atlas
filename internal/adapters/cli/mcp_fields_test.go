package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian"
	"github.com/masonhuemmer/atlas/internal/domain"
)

// fields reaches the CLI as one --fields=<json> token, so it must be passed
// to MCP as a JSON string and still sits behind the write gate.
func TestMCPJiraCreateFieldsStringStaysGated(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.Jira.(atlassian.JiraAPI).Memory
	before := mem.IssueCount()
	cs := connectMCP(t, d)
	flags := map[string]any{"project": "SDO", "type": "Task", "summary": "gated", "fields": `{"customfield_10538":"first\n\nsecond"}`}

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "jira", Verb: "create", Flags: flags}})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var preview struct {
		DryRun bool           `json:"dry_run"`
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal([]byte(toolText(t, res)), &preview); err != nil || !preview.DryRun {
		t.Fatal(err, toolText(t, res))
	}
	if doc, _ := preview.Fields["customfield_10538"].(map[string]any); doc["type"] != "doc" {
		t.Fatalf("preview should show the converted payload: %v", preview.Fields)
	}
	if mem.IssueCount() != before {
		t.Fatal("an issue was created without write_opt_in")
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "jira", Verb: "create", Flags: flags, WriteOptIn: true}})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var created domain.Issue
	if err := json.Unmarshal([]byte(toolText(t, res)), &created); err != nil || created.Key == "" {
		t.Fatal(err, toolText(t, res))
	}
	if mem.IssueCount() != before+1 {
		t.Fatalf("count %d", mem.IssueCount())
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_read", Arguments: readIn{
		Namespace: "jira", Verb: "get", Args: []string{created.Key}, Flags: map[string]any{"fields": "custom,history"},
	}})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var got getOut
	if err := json.Unmarshal([]byte(toolText(t, res)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.CustomFields) != 1 || got.CustomFields[0].Text != "first\nsecond" {
		t.Fatalf("%s", toolText(t, res))
	}
}

func TestMCPJiraFieldsObjectIsUsage(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.Jira.(atlassian.JiraAPI).Memory
	before := mem.IssueCount()
	cs := connectMCP(t, d)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{
		Namespace: "jira", Verb: "create", WriteOptIn: true,
		Flags: map[string]any{"project": "SDO", "type": "Task", "summary": "s", "fields": map[string]any{"customfield_10538": "x"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	assertClass(t, res, "usage")
	if mem.IssueCount() != before {
		t.Fatal("created an issue from an unsupported fields type")
	}
}

// A value inside the fields JSON that looks like a flag is data: the whole
// object is one --fields=<json> token, so it cannot turn off the dry-run.
func TestMCPJiraFieldsValueCannotInjectFlags(t *testing.T) {
	for _, v := range []string{"--dry-run=false", "--json", "--", "-h"} {
		d, _, _ := testDeps()
		mem := d.Jira.(atlassian.JiraAPI).Memory
		before := mem.IssueCount()
		cs := connectMCP(t, d)
		raw, _ := json.Marshal(map[string]any{"customfield_10538": v})
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{
			Namespace: "jira", Verb: "create",
			Flags: map[string]any{"project": "SDO", "type": "Task", "summary": "s", "fields": string(raw)},
		}})
		if err != nil {
			t.Fatal(err)
		}
		if mem.IssueCount() != before {
			t.Fatalf("value %q created an issue without write_opt_in: %s", v, toolText(t, res))
		}
		if !strings.Contains(toolText(t, res), `"dry_run":true`) {
			t.Fatalf("value %q: %s", v, toolText(t, res))
		}
	}
}

func TestMCPJiraEditFieldsStringStaysGated(t *testing.T) {
	d, _, _ := testDeps()
	cs := connectMCP(t, d)
	flags := map[string]any{"fields": `{"customfield_10538":"changed"}`}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "jira", Verb: "edit", Args: []string{"SDO-1"}, Flags: flags}})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	got, _ := runGet(t, d, "SDO-1", "--fields", "customfield_10538")
	if got.CustomFields[0].Text != "Alex Example ; Avery Example" {
		t.Fatalf("edit without write_opt_in changed the issue: %+v", got.CustomFields)
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "jira", Verb: "edit", Args: []string{"SDO-1"}, Flags: flags, WriteOptIn: true}})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	got, _ = runGet(t, d, "SDO-1", "--fields", "customfield_10538")
	if got.CustomFields[0].Text != "changed" {
		t.Fatalf("%+v", got.CustomFields)
	}
}
