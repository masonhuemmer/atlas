package cli

import (
	"flag"
	"strconv"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func jiraBoards(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fs := flag.NewFlagSet("jira boards", flag.ContinueOnError)
	fs.SetOutput(d.Stderr)
	siteFlag := fs.String("site", "", "site alias, hostname, or UUID")
	project := fs.String("project", "", "limit boards to a project key")
	if err := parseMixed(fs, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if fs.NArg() != 0 {
		return fail(d, domain.Usage("boards accepts no positional arguments"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Project: *project})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	boards, err := d.Jira.Boards(ctx(), site.Hostname, *project)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, boards)
}

func jiraBoard(args []string, d Deps, format string) int {
	if len(args) == 0 || hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	if args[0] != "placement" {
		return fail(d, domain.Usagef("unknown jira board verb %q", args[0]))
	}
	fs := flag.NewFlagSet("jira board placement", flag.ContinueOnError)
	fs.SetOutput(d.Stderr)
	siteFlag := fs.String("site", "", "site alias, hostname, or UUID")
	boardFlag := fs.String("board", "", "numeric Jira Software board ID")
	if err := parseMixed(fs, args[1:]); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if fs.NArg() != 1 {
		return fail(d, domain.Usage("placement requires one issue key"))
	}
	key := strings.ToUpper(strings.TrimSpace(fs.Arg(0)))
	if _, valid := domain.ProjectFromIssue(key); !valid {
		return fail(d, domain.Usage("placement requires an issue key such as SDO-1"))
	}
	boardID, err := positiveBoardID(*boardFlag)
	if err != nil {
		return fail(d, err)
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
	placement, err := d.Jira.Placement(ctx(), site.Hostname, key, boardID)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, placement)
}

func jiraTransitions(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fs := flag.NewFlagSet("jira transitions", flag.ContinueOnError)
	fs.SetOutput(d.Stderr)
	siteFlag := fs.String("site", "", "site alias, hostname, or UUID")
	boardFlag := fs.String("board", "", "optional board ID for column previews")
	if err := parseMixed(fs, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if fs.NArg() != 1 {
		return fail(d, domain.Usage("transitions requires one issue key"))
	}
	key := strings.ToUpper(strings.TrimSpace(fs.Arg(0)))
	if _, valid := domain.ProjectFromIssue(key); !valid {
		return fail(d, domain.Usage("transitions requires an issue key such as SDO-1"))
	}
	boardID := 0
	if *boardFlag != "" {
		var err error
		boardID, err = positiveBoardID(*boardFlag)
		if err != nil {
			return fail(d, err)
		}
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
	transitions, err := d.Jira.Transitions(ctx(), site.Hostname, key, boardID)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, transitions)
}

func positiveBoardID(raw string) (int, error) {
	id, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || id <= 0 {
		return 0, domain.Usage("--board requires a positive numeric board ID")
	}
	return id, nil
}
