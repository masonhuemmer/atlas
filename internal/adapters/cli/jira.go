package cli

import (
	"encoding/json"
	"flag"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func runJira(args []string, d Deps, format string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		return writeHelp(d.Stdout, jiraHelp)
	}
	verb, args := args[0], args[1:]
	switch verb {
	case "get":
		return jiraGet(args, d, format)
	case "search":
		return jiraSearch(args, d, format)
	case "users":
		return jiraUsers(args, d, format)
	case "boards":
		return jiraBoards(args, d, format)
	case "board":
		return jiraBoard(args, d, format)
	case "transitions":
		return jiraTransitions(args, d, format)
	case "create":
		return jiraCreate(args, d, format)
	case "edit":
		return jiraEdit(args, d, format)
	case "comment":
		return jiraComment(args, d, format)
	case "transition":
		return jiraTransition(args, d, format)
	case "link":
		return jiraLink(args, d, format)
	default:
		return fail(d, domain.Usagef("unknown jira verb %q", verb))
	}
}

func jiraGet(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira get", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := fsset.Arg(0)
	if strings.TrimSpace(key) == "" {
		return fail(d, domain.Usage("issue key is required").WithHint("atlas jira get KEY-1"))
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
	iss, err := d.Jira.Get(ctx(), site.Hostname, key)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, iss)
}

func jiraSearch(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira search", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	jql := fsset.String("jql", "", "JQL (one site only)")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, JQL: *jql})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	if strings.TrimSpace(*jql) == "" {
		return fail(d, domain.Usage("search requires --jql").WithHint("atlas jira search --jql 'project = KEY'"))
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	page, err := d.Jira.Search(ctx(), site.Hostname, *jql)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, page)
}

func jiraUsers(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira users", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	query := fsset.String("query", "", "name or email to find")
	project := fsset.String("project", "", "find users assignable to new issues in this project")
	issue := fsset.String("issue", "", "find users assignable to this issue")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if strings.TrimSpace(*query) == "" {
		return fail(d, domain.Usage("users requires --query").WithHint("atlas jira users --query 'Alex' --project SDO"))
	}
	if strings.TrimSpace(*project) != "" && strings.TrimSpace(*issue) != "" {
		return fail(d, domain.Usage("users accepts --project or --issue, not both"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Project: *project, Issue: *issue})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	result, err := d.Jira.SearchUsers(ctx(), site.Hostname, *query, *project, *issue)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, result)
}

func jiraCreate(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira create", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	project := fsset.String("project", "", "project key from config")
	issuetype := fsset.String("type", "", "issuetype.name (Task, Story, Sub-task, Incident, Change)")
	parent := fsset.String("parent", "", "parent issue key when creating a sub-task")
	summary := fsset.String("summary", "", "summary")
	description := fsset.String("description", "", "markdown description")
	assignee := fsset.String("assignee", "", "assignee.accountId")
	dry := fsset.Bool("dry-run", false, "")
	var labels []string
	fsset.Func("labels", "labels (repeatable or comma-separated)", func(s string) error {
		for _, p := range strings.Split(s, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				labels = append(labels, p)
			}
		}
		return nil
	})
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if strings.TrimSpace(*project) == "" || strings.TrimSpace(*issuetype) == "" || strings.TrimSpace(*summary) == "" {
		return fail(d, domain.Usage("create requires --project, --type, and --summary").WithHint("atlas jira create --project KEY --type Task --summary '…'"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Project: *project})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	parentKey, err := resolveParent(*parent, *project, site)
	if err != nil {
		return fail(d, err)
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	in := domain.CreateIssue{
		Project:     strings.ToUpper(strings.TrimSpace(*project)),
		IssueType:   strings.TrimSpace(*issuetype),
		Parent:      parentKey,
		Summary:     strings.TrimSpace(*summary),
		Description: *description,
		Labels:      labels,
		Assignee:    strings.TrimSpace(*assignee),
	}
	if in.Assignee == "" {
		in.Assignee = domain.DefaultAssigneeAccountID()
	}
	iss, err := d.Jira.Create(ctx(), site.Hostname, in, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, jiraDryRun{
			DryRun:    true,
			Namespace: "jira",
			Verb:      "create",
			Project:   in.Project,
			Parent:    in.Parent,
			Assignee:  in.Assignee,
			Summary:   in.Summary,
		})
	}
	return success(d, format, iss)
}

