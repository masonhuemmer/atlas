package cli

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian"
	"github.com/masonhuemmer/atlas/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConfluenceCreateDryRunDoesNotPersist(t *testing.T) {
	d, out, errw := testDeps()
	mem := d.Confluence.(atlassian.ConfluenceAPI).Memory
	before := mem.PageCount()
	code := Run([]string{"atlas", "confluence", "create", "--space", "CCAB", "--title", "CAB-110", "--body", "draft", "--dry-run"}, d)
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
	if preview["namespace"] != "confluence" || preview["verb"] != "create" {
		t.Fatalf("%v", preview)
	}
	if preview["space"] != "CCAB" || preview["title"] != "CAB-110" {
		t.Fatalf("%v", preview)
	}
	if mem.PageCount() != before {
		t.Fatal("seed page count changed on dry-run")
	}
}

func TestConfluenceCreatePersistsAndGetFindsPage(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "create", "--space", "CCAB", "--title", "CAB-110", "--body", "markdown body"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var created domain.Page
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		t.Fatal(err, out.String())
	}
	if created.ID == "" || created.Title != "CAB-110" {
		t.Fatalf("%+v", created)
	}
	if created.Site != "sesami-io.atlassian.net" || created.Space != "CCAB" {
		t.Fatalf("%+v", created)
	}
	if created.ContentFormat != domain.StorageBodyFormat {
		t.Fatalf("format %q", created.ContentFormat)
	}
	out.Reset()
	errw.Reset()
	code = Run([]string{"atlas", "confluence", "get", created.ID, "--site", "sesami-io"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var got domain.Page
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err, out.String())
	}
	if got.ID != created.ID || got.Body != "<p>markdown body</p>\n" {
		t.Fatalf("%+v", got)
	}
}

func TestConfluenceUpdatePersists(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "update", "100", "--body", "revised", "--site", "sesami-io"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var page domain.Page
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err, out.String())
	}
	if page.Body != "<p>revised</p>\n" || page.Version != 2 {
		t.Fatalf("%+v", page)
	}
}

func TestConfluenceCreatePreparedStorage(t *testing.T) {
	d, out, errw := testDeps()
	body := `<ac:structured-macro ac:name="code" />`
	code := Run([]string{"atlas", "confluence", "create", "--space", "CCAB", "--title", "Macro", "--body", body, "--body-format", "storage"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var page domain.Page
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Body != body || page.ContentFormat != domain.StorageBodyFormat {
		t.Fatalf("%+v", page)
	}
}

func TestConfluenceCreateRejectsUnknownBodyFormat(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "create", "--space", "CCAB", "--title", "Bad", "--body", "text", "--body-format", "wiki"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
}

func TestMCPConfluenceCreateWriteGate(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.Confluence.(atlassian.ConfluenceAPI).Memory
	before := mem.PageCount()
	cs := connectMCP(t, d)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{
			Namespace: "confluence", Verb: "create",
			Flags: map[string]any{"space": "CCAB", "title": "gated", "body": "nope"},
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
	if preview["namespace"] != "confluence" || preview["verb"] != "create" {
		t.Fatal(toolText(t, res))
	}
	if mem.PageCount() != before {
		t.Fatal("seed page count changed on dry-run")
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{
			Namespace: "confluence", Verb: "create", WriteOptIn: true,
			Flags: map[string]any{"space": "CCAB", "title": "gated", "body": "yes"},
		},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	var created domain.Page
	if err := json.Unmarshal([]byte(toolText(t, res)), &created); err != nil {
		t.Fatal(toolText(t, res))
	}
	if created.ID == "" || created.Title != "gated" {
		t.Fatalf("%+v", created)
	}
	if mem.PageCount() != before+1 {
		t.Fatalf("count %d", mem.PageCount())
	}

	got, err := mem.GetPage(context.Background(), created.Site, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "gated" || got.Body != "<p>yes</p>\n" {
		t.Fatalf("%+v", got)
	}
}

func TestMCPConfluenceUpdateWriteGate(t *testing.T) {
	d, _, _ := testDeps()
	mem := d.Confluence.(atlassian.ConfluenceAPI).Memory
	cs := connectMCP(t, d)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{
			Namespace: "confluence", Verb: "update",
			Args:  []string{"100"},
			Flags: map[string]any{"body": "should not persist", "site": "sesami-io"},
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
	got, err := mem.GetPage(context.Background(), "sesami-io.atlassian.net", "100")
	if err != nil {
		t.Fatal(err)
	}
	if got.Body == "should not persist" {
		t.Fatal("update persisted without write_opt_in")
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_write", Arguments: writeIn{
			Namespace: "confluence", Verb: "update", WriteOptIn: true,
			Args:  []string{"100"},
			Flags: map[string]any{"body": "opted in", "site": "sesami-io"},
		},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	got, err = mem.GetPage(context.Background(), "sesami-io.atlassian.net", "100")
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "<p>opted in</p>\n" {
		t.Fatalf("%+v", got)
	}
}
