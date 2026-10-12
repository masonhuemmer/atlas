package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian"
	"github.com/masonhuemmer/atlas/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPRCreateMinimumTitleSource(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "pr", "create", "--repo", "atlas", "--title", "Phase 6 create", "--source", "feature/pr-cli"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var created domain.PullRequest
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		t.Fatal(err, out.String())
	}
	if created.ID != 2 {
		t.Fatalf("id %d", created.ID)
	}
	if created.Title != "Phase 6 create" || created.Source != "feature/pr-cli" {
		t.Fatalf("%+v", created)
	}
	if created.Target != domain.DefaultTargetBranch {
		t.Fatalf("target %q", created.Target)
	}
	if created.Workspace != domain.DefaultWorkspace() || created.State != "OPEN" {
		t.Fatalf("%+v", created)
	}
	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "pr", "get", "--repo", "atlas", "--id", "2"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
}

func TestPRCreateMissingSourceIsUsage(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "pr", "create", "--repo", "atlas", "--title", "no source"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
}

func TestPRCreateDryRunDoesNotPersist(t *testing.T) {
	d, out, errw := testDeps()
	mem := d.PR.(atlassian.PRAPI).Memory
	before := mem.PRCount()
	code := Run([]string{"atlas", "pr", "create", "--repo", "atlas", "--title", "dry", "--source", "feature/x", "--dry-run"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err, out.String())
	}
	if preview["dry_run"] != true || preview["namespace"] != "pr" || preview["verb"] != "create" {
		t.Fatalf("%v", preview)
	}
	if mem.PRCount() != before {
		t.Fatal("seed PR count changed on dry-run")
	}
}

