package cli

import (
	"flag"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

const pipelineScheduleHelp = `atlas pipeline — schedules

Verbs: schedule-list, schedule-get, schedule-create, schedule-update, schedule-delete
schedule-list: --repo SLUG [--page 1] [--limit 20]
schedule-create: --repo SLUG --branch BRANCH --cron '0 0 0 * * ? *' [--custom NAME] [--enabled=false] [--dry-run]
schedule-get: --repo SLUG --id UUID
schedule-update: --repo SLUG --id UUID --enabled=true|false [--dry-run]
schedule-delete: --repo SLUG --id UUID [--dry-run]

--workspace defaults to config defaults.workspace. Cron uses seven fields, including seconds and year, in UTC.
Creation defaults to enabled. Without --custom, the default pipeline definition is selected.
The API updates only enabled status. To change branch, pipeline, or cron, create a replacement schedule.
`

func runPipelineSchedule(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, pipelineScheduleHelp)
	}
	verb := args[0]
	if verb != "schedule-list" && verb != "schedule-create" && verb != "schedule-get" && verb != "schedule-update" && verb != "schedule-delete" {
		return fail(d, domain.Usagef("unknown pipeline schedule verb %q", verb))
	}
	fsset := flag.NewFlagSet("pipeline "+verb, flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace := fsset.String("workspace", "", "Bitbucket workspace (config defaults.workspace)")
	repo := fsset.String("repo", "", "repository slug")
	var branch, custom, cron, id string
	var dryRun bool
	enabled := true
	page, limit := 1, 20
	if verb == "schedule-list" {
		fsset.IntVar(&page, "page", 1, "page number")
		fsset.IntVar(&limit, "limit", 20, "page size (1–100)")
	}
	if verb == "schedule-create" {
		fsset.StringVar(&branch, "branch", "", "branch to schedule")
		fsset.StringVar(&custom, "custom", "", "custom pipeline name (otherwise default)")
		fsset.StringVar(&cron, "cron", "", "seven-field UTC cron expression")
	}
	if verb == "schedule-create" || verb == "schedule-update" {
		fsset.BoolVar(&enabled, "enabled", true, "whether the schedule is enabled")
	}
	if verb == "schedule-get" || verb == "schedule-update" || verb == "schedule-delete" {
		fsset.StringVar(&id, "id", "", "schedule UUID")
	}
	if verb == "schedule-create" || verb == "schedule-update" || verb == "schedule-delete" {
		fsset.BoolVar(&dryRun, "dry-run", false, "preview without changing schedules")
	}
	if err := parseMixed(fsset, args[1:]); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r := strings.TrimSpace(*workspace), strings.TrimSpace(*repo)
	if ws == "" {
		ws = domain.DefaultWorkspace()
	}
	if !pipelinePathName(ws) || !pipelinePathName(r) {
		return fail(d, domain.Usage("schedule commands require a workspace and --repo SLUG"))
	}
	if fsset.NArg() != 0 || page < 1 || limit < 1 || limit > 100 {
		return fail(d, domain.Usage("use --page >= 1 and --limit between 1 and 100, with no positional arguments"))
	}
	if verb == "schedule-get" || verb == "schedule-update" || verb == "schedule-delete" {
		if !pipelineUUID.MatchString(id) {
			return fail(d, domain.Usage("--id must be a schedule UUID"))
		}
		id = "{" + strings.Trim(id, "{}") + "}"
	}
	if verb == "schedule-update" && !flagWasSet(fsset, "enabled") {
		return fail(d, domain.Usage("schedule-update requires --enabled=true or --enabled=false"))
	}
	branch, custom, cron = strings.TrimSpace(branch), strings.TrimSpace(custom), strings.TrimSpace(cron)
	if verb == "schedule-create" {
		if branch == "" || len(strings.Fields(cron)) != 7 {
			return fail(d, domain.Usage("schedule-create requires --branch and a seven-field UTC --cron"))
		}
		if flagWasSet(fsset, "custom") && custom == "" {
			return fail(d, domain.Usage("--custom requires a pipeline name"))
		}
	}
	if dryRun {
		preview := pipelineSchedulePreview{DryRun: true, Namespace: "pipeline", Verb: verb, Workspace: ws, Repo: r, UUID: id, Branch: branch, Custom: custom, Cron: cron}
		if verb != "schedule-delete" {
			preview.Enabled = &enabled
		}
		return success(d, format, preview)
	}
	if d.Pipeline == nil {
		return fail(d, domain.Service("pipeline adapter not configured"))
	}
	var result any
	var err error
	switch verb {
	case "schedule-list":
		result, err = d.Pipeline.ListSchedules(ctx(), ws, r, page, limit)
	case "schedule-create":
		result, err = d.Pipeline.CreateSchedule(ctx(), ws, r, domain.CreatePipelineSchedule{Branch: branch, Custom: custom, Cron: cron, Enabled: enabled})
	case "schedule-get":
		result, err = d.Pipeline.GetSchedule(ctx(), ws, r, id)
	case "schedule-update":
		result, err = d.Pipeline.SetScheduleEnabled(ctx(), ws, r, id, enabled)
	case "schedule-delete":
		err = d.Pipeline.DeleteSchedule(ctx(), ws, r, id)
		result = struct {
			Workspace string `json:"workspace"`
			Repo      string `json:"repo"`
			UUID      string `json:"uuid"`
			Deleted   bool   `json:"deleted"`
		}{ws, r, id, true}
	}
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, result)
}

type pipelineSchedulePreview struct {
	DryRun    bool   `json:"dry_run"`
	Namespace string `json:"namespace"`
	Verb      string `json:"verb"`
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	UUID      string `json:"uuid,omitempty"`
	Branch    string `json:"branch,omitempty"`
	Custom    string `json:"custom,omitempty"`
	Cron      string `json:"cron,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
}