func jiraEdit(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira edit", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	fieldsJSON := fsset.String("fields", "", "JSON object of REST field names")
	parent := fsset.String("parent", "", "new parent issue key for a sub-task")
	assignee := fsset.String("assignee", "", "assignee account ID")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := fsset.Arg(0)
	if strings.TrimSpace(key) == "" {
		return fail(d, domain.Usage("issue key is required").WithHint("atlas jira edit KEY-1 --fields '{...}'"))
	}
	site, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Issue: key})
	if err != nil {
		return fail(d, err)
	}
	if err := refuseCustomerJira(site); err != nil {
		return fail(d, err)
	}
	if strings.TrimSpace(*fieldsJSON) == "" && strings.TrimSpace(*parent) == "" && strings.TrimSpace(*assignee) == "" {
		return fail(d, domain.Usage("edit requires --fields, --parent, or --assignee").WithHint("atlas jira edit KEY-1 --assignee ACCOUNT_ID"))
	}
	fields := map[string]any{}
	if strings.TrimSpace(*fieldsJSON) != "" {
		fields, err = parseFieldsJSON(*fieldsJSON)
		if err != nil {
			return fail(d, err)
		}
	}
	if strings.TrimSpace(*parent) != "" {
		if _, exists := fields["parent"]; exists {
			return fail(d, domain.Usage("parent specified in both --fields and --parent"))
		}
		project, _ := domain.ProjectFromIssue(key)
		parentKey, err := resolveParent(*parent, project, site)
		if err != nil {
			return fail(d, err)
		}
		fields["parent"] = map[string]any{"key": parentKey}
	}
	if strings.TrimSpace(*assignee) != "" {
		if _, exists := fields["assignee"]; exists {
			return fail(d, domain.Usage("assignee specified in both --fields and --assignee"))
		}
		fields["assignee"] = map[string]any{"accountId": strings.TrimSpace(*assignee)}
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	iss, err := d.Jira.Edit(ctx(), site.Hostname, key, fields, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, jiraDryRun{
			DryRun:    true,
			Namespace: "jira",
			Verb:      "edit",
			Key:       strings.ToUpper(strings.TrimSpace(key)),
			Parent:    strings.ToUpper(strings.TrimSpace(*parent)),
			Assignee:  strings.TrimSpace(*assignee),
		})
	}
	return success(d, format, iss)
}

func jiraComment(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira comment", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	body := fsset.String("body", "", "markdown comment body")
	internal := fsset.Bool("internal", false, "create a JSM internal note")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := fsset.Arg(0)
	if strings.TrimSpace(key) == "" {
		return fail(d, domain.Usage("issue key is required").WithHint("atlas jira comment KEY-1 --body '…'"))
	}
	if strings.TrimSpace(*body) == "" {
		return fail(d, domain.Usage("comment requires --body").WithHint("atlas jira comment KEY-1 --body '…'"))
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
	comment := d.Jira.Comment
	if *internal {
		comment = d.Jira.CommentInternal
	}
	if err := comment(ctx(), site.Hostname, key, *body, *dry); err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, jiraDryRun{
			DryRun:    true,
			Namespace: "jira",
			Verb:      "comment",
			Key:       strings.ToUpper(strings.TrimSpace(key)),
			Body:      *body,
			Internal:  *internal,
		})
	}
	if *internal {
		return success(d, format, map[string]any{"key": strings.ToUpper(strings.TrimSpace(key)), "body": *body, "internal": true})
	}
	return success(d, format, map[string]any{"key": strings.ToUpper(strings.TrimSpace(key)), "body": *body})
}

func jiraTransition(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira transition", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	name := fsset.String("name", "", "transition name (not id)")
	id := fsset.String("id", "", "transition ID returned by atlas jira transitions")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	key := fsset.Arg(0)
	if strings.TrimSpace(key) == "" {
		return fail(d, domain.Usage("issue key is required").WithHint("atlas jira transition KEY-1 --name Done"))
	}
	if (strings.TrimSpace(*name) == "") == (strings.TrimSpace(*id) == "") {
		return fail(d, domain.Usage("transition requires exactly one of --name or --id").WithHint("atlas jira transition KEY-1 --name Done"))
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
	var iss domain.Issue
	if *id != "" {
		iss, err = d.Jira.TransitionID(ctx(), site.Hostname, key, *id, *dry)
	} else {
		iss, err = d.Jira.Transition(ctx(), site.Hostname, key, *name, *dry)
	}
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, jiraDryRun{
			DryRun:    true,
			Namespace: "jira",
			Verb:      "transition",
			Key:       strings.ToUpper(strings.TrimSpace(key)),
			Name:      strings.TrimSpace(*name),
			ID:        strings.TrimSpace(*id),
		})
	}
	return success(d, format, iss)
}

