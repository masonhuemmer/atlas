package cli

import (
	"flag"
	"io"
	"regexp"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

const pipelineHelp = `atlas pipeline — Bitbucket Cloud pipeline runs

Verbs: list, get, steps, logs, run, stop
list: atlas pipeline list --repo SLUG [--branch BRANCH] [--status STATUS] [--page 1] [--limit 20]
get:  atlas pipeline get --repo SLUG --id UUID
steps: atlas pipeline steps --repo SLUG --id UUID [--page 1] [--limit 20]
logs: atlas pipeline logs --repo SLUG --id UUID --step UUID [--human]
run: atlas pipeline run --repo SLUG --branch BRANCH [--custom NAME] [--dry-run]
stop: atlas pipeline stop --repo SLUG --id UUID [--dry-run]
Variables: variable-list, variable-get, variable-create, variable-update, variable-delete
Schedules: schedule-list, schedule-get, schedule-create, schedule-update, schedule-delete
Use atlas pipeline <verb> --help for variable and schedule flags.

--workspace defaults to config defaults.workspace. Uses the workspace Bitbucket credential.
Pagination: --page starts at 1; --limit is 1–100. Output includes next when another page exists.
Status filters: PARSING, PENDING, PAUSED, HALTED, BUILDING, ERROR, PASSED, FAILED, STOPPED, UNKNOWN.
Use run UUIDs from list and step UUIDs from steps. Braces are optional.
Reads require read:pipeline:bitbucket. Starting runs also requires write:pipeline:bitbucket.
Output: JSON (default) or --human.
logs --human prints the complete log as plain text.
CLI writes execute unless --dry-run is set. MCP writes require write_opt_in=true.
Stop acknowledges the request; use get to check the final state.
`

const pipelineUUIDPattern = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

var pipelineUUID = regexp.MustCompile(`^(` + pipelineUUIDPattern + `|\{` + pipelineUUIDPattern + `\})$`)

func runPipeline(args []string, d Deps, format string) int {
	if len(args) > 0 && strings.HasPrefix(args[0], "schedule-") {
		return runPipelineSchedule(args, d, format)
	}
	if len(args) > 0 && strings.HasPrefix(args[0], "variable-") {
		return runPipelineVariable(args, d, format)
	}
	if len(args) == 0 || args[0] == "help" || hasHelp(args) {
		return writeHelp(d.Stdout, pipelineHelp)
	}
	verb := args[0]
	if verb != "list" && verb != "get" && verb != "steps" && verb != "logs" && verb != "run" && verb != "stop" {
		return fail(d, domain.Usagef("unknown pipeline verb %q", args[0]))
	}
	fsset := flag.NewFlagSet("pipeline "+verb, flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace := fsset.String("workspace", "", "Bitbucket workspace (config defaults.workspace)")
	repo := fsset.String("repo", "", "repository slug")
	var branch, status, id, step, custom string
	var dryRun bool
	page, limit := 1, 20
	if verb == "list" {
		fsset.StringVar(&branch, "branch", "", "filter by branch name")
		fsset.StringVar(&status, "status", "", "filter by Bitbucket pipeline status")
	}
	if verb == "run" {
		fsset.StringVar(&branch, "branch", "", "branch to run (required)")
		fsset.StringVar(&custom, "custom", "", "custom pipeline name in bitbucket-pipelines.yml")
	}
	if verb == "run" || verb == "stop" {
		fsset.BoolVar(&dryRun, "dry-run", false, "preview without changing pipelines")
	}
	if verb == "list" || verb == "steps" {
		fsset.IntVar(&page, "page", 1, "page number")
		fsset.IntVar(&limit, "limit", 20, "page size (1–100)")
	}
	if verb != "list" && verb != "run" {
		fsset.StringVar(&id, "id", "", "pipeline UUID (with or without braces)")
	}
	if verb == "logs" {
		fsset.StringVar(&step, "step", "", "step UUID (with or without braces)")
	}
	if err := parseMixed(fsset, args[1:]); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r := strings.TrimSpace(*workspace), strings.TrimSpace(*repo)
	if ws == "" {
		ws = domain.DefaultWorkspace()
	}
	if ws == "" || r == "" {
		return fail(d, domain.Usage("pipeline commands require --repo and a workspace").WithHint("set --workspace or config defaults.workspace"))
	}
	if ws == "." || ws == ".." || r == "." || r == ".." || strings.ContainsAny(ws+r, "/\\?#") {
		return fail(d, domain.Usage("workspace and repo must be names, not paths or URLs"))
	}
	if fsset.NArg() != 0 || page < 1 || limit < 1 || limit > 100 {
		return fail(d, domain.Usage("use --page >= 1 and --limit between 1 and 100, with no positional arguments"))
	}
	if verb != "list" && verb != "run" && !pipelineUUID.MatchString(id) {
		return fail(d, domain.Usage("--id must be a pipeline UUID"))
	}
	if verb == "logs" && !pipelineUUID.MatchString(step) {
		return fail(d, domain.Usage("--step must be a step UUID"))
	}
	if id != "" {
		id = "{" + strings.Trim(id, "{}") + "}"
	}
	if step != "" {
		step = "{" + strings.Trim(step, "{}") + "}"
	}
	branch, custom = strings.TrimSpace(branch), strings.TrimSpace(custom)
	status = strings.ToUpper(strings.TrimSpace(status))
	if status != "" {
		switch status {
		case "PARSING", "PENDING", "PAUSED", "HALTED", "BUILDING", "ERROR", "PASSED", "FAILED", "STOPPED", "UNKNOWN":
		default:
			return fail(d, domain.Usage("unsupported pipeline status").WithHint("use PARSING, PENDING, PAUSED, HALTED, BUILDING, ERROR, PASSED, FAILED, STOPPED, or UNKNOWN"))
		}
	}
	if verb == "run" && branch == "" {
		return fail(d, domain.Usage("pipeline run requires --branch"))
	}
	if verb == "run" && flagWasSet(fsset, "custom") && custom == "" {
		return fail(d, domain.Usage("--custom requires a pipeline name"))
	}
	if dryRun {
		return success(d, format, pipelinePreview{DryRun: true, Namespace: "pipeline", Verb: verb, Workspace: ws, Repo: r, UUID: id, Branch: branch, Custom: custom})
	}
	if d.Pipeline == nil {
		return fail(d, domain.Service("pipeline adapter not configured"))
	}
	var result any
	var err error
	switch verb {
	case "run":
		result, err = d.Pipeline.Run(ctx(), ws, r, domain.RunPipeline{Branch: branch, Custom: custom})
	case "stop":
		result, err = d.Pipeline.Stop(ctx(), ws, r, id)
	case "get":
		result, err = d.Pipeline.Get(ctx(), ws, r, id)
	case "steps":
		result, err = d.Pipeline.Steps(ctx(), ws, r, id, page, limit)
	case "logs":
		var log domain.PipelineLog
		log, err = d.Pipeline.Logs(ctx(), ws, r, id, step)
		if err == nil && format == "human" {
			if _, err := io.WriteString(d.Stdout, log.Log); err != nil {
				return fail(d, domain.Service("could not write pipeline log"))
			}
			return domain.ExitOK
		}
		result = log
	case "list":
		result, err = d.Pipeline.List(ctx(), ws, r, domain.PipelineListOptions{Branch: branch, Status: status, Page: page, Limit: limit})
	}
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, result)
}

type pipelinePreview struct {
	DryRun    bool   `json:"dry_run"`
	Namespace string `json:"namespace"`
	Verb      string `json:"verb"`
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	UUID      string `json:"uuid,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Custom    string `json:"custom,omitempty"`
}
