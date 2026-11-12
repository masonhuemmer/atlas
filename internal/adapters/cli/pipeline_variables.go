package cli

import (
	"flag"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

const pipelineVariableHelp = `atlas pipeline — stored variables

Verbs: variable-list, variable-get, variable-create, variable-update, variable-delete
variable-list: --scope repository|workspace|deployment [--repo SLUG] [--environment UUID] [--page 1] [--limit 20]
variable-create: --key NAME --value-file FILE [--secured=false] [--dry-run]
variable-update: --id UUID --key NAME --value-file FILE [--secured=false] [--dry-run]
variable-get: --id UUID
variable-delete: --id UUID [--dry-run]

--scope defaults to repository. --workspace defaults to config defaults.workspace.
Repository scope requires --repo. Deployment scope also requires --environment UUID.
Workspace scope uses neither --repo nor --environment. Output always omits variable values.
Writes replace key, value, and secured status. Secured defaults to true.
Use --value-file - to read the exact value from terminal stdin (file path required through MCP).
`

var pipelineVariableKey = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func runPipelineVariable(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, pipelineVariableHelp)
	}
	verb := args[0]
	if verb != "variable-list" && verb != "variable-create" && verb != "variable-update" && verb != "variable-get" && verb != "variable-delete" {
		return fail(d, domain.Usagef("unknown pipeline variable verb %q", verb))
	}
	fsset := flag.NewFlagSet("pipeline "+verb, flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	scope := fsset.String("scope", "repository", "repository, workspace, or deployment")
	workspace := fsset.String("workspace", "", "Bitbucket workspace (config defaults.workspace)")
	repo := fsset.String("repo", "", "repository slug")
	environment := fsset.String("environment", "", "deployment environment UUID")
	var id, key, valueFile string
	var dryRun bool
	secured := true
	page, limit := 1, 20
	if verb == "variable-list" {
		fsset.IntVar(&page, "page", 1, "page number")
		fsset.IntVar(&limit, "limit", 20, "page size (1–100)")
	}
	if verb == "variable-create" || verb == "variable-update" {
		fsset.StringVar(&key, "key", "", "variable key")
		fsset.StringVar(&valueFile, "value-file", "", "value file, or - for terminal stdin")
		fsset.BoolVar(&secured, "secured", true, "store a secured variable")
	}
	if verb == "variable-create" || verb == "variable-update" || verb == "variable-delete" {
		fsset.BoolVar(&dryRun, "dry-run", false, "preview without changing variables")
	}
	if verb == "variable-update" || verb == "variable-get" || verb == "variable-delete" {
		fsset.StringVar(&id, "id", "", "variable UUID")
	}
	if err := parseMixed(fsset, args[1:]); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if fsset.NArg() != 0 || page < 1 || limit < 1 || limit > 100 {
		return fail(d, domain.Usage("use --page >= 1 and --limit between 1 and 100, with no positional arguments"))
	}
	target, err := resolveVariableTarget(*scope, *workspace, *repo, *environment)
	if err != nil {
		return fail(d, err)
	}
	if verb == "variable-update" || verb == "variable-get" || verb == "variable-delete" {
		if !pipelineUUID.MatchString(id) {
			return fail(d, domain.Usage("--id must be a variable UUID"))
		}
		id = "{" + strings.Trim(id, "{}") + "}"
	}
	if verb == "variable-create" || verb == "variable-update" {
		if !pipelineVariableKey.MatchString(key) || valueFile == "" {
			return fail(d, domain.Usage("variable writes require --key NAME and --value-file FILE"))
		}
	}
	if dryRun {
		preview := pipelineVariablePreview{PipelineVariableTarget: target, DryRun: true, Namespace: "pipeline", Verb: verb, UUID: id, Key: key}
		if verb != "variable-delete" {
			preview.Secured = &secured
		}
		return success(d, format, preview)
	}
	if d.Pipeline == nil {
		return fail(d, domain.Service("pipeline adapter not configured"))
	}
	var result any
	switch verb {
	case "variable-list":
		result, err = d.Pipeline.ListVariables(ctx(), target, page, limit)
	case "variable-get":
		result, err = d.Pipeline.GetVariable(ctx(), target, id)
	case "variable-delete":
		err = d.Pipeline.DeleteVariable(ctx(), target, id)
		result = struct {
			domain.PipelineVariableTarget
			UUID    string `json:"uuid"`
			Deleted bool   `json:"deleted"`
		}{target, id, true}
	default:
		value, readErr := pipelineVariableValue(d, valueFile)
		if readErr != nil {
			return fail(d, readErr)
		}
		in := domain.WritePipelineVariable{Key: key, Value: value, Secured: secured}
		if verb == "variable-create" {
			result, err = d.Pipeline.CreateVariable(ctx(), target, in)
		} else {
			result, err = d.Pipeline.UpdateVariable(ctx(), target, id, in)
		}
	}
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, result)
}

type pipelineVariablePreview struct {
	domain.PipelineVariableTarget
	DryRun    bool   `json:"dry_run"`
	Namespace string `json:"namespace"`
	Verb      string `json:"verb"`
	UUID      string `json:"uuid,omitempty"`
	Key       string `json:"key,omitempty"`
	Secured   *bool  `json:"secured,omitempty"`
}

func pipelineVariableValue(d Deps, path string) (string, error) {
	reader := d.Stdin
	if path != "-" {
		file, err := os.Open(path)
		if err != nil {
			return "", domain.Usage("cannot open variable value file")
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", domain.Usage("cannot read variable value")
	}
	return string(data), nil
}

func resolveVariableTarget(scope, workspace, repo, environment string) (domain.PipelineVariableTarget, error) {
	target := domain.PipelineVariableTarget{Scope: scope, Workspace: strings.TrimSpace(workspace), Repo: strings.TrimSpace(repo), Environment: environment}
	if target.Workspace == "" {
		target.Workspace = domain.DefaultWorkspace()
	}
	if !pipelinePathName(target.Workspace) {
		return target, domain.Usage("a workspace name is required")
	}
	switch scope {
	case "workspace":
		if target.Repo != "" || environment != "" {
			return target, domain.Usage("workspace variables do not accept --repo or --environment")
		}
	case "repository", "deployment":
		if !pipelinePathName(target.Repo) {
			return target, domain.Usage("--repo must be a repository name")
		}
		if scope == "deployment" {
			if !pipelineUUID.MatchString(environment) {
				return target, domain.Usage("deployment scope requires --environment UUID")
			}
			target.Environment = "{" + strings.Trim(environment, "{}") + "}"
		} else if environment != "" {
			return target, domain.Usage("--environment requires --scope deployment")
		}
	default:
		return target, domain.Usage("--scope must be repository, workspace, or deployment")
	}
	return target, nil
}

func pipelinePathName(value string) bool {
	return value != "" && value != "." && value != ".." && !strings.ContainsAny(value, "/\\?#")
}
