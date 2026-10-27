package cli

import (
	"flag"
	"regexp"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

var jiraCommentID = regexp.MustCompile(`^[0-9]+$`)

func jiraComments(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira comments", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	startAt := fsset.Int("start-at", 0, "first comment offset")
	limit := fsset.Int("limit", 50, "comments per page (1..100)")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := strings.TrimSpace(fsset.Arg(0))
	if key == "" || fsset.NArg() != 1 || *startAt < 0 || *limit < 1 || *limit > 100 {
		return fail(d, domain.Usage("jira comments requires one issue key, --start-at >= 0, and --limit 1..100"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Issue: key})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	page, err := d.Jira.Comments(ctx(), site.Hostname, key, *startAt, *limit)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, page)
}

func jiraCommentEdit(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira comment edit", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	id := fsset.String("id", "", "comment ID from jira comments")
	body := fsset.String("body", "", "replacement Markdown body")
	dry := fsset.Bool("dry-run", false, "preview without editing")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := strings.TrimSpace(fsset.Arg(0))
	if key == "" || fsset.NArg() != 1 || !jiraCommentID.MatchString(*id) || strings.TrimSpace(*body) == "" {
		return fail(d, domain.Usage("jira comment edit requires one issue key, numeric --id, and --body"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Issue: key})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	if *dry {
		current, err := d.Jira.GetComment(ctx(), site.Hostname, key, *id)
		if err != nil {
			return fail(d, err)
		}
		if current.UnsupportedContent != "" {
			return fail(d, domain.Usage("comment contains rich Jira content Atlas cannot preserve").WithHint("Edit this comment in Jira; unsupported content: "+current.UnsupportedContent))
		}
		return success(d, format, map[string]any{"dry_run": true, "key": strings.ToUpper(key), "action": "edit", "current": current, "body": *body})
	}
	updated, err := d.Jira.EditComment(ctx(), site.Hostname, key, *id, *body)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, map[string]any{"key": strings.ToUpper(key), "comment": updated})
}

func jiraCommentDelete(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira comment delete", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	id := fsset.String("id", "", "comment ID from jira comments")
	dry := fsset.Bool("dry-run", false, "preview without deleting")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := strings.TrimSpace(fsset.Arg(0))
	if key == "" || fsset.NArg() != 1 || !jiraCommentID.MatchString(*id) {
		return fail(d, domain.Usage("jira comment delete requires one issue key and numeric --id"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Issue: key})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	current, err := d.Jira.GetComment(ctx(), site.Hostname, key, *id)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, map[string]any{"dry_run": true, "key": strings.ToUpper(key), "action": "delete", "comment": current})
	}
	if err := d.Jira.DeleteComment(ctx(), site.Hostname, key, *id); err != nil {
		return fail(d, err)
	}
	return success(d, format, map[string]any{"key": strings.ToUpper(key), "id": *id, "deleted": true})
}
