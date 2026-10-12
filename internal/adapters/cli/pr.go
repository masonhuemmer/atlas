package cli

import (
	"flag"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func runPR(args []string, d Deps, format string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return writeHelp(d.Stdout, prHelp)
	}
	verb, args := args[0], args[1:]
	switch verb {
	case "get":
		return prGet(args, d, format)
	case "list":
		return prList(args, d, format)
	case "create":
		return prCreate(args, d, format)
	case "edit":
		return prEdit(args, d, format)
	case "comment":
		return prComment(args, d, format)
	case "approve":
		return prApprove(args, d, format)
	case "merge":
		return prMerge(args, d, format)
	case "diff":
		return prDiff(args, d, format)
	default:
		return fail(d, domain.Usagef("unknown pr verb %q", verb).WithHint("atlas pr get|list|create|edit|comment|approve|merge|diff"))
	}
}

func prGet(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr get", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace, repo, id := prCommonFlags(fsset)
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, n, err := requirePR(*workspace, *repo, *id)
	if err != nil {
		return fail(d, err)
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	pr, err := d.PR.Get(ctx(), ws, r, n)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, pr)
}

func prList(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr list", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace := fsset.String("workspace", "", "Bitbucket workspace (config defaults.workspace)")
	repo := fsset.String("repo", "", "repository slug")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, err := requireRepo(*workspace, *repo)
	if err != nil {
		return fail(d, err)
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	page, err := d.PR.List(ctx(), ws, r)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, page)
}

func prCreate(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr create", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace := fsset.String("workspace", "", "Bitbucket workspace (config defaults.workspace)")
	repo := fsset.String("repo", "", "repository slug")
	title := fsset.String("title", "", "PR title")
	source := fsset.String("source", "", "source branch")
	target := fsset.String("target", "", "target branch (default main)")
	description := fsset.String("description", "", "markdown description")
	dry := fsset.Bool("dry-run", false, "")
	var reviewers []string
	fsset.Func("reviewers", "reviewer account ids (repeatable)", func(s string) error {
		s = strings.TrimSpace(s)
		if s != "" {
			reviewers = append(reviewers, s)
		}
		return nil
	})
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, err := requireRepo(*workspace, *repo)
	if err != nil {
		return fail(d, err)
	}
	if strings.TrimSpace(*title) == "" || strings.TrimSpace(*source) == "" {
		return fail(d, domain.Usage("create requires --title and --source").WithHint("atlas pr create --repo atlas --title '…' --source feature/branch"))
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	in := domain.CreatePullRequest{
		Title:       strings.TrimSpace(*title),
		Source:      strings.TrimSpace(*source),
		Target:      strings.TrimSpace(*target),
		Description: *description,
		Reviewers:   reviewers,
	}
	pr, err := d.PR.Create(ctx(), ws, r, in, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, prDryRun{
			DryRun:    true,
			Namespace: "pr",
			Verb:      "create",
			Repo:      r,
			Workspace: ws,
			Title:     in.Title,
			Source:    in.Source,
		})
	}
	return success(d, format, pr)
}

func prEdit(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr edit", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace, repo, id := prCommonFlags(fsset)
	title := fsset.String("title", "", "PR title")
	description := fsset.String("description", "", "markdown description (empty clears it)")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, n, err := requirePR(*workspace, *repo, *id)
	if err != nil {
		return fail(d, err)
	}
	var in domain.EditPullRequest
	fsset.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "title":
			trimmed := strings.TrimSpace(*title)
			in.Title = &trimmed
		case "description":
			in.Description = description
		}
	})
	if in.Title == nil && in.Description == nil {
		return fail(d, domain.Usage("edit requires --title or --description").WithHint("atlas pr edit --repo atlas --id 1 --description '…'"))
	}
	if in.Title != nil && *in.Title == "" {
		return fail(d, domain.Usage("title cannot be empty"))
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	pr, err := d.PR.Edit(ctx(), ws, r, n, in, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, prDryRun{
			DryRun: true, Namespace: "pr", Verb: "edit", Repo: r, Workspace: ws,
			ID: n, Title: pr.Title, Description: &pr.Description,
		})
	}
	return success(d, format, pr)
}

