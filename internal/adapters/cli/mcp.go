package cli

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/masonhuemmer/atlas/internal/domain"
)

//go:embed skill.md
var skillMarkdown string

const skillURI = "atlas://skill"

const mcpHelp = `atlas mcp — stdio MCP for agents

Verbs: serve
serve: JSON-RPC on stdin/stdout. Tools: atlas_status, atlas_help, atlas_read, atlas_write.
Recipe topics: jira-search, confluence-write, pr-review, jsm-customer (also MCP prompts).
Skill: atlas_help topic=atlas and MCP resource atlas://skill.
atlas_read runs read verbs only and is marked read-only.
atlas_write runs write verbs only; they dry-run unless write_opt_in is true.
Do not use --human. Login stays atlas auth login in a terminal.
No session required for --help.
`

type helpIn struct {
	Namespace string `json:"namespace,omitempty" jsonschema:"optional CLI namespace"`
	Verb      string `json:"verb,omitempty" jsonschema:"optional verb"`
	Topic     string `json:"topic,omitempty" jsonschema:"recipe topic: jira-search, confluence-write, pr-review, jsm-customer, or atlas"`
}

type readIn struct {
	Namespace string         `json:"namespace,omitempty" jsonschema:"CLI namespace"`
	Verb      string         `json:"verb,omitempty" jsonschema:"CLI read verb"`
	Args      []string       `json:"args,omitempty" jsonschema:"positional ids after the verb"`
	Flags     map[string]any `json:"flags,omitempty" jsonschema:"CLI long flag names without dashes"`
}

type writeIn struct {
	Namespace  string         `json:"namespace,omitempty" jsonschema:"CLI namespace"`
	Verb       string         `json:"verb,omitempty" jsonschema:"CLI write verb"`
	Args       []string       `json:"args,omitempty" jsonschema:"positional ids after the verb"`
	Flags      map[string]any `json:"flags,omitempty" jsonschema:"CLI long flag names without dashes"`
	WriteOptIn bool           `json:"write_opt_in,omitempty" jsonschema:"true to perform a real workload write"`
}

func runMCP(args []string, d Deps, format string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" || hasHelp(args) {
		return writeHelp(d.Stdout, mcpHelp)
	}
	if args[0] != "serve" {
		return fail(d, domain.Usagef("unknown mcp verb %q", args[0]))
	}
	if format == "human" {
		return fail(d, domain.Usage("mcp serve is JSON-RPC on stdio; do not use --human"))
	}
	if err := ServeMCP(d); err != nil {
		return fail(d, domain.Service(err.Error()))
	}
	return domain.ExitOK
}

func ServeMCP(d Deps) error {
	return NewMCPServer(d).Run(context.Background(), &mcp.StdioTransport{})
}

func NewMCPServer(d Deps) *mcp.Server {
	yes, no := true, false
	s := mcp.NewServer(&mcp.Implementation{Name: "atlas", Version: "1.0.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_status",
		Description: "Signed-in, session usable, per-site role and per-workspace usability. No tokens. Does not open a browser.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true},
	}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		return callCLI(d, []string{"atlas", "auth", "status"}), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_help",
		Description: "CLI help for a namespace or verb, or recipe topic jira-search, confluence-write, pr-review, jsm-customer, atlas. No session required.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &no},
	}, handleHelp)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_read",
		Description: "Run one read CLI namespace+verb (get, search, users, list, diff, desks, types, site list/resolve, auth status) with a flag map. Returns that command's JSON. Refuses write verbs; use atlas_write. Lookup examples: help topics jira-search, confluence-write, pr-review, jsm-customer. Skill: atlas://skill.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &yes},
	}, func(_ context.Context, _ *mcp.CallToolRequest, in readIn) (*mcp.CallToolResult, any, error) {
		args, err := buildReadArgs(in.Namespace, in.Verb, in.Args, in.Flags)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return callCLI(d, args), nil, nil
	})
	mcp.AddTool(s, &mcp.Tool{
		Name:        "atlas_write",
		Description: "Run one write CLI namespace+verb (create, edit, comment, transition, link, update, move, merge) with a flag map. Returns a dry-run preview unless write_opt_in is true. Refuses read verbs; use atlas_read. Skill: atlas://skill.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &yes, OpenWorldHint: &yes},
	}, func(_ context.Context, _ *mcp.CallToolRequest, in writeIn) (*mcp.CallToolResult, any, error) {
		args, err := buildWriteArgs(in.Namespace, in.Verb, in.Args, in.Flags, in.WriteOptIn)
		if err != nil {
			return toolErr(err), nil, nil
		}
		return callCLI(d, args), nil, nil
	})
	for _, name := range recipeNames {
		n := name
		s.AddPrompt(&mcp.Prompt{Name: n, Description: "Lookup recipe " + n}, recipePrompt(n))
	}
	s.AddResource(&mcp.Resource{
		URI:         skillURI,
		Name:        "atlas",
		Description: "How to call atlas_status, atlas_help, atlas_read, and atlas_write on one configured cloud.",
		MIMEType:    "text/markdown",
	}, readSkill)
	return s
}

func readSkill(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := ""
	if req != nil && req.Params != nil {
		uri = req.Params.URI
	}
	if uri != skillURI {
		return nil, mcp.ResourceNotFoundError(uri)
	}
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     strings.TrimSpace(skillMarkdown) + "\n",
		}},
	}, nil
}

func handleHelp(_ context.Context, _ *mcp.CallToolRequest, in helpIn) (*mcp.CallToolResult, any, error) {
	if in.Topic != "" && (in.Namespace != "" || in.Verb != "") {
		return toolErr(domain.Usage("use topic or namespace, not both")), nil, nil
	}
	if in.Topic != "" {
		body, ok := recipe(in.Topic)
		if !ok {
			return toolErr(domain.Usagef("unknown topic %q", in.Topic)), nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimSpace(body)}}}, nil, nil
	}
	if in.Namespace == "" && in.Verb == "" {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimSpace(mcpHelp)}}}, nil, nil
	}
	args := []string{"atlas"}
	if in.Namespace != "" {
		args = append(args, in.Namespace)
	}
	if in.Verb != "" {
		args = append(args, in.Verb)
	}
	args = append(args, "--help")
	return callCLI(Deps{}, args), nil, nil
}

func recipePrompt(name string) mcp.PromptHandler {
	return func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		body, _ := recipe(name)
		return &mcp.GetPromptResult{
			Messages: []*mcp.PromptMessage{{Role: "user", Content: &mcp.TextContent{Text: strings.TrimSpace(body)}}},
		}, nil
	}
}

func callCLI(d Deps, args []string) *mcp.CallToolResult {
	var out, errw bytes.Buffer
	d.Stdout, d.Stderr = &out, &errw
	code := Run(args, d)
	return toolFromCLI(redact(out.String()), redact(errw.String()), code)
}

func toolFromCLI(stdout, stderr string, code int) *mcp.CallToolResult {
	if code == domain.ExitOK {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimSpace(stdout)}}}
	}
	text := strings.TrimSpace(stderr)
	if text == "" {
		text = `{"class":"usage","message":"command failed"}`
	}
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func toolErr(err error) *mcp.CallToolResult {
	msg := err.Error()
	cls := domain.ClassOf(err)
	hint := ""
	if de, ok := err.(*domain.Error); ok {
		hint = de.Hint
	}
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(errObj{Class: cls, Message: redact(msg), Hint: hint})
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimSpace(buf.String())}}}
}
