package cli

import (
	"encoding/json"
	"flag"
	"regexp"
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
	case "comments":
		return jiraComments(args, d, format)
	case "attachments":
		return jiraAttachments(args, d, format)
	case "download":
		return jiraDownload(args, d, format)
	case "search":
		return jiraSearch(args, d, format)
	case "suggest":
		return jiraSuggest(args, d, format)
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
	fieldsFlag := fsset.String("fields", "", "comma-separated output fields (comment selects comments, custom and customfield_N select custom fields, history selects status history)")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	sel, err := jiraGetFields(*fieldsFlag)
	if err != nil {
		return fail(d, err)
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
	if !sel.any() {
		return success(d, format, iss)
	}
	fullJSON, _ := json.Marshal(iss)
	var full map[string]any
	_ = json.Unmarshal(fullJSON, &full)
	selected := map[string]any{"key": iss.Key, "site": iss.Site, "browse_url": iss.BrowseURL}
	for _, field := range sel.fields {
		if value, ok := full[field]; ok {
			selected[field] = value
		} else if field == "comments" {
			selected[field] = []string{}
		}
	}
	if sel.custom || len(sel.customIDs) > 0 || sel.history {
		extra, err := d.Jira.GetCustom(ctx(), site.Hostname, key, sel.customIDs, sel.custom, sel.history)
		if err != nil {
			return fail(d, err)
		}
		if len(extra.CustomFields) > 0 {
			selected["custom_fields"] = extra.CustomFields
		}
		if len(extra.StatusHistory) > 0 {
			selected["status_history"] = extra.StatusHistory
		}
	}
	return success(d, format, selected)
}

// jiraGetSelection is a parsed jira get --fields value.
type jiraGetSelection struct {
	fields    []string // top-level issue keys, as in the issue JSON
	custom    bool     // every non-empty custom field
	customIDs []string // customfield_N tokens
	history   bool     // status transitions from the changelog
}

func (s jiraGetSelection) any() bool {
	return len(s.fields) > 0 || s.custom || len(s.customIDs) > 0 || s.history
}

var customFieldToken = regexp.MustCompile(`^customfield_\d+$`)

func jiraGetFields(raw string) (jiraGetSelection, error) {
	var sel jiraGetSelection
	if strings.TrimSpace(raw) == "" {
		return sel, nil
	}
	allowed := map[string]bool{
		"summary": true, "description": true, "status": true, "status_id": true,
		"issuetype": true, "parent": true, "priority": true, "labels": true,
		"assignee": true, "reporter": true, "created": true, "updated": true,
		"project": true, "comments": true, "links": true,
		"key": true, // always present; accepted so callers can list it
	}
	for _, rawField := range strings.Split(raw, ",") {
		field := strings.ToLower(strings.TrimSpace(rawField))
		switch {
		case field == "comment":
			sel.fields = append(sel.fields, "comments")
		case field == "custom":
			sel.custom = true
		case field == "history":
			sel.history = true
		case customFieldToken.MatchString(field):
			sel.customIDs = append(sel.customIDs, field)
		case allowed[field]:
			sel.fields = append(sel.fields, field)
		default:
			return sel, domain.Usagef("unknown jira get field %q", rawField).
				WithHint("issue fields, comment, custom, customfield_NNNNN, or history")
		}
	}
	return sel, nil
}

func jiraSearch(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira search", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	jql := fsset.String("jql", "", "JQL (one site only)")
	limit := fsset.Int("limit", 50, "results per page (1..100)")
	pageToken := fsset.String("page-token", "", "opaque token from the previous page")
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
	if *limit < 1 || *limit > 100 {
		return fail(d, domain.Usage("search --limit must be 1..100"))
	}
	if d.Jira == nil {
		return fail(d, domain.Service("jira adapter not configured"))
	}
	page, err := d.Jira.SearchPage(ctx(), site.Hostname, *jql, *limit, *pageToken)
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, page)
}

