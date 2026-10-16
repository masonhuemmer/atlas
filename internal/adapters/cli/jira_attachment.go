package cli

import (
	"flag"
	"os"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func jiraAttachments(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira attachments", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := strings.TrimSpace(fsset.Arg(0))
	if key == "" || fsset.NArg() != 1 {
		return fail(d, domain.Usage("one issue key is required"))
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
	items, err := d.Jira.Attachments(ctx(), site.Hostname, key)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, map[string]any{"key": strings.ToUpper(key), "site": site.Hostname, "attachments": items})
}

func jiraDownload(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira download", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	id := fsset.String("attachment", "", "attachment ID from jira attachments")
	output := fsset.String("output", "", "local output path")
	dryRun := fsset.Bool("dry-run", false, "preview without writing a file")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := strings.TrimSpace(fsset.Arg(0))
	if key == "" || fsset.NArg() != 1 || strings.TrimSpace(*id) == "" || strings.TrimSpace(*output) == "" {
		return fail(d, domain.Usage("jira download requires one issue key, --attachment ID, and --output PATH"))
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
	items, err := d.Jira.Attachments(ctx(), site.Hostname, key)
	if err != nil {
		return fail(d, err)
	}
	var selected *domain.JiraAttachment
	for i := range items {
		if items[i].ID == *id {
			selected = &items[i]
			break
		}
	}
	if selected == nil {
		return fail(d, domain.NotFound("attachment not found on issue "+strings.ToUpper(key)))
	}
	result := map[string]any{"key": strings.ToUpper(key), "site": site.Hostname, "attachment": selected, "output": *output, "dry_run": *dryRun}
	if *dryRun {
		return success(d, format, result)
	}
	f, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	n, downloadErr := d.Jira.DownloadAttachment(ctx(), site.Hostname, *id, f)
	closeErr := f.Close()
	if downloadErr != nil || closeErr != nil {
		_ = os.Remove(*output)
		if downloadErr != nil {
			return fail(d, downloadErr)
		}
		return fail(d, domain.Service(closeErr.Error()))
	}
	result["bytes"] = n
	return success(d, format, result)
}
