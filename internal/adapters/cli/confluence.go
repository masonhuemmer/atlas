package cli

import (
	"flag"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func runConfluence(args []string, d Deps, format string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return writeHelp(d.Stdout, confluenceHelp)
	}
	verb, args := args[0], args[1:]
	switch verb {
	case "get":
		return confluenceGet(args, d, format)
	case "search":
		return confluenceSearch(args, d, format)
	case "create":
		return confluenceCreate(args, d, format)
	case "update":
		return confluenceUpdate(args, d, format)
	default:
		return fail(d, domain.Usagef("unknown confluence verb %q", verb).WithHint("atlas confluence get|search|create|update"))
	}
}

func confluenceGet(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, confluenceHelp)
	}
	fsset := flag.NewFlagSet("confluence get", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	pageID := fsset.Arg(0)
	if strings.TrimSpace(pageID) == "" {
		return fail(d, domain.Usage("page id is required").WithHint("atlas confluence get <pageId> --site ALIAS"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerConfluence(site); err != nil {
		return fail(d, err)
	}
	if d.Confluence == nil {
		return fail(d, domain.Service("confluence adapter not configured"))
	}
	page, err := d.Confluence.Get(ctx(), site.Hostname, pageID)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, page)
}

func confluenceSearch(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, confluenceHelp)
	}
	fsset := flag.NewFlagSet("confluence search", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	cql := fsset.String("cql", "", "CQL (one site only)")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if strings.TrimSpace(*cql) == "" {
		return fail(d, domain.Usage("search requires --cql").WithHint(`atlas confluence search --cql 'space = KEY AND type = page'`))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, CQL: *cql})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerConfluence(site); err != nil {
		return fail(d, err)
	}
	if d.Confluence == nil {
		return fail(d, domain.Service("confluence adapter not configured"))
	}
	page, err := d.Confluence.Search(ctx(), site.Hostname, *cql)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, page)
}

func confluenceCreate(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, confluenceHelp)
	}
	fsset := flag.NewFlagSet("confluence create", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	space := fsset.String("space", "", "space key or numeric spaceId")
	title := fsset.String("title", "", "page title")
	body := fsset.String("body", "", "page body")
	bodyFormat := fsset.String("body-format", domain.DefaultBodyFormat, "markdown (default) or storage XHTML")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if strings.TrimSpace(*space) == "" || strings.TrimSpace(*title) == "" || strings.TrimSpace(*body) == "" {
		return fail(d, domain.Usage("create requires --space, --title, and --body").WithHint("atlas confluence create --space KEY --title '…' --body '…'"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Space: *space})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerConfluence(site); err != nil {
		return fail(d, err)
	}
	if d.Confluence == nil {
		return fail(d, domain.Service("confluence adapter not configured"))
	}
	in := domain.CreatePage{
		Space:      strings.ToUpper(strings.TrimSpace(*space)),
		Title:      strings.TrimSpace(*title),
		Body:       *body,
		BodyFormat: *bodyFormat,
	}
	page, err := d.Confluence.Create(ctx(), site.Hostname, in, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, confluenceDryRun{
			DryRun:    true,
			Namespace: "confluence",
			Verb:      "create",
			Space:     in.Space,
			Title:     in.Title,
		})
	}
	return success(d, format, page)
}

func confluenceUpdate(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, confluenceHelp)
	}
	fsset := flag.NewFlagSet("confluence update", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	body := fsset.String("body", "", "page body")
	bodyFormat := fsset.String("body-format", domain.DefaultBodyFormat, "markdown (default) or storage XHTML")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	pageID := fsset.Arg(0)
	if strings.TrimSpace(pageID) == "" {
		return fail(d, domain.Usage("page id is required").WithHint("atlas confluence update <pageId> --body '…' --site ALIAS"))
	}
	if strings.TrimSpace(*body) == "" {
		return fail(d, domain.Usage("update requires --body").WithHint("atlas confluence update <pageId> --body '…'"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerConfluence(site); err != nil {
		return fail(d, err)
	}
	if d.Confluence == nil {
		return fail(d, domain.Service("confluence adapter not configured"))
	}
	page, err := d.Confluence.Update(ctx(), site.Hostname, pageID, *body, *bodyFormat, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, confluenceDryRun{
			DryRun:    true,
			Namespace: "confluence",
			Verb:      "update",
			ID:        strings.TrimSpace(pageID),
		})
	}
	return success(d, format, page)
}

type confluenceDryRun struct {
	DryRun    bool   `json:"dry_run"`
	Namespace string `json:"namespace"`
	Verb      string `json:"verb"`
	Space     string `json:"space,omitempty"`
	Title     string `json:"title,omitempty"`
	ID        string `json:"id,omitempty"`
	Body      string `json:"body,omitempty"`
}

func refuseCustomerConfluence(site domain.Site) error {
	if site.Role != domain.RoleJSMCustomer {
		return nil
	}
	return domain.Usage("confluence is not available on a jsm_customer site").WithHint("use atlas jsm for customer REST")
}