func jiraLink(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira link", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	linkType := fsset.String("type", "", "link type (default Relates)")
	dry := fsset.Bool("dry-run", false, "")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	inward := fsset.Arg(0)
	outward := fsset.Arg(1)
	if strings.TrimSpace(inward) == "" || strings.TrimSpace(outward) == "" {
		return fail(d, domain.Usage("link requires inward and outward issue keys").WithHint("atlas jira link KEY-1 OTHER-2 [--type Relates]"))
	}
	typ := strings.TrimSpace(*linkType)
	if typ == "" {
		typ = domain.DefaultLinkType
	}
	inSite, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Issue: inward})
	if err != nil {
		return fail(d, err)
	}
	outSite, err := domain.Resolve(domain.ResolveInput{Site: *siteFlag, Issue: outward})
	if err != nil {
		return fail(d, err)
	}
	if inSite.Hostname != outSite.Hostname {
		return fail(d, domain.Usage("cannot link issues across clouds").WithHint("both keys must resolve to the same site"))
	}
	if err := refuseCustomerJira(inSite); err != nil {
		return fail(d, err)
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	inward = strings.ToUpper(strings.TrimSpace(inward))
	outward = strings.ToUpper(strings.TrimSpace(outward))
	if err := d.Jira.Link(ctx(), inSite.Hostname, inward, outward, typ, *dry); err != nil {
		return fail(d, err)
	}
	if *dry {
		return success(d, format, jiraDryRun{
			DryRun:    true,
			Namespace: "jira",
			Verb:      "link",
			Inward:    inward,
			Outward:   outward,
			Type:      typ,
		})
	}
	inIss, err := d.Jira.Get(ctx(), inSite.Hostname, inward)
	if err != nil {
		return fail(d, err)
	}
	outIss, err := d.Jira.Get(ctx(), inSite.Hostname, outward)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, jiraLinkResult{Type: typ, Inward: inIss, Outward: outIss})
}

type jiraLinkResult struct {
	Type    string       `json:"type"`
	Inward  domain.Issue `json:"inward"`
	Outward domain.Issue `json:"outward"`
}

type jiraDryRun struct {
	DryRun    bool   `json:"dry_run"`
	Internal  bool   `json:"internal,omitempty"`
	Namespace string `json:"namespace"`
	Verb      string `json:"verb"`
	Project   string `json:"project,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Assignee  string `json:"assignee,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Key       string `json:"key,omitempty"`
	Name      string `json:"name,omitempty"`
	ID        string `json:"id,omitempty"`
	Body      string `json:"body,omitempty"`
	Inward    string `json:"inward,omitempty"`
	Outward   string `json:"outward,omitempty"`
	Type      string `json:"type,omitempty"`
}

func resolveParent(raw, project string, site domain.Site) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(raw))
	if key == "" {
		return "", nil
	}
	parentProject, ok := domain.ProjectFromIssue(key)
	if !ok {
		return "", domain.Usage("parent must be a Jira issue key such as STORY-1")
	}
	if project != "" && !strings.EqualFold(project, parentProject) {
		return "", domain.Usage("sub-task and parent must be in the same project")
	}
	if _, err := domain.Resolve(domain.ResolveInput{Site: site.Hostname, Issue: key}); err != nil {
		return "", domain.Usage("sub-task and parent must be on the same site")
	}
	return key, nil
}

func parseFieldsJSON(s string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, domain.Usage("edit requires --fields JSON").WithHint(`atlas jira edit KEY-1 --fields '{"summary":"…"}'`)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(s), &fields); err != nil {
		return nil, domain.Usage("fields must be a JSON object").WithHint(`atlas jira edit KEY-1 --fields '{"summary":"…"}'`)
	}
	if fields == nil {
		return nil, domain.Usage("fields must be a JSON object")
	}
	return fields, nil
}

func refuseCustomerJira(site domain.Site) error {
	if site.Role != domain.RoleJSMCustomer {
		return nil
	}
	return domain.Usage("jira is not available on a jsm_customer site").WithHint("use atlas jsm, not atlas jira search")
}
