package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPHelpNoSession(t *testing.T) {
	d, _, _ := testDeps()
	cs := connectMCP(t, d)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_help", Arguments: helpIn{}})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	root := toolText(t, res)
	if !strings.Contains(root, "mcp") {
		t.Fatal(root)
	}
	for _, topic := range recipeNames {
		if !strings.Contains(root, topic) {
			t.Fatalf("overview missing %s in %s", topic, root)
		}
	}
	for _, want := range []string{"atlas://skill", "topic=atlas"} {
		if !strings.Contains(root, want) {
			t.Fatalf("overview missing %s in %s", want, root)
		}
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_help", Arguments: helpIn{Namespace: "auth"},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	if !strings.Contains(toolText(t, res), "status") {
		t.Fatal(toolText(t, res))
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_help", Arguments: helpIn{Namespace: "mcp"},
	})
	if err != nil || res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	if !strings.Contains(toolText(t, res), "write_opt_in") {
		t.Fatal(toolText(t, res))
	}
}

func TestMCPHelpTopics(t *testing.T) {
	d, _, _ := testDeps()
	cs := connectMCP(t, d)
	cases := map[string][]string{
		"jira-search":      {"jira get KEY-1", "Never dual-query", "atlas jsm"},
		"confluence-write": {"space = KEY", "move PAGE_ID", "write_opt_in"},
		"pr-review":        {"defaults.workspace", "pr edit", "pr merge", "No PR delete", "auth login --workspace"},
		"jsm-customer":     {"jsm_customer", "public: true", "jsm desks"},
	}
	for topic, wants := range cases {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "atlas_help", Arguments: helpIn{Topic: topic},
		})
		if err != nil || res.IsError {
			t.Fatal(topic, err, toolText(t, res))
		}
		body := toolText(t, res)
		for _, w := range wants {
			if !strings.Contains(body, w) {
				t.Fatalf("%s missing %q in %s", topic, w, body)
			}
		}
	}
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_help", Arguments: helpIn{Topic: "nope"},
	})
	if err != nil || !res.IsError {
		t.Fatal(err, toolText(t, res))
	}
	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "atlas_help", Arguments: helpIn{Topic: "atlas", Namespace: "jira"},
	})
	if err != nil || !res.IsError {
		t.Fatal(err, toolText(t, res))
	}
}

func TestPromptsListFourRecipes(t *testing.T) {
	d, _, _ := testDeps()
	cs := connectMCP(t, d)
	pl, err := cs.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, p := range pl.Prompts {
		got[p.Name] = true
	}
	for _, name := range recipeNames {
		if !got[name] {
			t.Fatalf("missing prompt %s in %#v", name, got)
		}
	}
	if len(pl.Prompts) != 4 {
		t.Fatalf("count %d", len(pl.Prompts))
	}
	tl, err := cs.ListTools(context.Background(), nil)
	if err != nil || len(tl.Tools) != 4 {
		t.Fatal(err, len(tl.Tools))
	}
}

// atlas_help is marked read-only: help for any verb, write or auth, must
// print help and never run the verb.
func TestMCPHelpNeverRunsVerb(t *testing.T) {
	d, _, _ := testDeps()
	cs := connectMCP(t, d)
	verbs := map[string][]string{
		"auth":       {"status", "login", "logout"},
		"site":       {"list", "resolve"},
		"jira":       {"get", "search", "users", "create", "edit", "comment", "transition", "link"},
		"confluence": {"get", "search", "create", "update", "move"},
		"pr":         {"get", "list", "create", "edit", "comment", "merge", "diff"},
		"jsm":        {"desks", "types", "list", "get", "create", "comment", "transition"},
	}
	var pairs [][2]string
	for ns, vs := range verbs {
		for _, v := range vs {
			pairs = append(pairs, [2]string{ns, v})
		}
	}
	for _, p := range pairs {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "atlas_help", Arguments: helpIn{Namespace: p[0], Verb: p[1]},
		})
		if err != nil {
			t.Fatal(p, err)
		}
		text := toolText(t, res)
		if res.IsError {
			t.Fatalf("%s %s: %s", p[0], p[1], text)
		}
		if !strings.Contains(text, "Verbs:") {
			t.Fatalf("%s %s did not return help: %s", p[0], p[1], text)
		}
	}
}