func TestPREditDescriptionAndTitle(t *testing.T) {
	d, out, errw := testDeps()
	mem := d.PR.(atlassian.PRAPI).Memory
	before, err := mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"atlas", "pr", "edit", "--repo", "atlas", "--id", "1", "--description", "## Updated\n\nDetails"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var edited domain.PullRequest
	if err := json.Unmarshal(out.Bytes(), &edited); err != nil {
		t.Fatal(err)
	}
	if edited.Title != before.Title || edited.Description != "## Updated\n\nDetails" {
		t.Fatalf("%+v", edited)
	}
	out.Reset()
	if code := Run([]string{"atlas", "pr", "edit", "--repo", "atlas", "--id", "1", "--title", "New title"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	got, err := mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil || got.Title != "New title" || got.Description != edited.Description {
		t.Fatalf("%v %+v", err, got)
	}
	if code := Run([]string{"atlas", "pr", "edit", "--repo", "atlas", "--id", "1", "--description", ""}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	got, err = mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil || got.Description != "" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestPREditDryRunAndValidation(t *testing.T) {
	d, out, errw := testDeps()
	mem := d.PR.(atlassian.PRAPI).Memory
	before, err := mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil {
		t.Fatal(err)
	}
	if code := Run([]string{"atlas", "pr", "edit", "--repo", "atlas", "--id", "1", "--description", "preview", "--dry-run"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var preview map[string]any
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if preview["dry_run"] != true || preview["verb"] != "edit" || preview["description"] != "preview" {
		t.Fatal(preview)
	}
	got, err := mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil || got.Description != before.Description {
		t.Fatalf("%v %+v", err, got)
	}
	for _, args := range [][]string{
		{"atlas", "pr", "edit", "--repo", "atlas", "--id", "1"},
		{"atlas", "pr", "edit", "--repo", "atlas", "--id", "1", "--title", ""},
	} {
		if code := Run(args, d); code != domain.ExitUsage {
			t.Fatalf("%v: exit %d", args, code)
		}
	}
}

func TestMCPPREditWriteGate(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.PR.(atlassian.PRAPI).Memory
	cs := connectMCP(t, d)
	args := writeIn{Namespace: "pr", Verb: "edit", Flags: map[string]any{
		"repo": "atlas", "id": float64(1), "description": "MCP edit",
	}}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: args})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var preview map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &preview); err != nil || preview["dry_run"] != true {
		t.Fatal(err, toolText(t, res))
	}
	got, err := mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil || got.Description == "MCP edit" {
		t.Fatalf("%v %+v", err, got)
	}
	args.WriteOptIn = true
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: args})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	got, err = mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil || got.Description != "MCP edit" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestPRApproveDryRunAndWrite(t *testing.T) {
	d, out, errw := testDeps()
	mem := d.PR.(atlassian.PRAPI).Memory
	args := []string{"atlas", "pr", "approve", "--repo", "atlas", "--id", "1"}
	code := Run(append(append([]string(nil), args...), "--dry-run"), d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var preview prDryRun
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil || !preview.DryRun || preview.Verb != "approve" {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	got, err := mem.ApprovePR(context.Background(), domain.DefaultWorkspace(), "atlas", 1, true)
	if err != nil || got.Approved {
		t.Fatalf("dry-run changed approval: %+v %v", got, err)
	}
	out.Reset()
	errw.Reset()
	if code = Run(args, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var approval domain.PullRequestApproval
	if err := json.Unmarshal(out.Bytes(), &approval); err != nil || !approval.Approved || approval.ID != 1 {
		t.Fatalf("approval=%+v err=%v", approval, err)
	}
	got, err = mem.ApprovePR(context.Background(), domain.DefaultWorkspace(), "atlas", 1, true)
	if err != nil || !got.Approved {
		t.Fatalf("approval not persisted: %+v %v", got, err)
	}
	out.Reset()
	if code = Run([]string{"atlas", "pr", "get", "--repo", "atlas", "--id", "1"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var pr domain.PullRequest
	if err := json.Unmarshal(out.Bytes(), &pr); err != nil || len(pr.Participants) != 1 || !pr.Participants[0].Approved {
		t.Fatalf("readback=%s err=%v", out.String(), err)
	}
}

func TestMCPPRApproveWriteGate(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.PR.(atlassian.PRAPI).Memory
	cs := connectMCP(t, d)
	flags := map[string]any{"repo": "atlas", "id": float64(1)}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{Namespace: "pr", Verb: "approve", Flags: flags},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var preview prDryRun
	if err := json.Unmarshal([]byte(toolText(t, res)), &preview); err != nil || !preview.DryRun {
		t.Fatal(err, toolText(t, res))
	}
	got, err := mem.ApprovePR(context.Background(), domain.DefaultWorkspace(), "atlas", 1, true)
	if err != nil || got.Approved {
		t.Fatalf("approval changed without opt-in: %+v %v", got, err)
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{Namespace: "pr", Verb: "approve", Flags: flags, WriteOptIn: true},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var approval domain.PullRequestApproval
	if err := json.Unmarshal([]byte(toolText(t, res)), &approval); err != nil || !approval.Approved {
		t.Fatal(err, toolText(t, res))
	}
}

func TestMCPPRMergeWriteGate(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.PR.(atlassian.PRAPI).Memory
	cs := connectMCP(t, d)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{
			Namespace: "pr", Verb: "merge",
			Flags: map[string]any{"repo": "atlas", "id": float64(1)},
		},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var preview map[string]any
	if err := json.Unmarshal([]byte(toolText(t, res)), &preview); err != nil {
		t.Fatal(toolText(t, res))
	}
	if preview["dry_run"] != true {
		t.Fatal(toolText(t, res))
	}
	if preview["namespace"] != "pr" || preview["verb"] != "merge" {
		t.Fatal(toolText(t, res))
	}
	got, err := mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "OPEN" {
		t.Fatalf("seed PR merged without write_opt_in %+v", got)
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{
			Namespace: "pr", Verb: "merge", WriteOptIn: true,
			Flags: map[string]any{"repo": "atlas", "id": float64(1)},
		},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var merged domain.PullRequest
	if err := json.Unmarshal([]byte(toolText(t, res)), &merged); err != nil {
		t.Fatal(toolText(t, res))
	}
	if merged.State != "MERGED" {
		t.Fatalf("%+v", merged)
	}
	got, err = mem.GetPR(context.Background(), domain.DefaultWorkspace(), "atlas", 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "MERGED" {
		t.Fatalf("%+v", got)
	}
}