func prComment(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr comment", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace, repo, id := prCommonFlags(fsset)
	body := fsset.String("body", "", "markdown comment body")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, n, err := requirePR(*workspace, *repo, *id)
	if err != nil {
		return fail(d, err)
	}
	if strings.TrimSpace(*body) == "" {
		return fail(d, domain.Usage("comment requires --body").WithHint("atlas pr comment --repo atlas --id 1 --body '…'"))
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	if err := d.PR.Comment(ctx(), ws, r, n, *body, *dry); err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, prDryRun{
			DryRun:    true,
			Namespace: "pr",
			Verb:      "comment",
			Repo:      r,
			Workspace: ws,
			ID:        n,
			Body:      *body,
		})
	}
	return success(d, format, map[string]any{"id": n, "repo": r, "workspace": ws, "body": *body})
}

func prApprove(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr approve", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace, repo, id := prCommonFlags(fsset)
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, n, err := requirePR(*workspace, *repo, *id)
	if err != nil {
		return fail(d, err)
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	approval, err := d.PR.Approve(ctx(), ws, r, n, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, prDryRun{
			DryRun: true, Namespace: "pr", Verb: "approve", Repo: r, Workspace: ws, ID: n,
		})
	}
	return success(d, format, approval)
}

func prMerge(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr merge", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace, repo, id := prCommonFlags(fsset)
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, n, err := requirePR(*workspace, *repo, *id)
	if err != nil {
		return fail(d, err)
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	pr, err := d.PR.Merge(ctx(), ws, r, n, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, prDryRun{
			DryRun:    true,
			Namespace: "pr",
			Verb:      "merge",
			Repo:      r,
			Workspace: ws,
			ID:        n,
		})
	}
	return success(d, format, pr)
}

func prDiff(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, prHelp)
	}
	fsset := flag.NewFlagSet("pr diff", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	workspace, repo, id := prCommonFlags(fsset)
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	ws, r, n, err := requirePR(*workspace, *repo, *id)
	if err != nil {
		return fail(d, err)
	}
	if d.PR == nil {
		return fail(d, domain.Service("pr adapter not configured"))
	}
	diff, err := d.PR.Diff(ctx(), ws, r, n)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, diff)
}

type prDryRun struct {
	DryRun      bool    `json:"dry_run"`
	Namespace   string  `json:"namespace"`
	Verb        string  `json:"verb"`
	Repo        string  `json:"repo,omitempty"`
	Workspace   string  `json:"workspace,omitempty"`
	Title       string  `json:"title,omitempty"`
	Description *string `json:"description,omitempty"`
	Source      string  `json:"source,omitempty"`
	ID          int     `json:"id,omitempty"`
	Body        string  `json:"body,omitempty"`
}

func prCommonFlags(fsset *flag.FlagSet) (workspace, repo *string, id *int) {
	workspace = fsset.String("workspace", "", "Bitbucket workspace (config defaults.workspace)")
	repo = fsset.String("repo", "", "repository slug")
	id = fsset.Int("id", 0, "pull request id")
	return
}

func requireRepo(workspace, repo string) (string, string, error) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		workspace = domain.DefaultWorkspace()
	}
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", "", domain.Usage("repo is required").WithHint("atlas pr get --repo atlas --id 1")
	}
	return workspace, repo, nil
}

func requirePR(workspace, repo string, id int) (string, string, int, error) {
	ws, r, err := requireRepo(workspace, repo)
	if err != nil {
		return "", "", 0, err
	}
	if id <= 0 {
		return "", "", 0, domain.Usage("pr id is required").WithHint("atlas pr get --repo atlas --id 1")
	}
	return ws, r, id, nil
}