var jiraProjectKey = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]+$`)

func jiraSuggest(args []string, d Deps, format string) int {
	if hasHelp(args) {
		return writeHelp(d.Stdout, jiraHelp)
	}
	fsset := flag.NewFlagSet("jira suggest", flag.ContinueOnError)
	fsset.SetOutput(d.Stderr)
	siteFlag := fsset.String("site", "", "site alias, hostname, or UUID")
	query := fsset.String("query", "", "issue key or summary text")
	project := fsset.String("project", "", "project key to narrow suggestions")
	if err := parseMixed(fsset, args); err != nil {
		return fail(d, domain.Usage(err.Error()))
	}
	if strings.TrimSpace(*query) == "" {
		return fail(d, domain.Usage("suggest requires --query"))
	}
	if *project != "" && !jiraProjectKey.MatchString(*project) {
		return fail(d, domain.Usage("suggest --project must be a project key"))
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
	result, err := d.Jira.Suggest(ctx(), site.Hostname, strings.TrimSpace(*query), strings.ToUpper(*project))
	if err != nil {
		return fail(d, err)
	}
	return success(d, format, result)
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
	fieldsJSON := fsset.String("fields", "", "JSON object of extra REST fields, merged into the issue")
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
	var extra map[string]any
	haveFields := strings.TrimSpace(*fieldsJSON) != ""
	if haveFields {
		var err error
		extra, err = parseFieldsJSON(*fieldsJSON, createFieldsExample)
		if err != nil {
			return fail(d, err)
		}
		if err := checkCreateFields(extra, map[string]bool{
			"description": *description != "",
			"parent":      strings.TrimSpace(*parent) != "",
			"labels":      len(labels) > 0,
			"assignee":    strings.TrimSpace(*assignee) != "",
		}); err != nil {
			return fail(d, err)
		}
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
		Fields:      extra,
	}
	if in.Assignee == "" && !hasField(extra, "assignee") {
		in.Assignee = domain.DefaultAssigneeAccountID()
	}
	iss, err := d.Jira.Create(ctx(), site.Hostname, in, *dry)
	if err != nil {
		return fail(d, err)
	}
	if *dry {
		preview := jiraDryRun{
			DryRun:    true,
			Namespace: "jira",
			Verb:      "create",
			Project:   in.Project,
			Parent:    in.Parent,
			Assignee:  in.Assignee,
			Summary:   in.Summary,
		}
		if haveFields {
			// The preview shows the payload as it will be sent. This may read
			// the site's field schema once; it never writes.
			if preview.Fields, err = d.Jira.CreateFields(ctx(), site.Hostname, in); err != nil {
				return fail(d, err)
			}
		}
		return success(d, format, preview)
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
		fields, err = parseFieldsJSON(*fieldsJSON, editFieldsExample)
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
	if len(args) > 0 {
		switch args[0] {
		case "edit":
			return jiraCommentEdit(args[1:], d, format)
		case "delete":
			return jiraCommentDelete(args[1:], d, format)
		}
	}
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
	// Fields is the merged `fields` payload of a create that used --fields.
	Fields map[string]any `json:"fields,omitempty"`
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

const (
	createFieldsExample = `atlas jira create --project KEY --type Task --summary '…' --fields '{"customfield_10538":"…"}'`
	editFieldsExample   = `atlas jira edit KEY-1 --fields '{"summary":"…"}'`
)

func parseFieldsJSON(s, example string) (map[string]any, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, domain.Usage("--fields requires a JSON object").WithHint(example)
	}
	var fields map[string]any
	if err := json.Unmarshal([]byte(s), &fields); err != nil {
		return nil, domain.Usage("fields must be a JSON object").WithHint(example)
	}
	if fields == nil {
		return nil, domain.Usage("fields must be a JSON object").WithHint(example)
	}
	return fields, nil
}

// checkCreateFields refuses --fields keys that create owns through flags.
// flagSet reports which of the shared keys also arrived as a flag.
func checkCreateFields(fields map[string]any, flagSet map[string]bool) error {
	for _, reserved := range []string{"project", "issuetype", "summary"} {
		if hasField(fields, reserved) {
			return domain.Usagef("--fields cannot set %q", reserved).WithHint("use --project, --type, and --summary")
		}
	}
	for _, shared := range []string{"description", "parent", "labels", "assignee"} {
		if flagSet[shared] && hasField(fields, shared) {
			return domain.Usagef("%s specified in both --fields and --%s", shared, shared)
		}
	}
	return nil
}

// hasField reports whether fields has name, ignoring case and surrounding space.
func hasField(fields map[string]any, name string) bool {
	for key := range fields {
		if strings.EqualFold(strings.TrimSpace(key), name) {
			return true
		}
	}
	return false
}

func refuseCustomerJira(site domain.Site) error {
	if site.Role != domain.RoleJSMCustomer {
		return nil
	}
	return domain.Usage("jira is not available on a jsm_customer site").WithHint("use atlas jsm, not atlas jira search")
}
