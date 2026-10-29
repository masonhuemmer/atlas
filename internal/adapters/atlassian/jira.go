package atlassian

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian/rest"
	"github.com/masonhuemmer/atlas/internal/domain"
)

// Memory is the fake REST seed. Issues are keyed by hostname+key; pages by hostname+id;
// PRs by workspace+repo+id; customer requests by hostname+key.
type Memory struct {
	mu             sync.Mutex
	issues         map[string]domain.Issue
	comments       map[string][]domain.JiraComment
	nextCommentID  int
	users          []domain.JiraUser
	transitions    map[string][]string
	next           map[string]int
	pages          map[string]domain.Page
	spaces         map[string]string
	nextPage       int
	prs            map[string]domain.PullRequest
	prApprovals    map[string]bool
	prDiffs        map[string]string
	nextPR         map[string]int
	desks          []domain.ServiceDesk
	types          map[string][]domain.RequestType
	requests       map[string]domain.CustomerRequest
	jsmTransitions map[string][]string
	nextReq        map[string]int
	lastComment    domain.Comment
	lastCreate     map[string]any
	searchCalls    int
	customMeta     map[string]rest.FieldMeta
	custom         map[string]map[string]any
	history        map[string][]domain.StatusChange
}

// JiraAPI is the in-process adapter used by ATLAS_FAKE and tests.
type JiraAPI struct {
	Memory *Memory
}

func (j JiraAPI) Get(ctx context.Context, hostname, key string) (domain.Issue, error) {
	if j.Memory == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	return j.Memory.Get(ctx, hostname, key)
}

func (j JiraAPI) GetCustom(ctx context.Context, hostname, key string, ids []string, all, history bool) (domain.IssueCustom, error) {
	if j.Memory == nil {
		return domain.IssueCustom{}, domain.Service("jira memory not configured")
	}
	return j.Memory.GetCustom(ctx, hostname, key, ids, all, history)
}

func (j JiraAPI) Attachments(ctx context.Context, hostname, key string) ([]domain.JiraAttachment, error) {
	if _, err := j.Get(ctx, hostname, key); err != nil {
		return nil, err
	}
	return []domain.JiraAttachment{}, nil
}

func (j JiraAPI) DownloadAttachment(_ context.Context, _, _ string, _ io.Writer) (int64, error) {
	return 0, domain.NotFound("attachment not found")
}

func (j JiraAPI) Search(ctx context.Context, hostname, jql string) (domain.SearchResult, error) {
	if j.Memory == nil {
		return domain.SearchResult{}, domain.Service("jira memory not configured")
	}
	return j.Memory.Search(ctx, hostname, jql)
}

func (j JiraAPI) SearchPage(ctx context.Context, hostname, jql string, limit int, pageToken string) (domain.SearchResult, error) {
	if j.Memory == nil {
		return domain.SearchResult{}, domain.Service("jira memory not configured")
	}
	return j.Memory.SearchPage(ctx, hostname, jql, limit, pageToken)
}

func (j JiraAPI) Suggest(ctx context.Context, hostname, query, project string) (domain.IssueSuggestions, error) {
	if j.Memory == nil {
		return domain.IssueSuggestions{}, domain.Service("jira memory not configured")
	}
	return j.Memory.Suggest(ctx, hostname, query, project)
}

func (j JiraAPI) SearchUsers(ctx context.Context, hostname, query, project, issue string) (domain.UserSearchResult, error) {
	if j.Memory == nil {
		return domain.UserSearchResult{}, domain.Service("jira memory not configured")
	}
	return j.Memory.SearchUsers(ctx, hostname, query, project, issue)
}

func (j JiraAPI) Create(ctx context.Context, hostname string, in domain.CreateIssue, dryRun bool) (domain.Issue, error) {
	if j.Memory == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	return j.Memory.Create(ctx, hostname, in, dryRun)
}

func (j JiraAPI) CreateFields(ctx context.Context, hostname string, in domain.CreateIssue) (map[string]any, error) {
	if j.Memory == nil {
		return nil, domain.Service("jira memory not configured")
	}
	return j.Memory.CreateFields(ctx, hostname, in)
}

func (j JiraAPI) Edit(ctx context.Context, hostname, key string, fields map[string]any, dryRun bool) (domain.Issue, error) {
	if j.Memory == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	return j.Memory.Edit(ctx, hostname, key, fields, dryRun)
}

func (j JiraAPI) Comment(ctx context.Context, hostname, key, body string, dryRun bool) error {
	if j.Memory == nil {
		return domain.Service("jira memory not configured")
	}
	return j.Memory.Comment(ctx, hostname, key, body, dryRun)
}

func (j JiraAPI) CommentInternal(ctx context.Context, hostname, key, body string, dryRun bool) error {
	if j.Memory == nil {
		return domain.Service("jira memory not configured")
	}
	return j.Memory.CommentInternal(ctx, hostname, key, body, dryRun)
}

func (j JiraAPI) Comments(ctx context.Context, hostname, key string, startAt, limit int) (domain.JiraCommentPage, error) {
	if j.Memory == nil {
		return domain.JiraCommentPage{}, domain.Service("jira memory not configured")
	}
	return j.Memory.Comments(ctx, hostname, key, startAt, limit)
}

func (j JiraAPI) GetComment(ctx context.Context, hostname, key, id string) (domain.JiraComment, error) {
	if j.Memory == nil {
		return domain.JiraComment{}, domain.Service("jira memory not configured")
	}
	return j.Memory.GetComment(ctx, hostname, key, id)
}

func (j JiraAPI) EditComment(ctx context.Context, hostname, key, id, body string) (domain.JiraComment, error) {
	if j.Memory == nil {
		return domain.JiraComment{}, domain.Service("jira memory not configured")
	}
	return j.Memory.EditComment(ctx, hostname, key, id, body)
}

func (j JiraAPI) DeleteComment(ctx context.Context, hostname, key, id string) error {
	if j.Memory == nil {
		return domain.Service("jira memory not configured")
	}
	return j.Memory.DeleteComment(ctx, hostname, key, id)
}

func (j JiraAPI) Transition(ctx context.Context, hostname, key, name string, dryRun bool) (domain.Issue, error) {
	if j.Memory == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	return j.Memory.Transition(ctx, hostname, key, name, dryRun)
}

func (j JiraAPI) TransitionID(ctx context.Context, hostname, key, id string, dryRun bool) (domain.Issue, error) {
	return j.Transition(ctx, hostname, key, id, dryRun)
}

func (j JiraAPI) Link(ctx context.Context, hostname, inward, outward, linkType string, dryRun bool) error {
	if j.Memory == nil {
		return domain.Service("jira memory not configured")
	}
	return j.Memory.Link(ctx, hostname, inward, outward, linkType, dryRun)
}

func (m *Memory) Get(_ context.Context, hostname, key string) (domain.Issue, error) {
	if m == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	key = strings.ToUpper(strings.TrimSpace(key))
	if hostname == "" || key == "" {
		return domain.Issue{}, domain.Usage("hostname and issue key are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	iss, ok := m.issues[memKey(hostname, key)]
	if !ok {
		return domain.Issue{}, domain.NotFound("issue not found").WithHint("check the key and site")
	}
	return cloneIssue(iss), nil
}

// GetCustom serves the seeded custom fields and status history. Field
// selection and text flattening are the live adapter's own code.
func (m *Memory) GetCustom(_ context.Context, hostname, key string, ids []string, all, history bool) (domain.IssueCustom, error) {
	if m == nil {
		return domain.IssueCustom{}, domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	key = strings.ToUpper(strings.TrimSpace(key))
	if hostname == "" || key == "" {
		return domain.IssueCustom{}, domain.Usage("hostname and issue key are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	lookup := memKey(hostname, key)
	if _, ok := m.issues[lookup]; !ok {
		return domain.IssueCustom{}, domain.NotFound("issue not found").WithHint("check the key and site")
	}
	var out domain.IssueCustom
	if all || len(ids) > 0 {
		out.CustomFields = rest.BuildCustomFields(m.custom[lookup], m.customMeta, ids, all)
	}
	if history {
		out.StatusHistory = append([]domain.StatusChange(nil), m.history[lookup]...)
	}
	return out, nil
}

func (m *Memory) Search(_ context.Context, hostname, jql string) (domain.SearchResult, error) {
	if m == nil {
		return domain.SearchResult{}, domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return domain.SearchResult{}, domain.Usage("hostname is required")
	}
	projects := domain.JQLProjects(jql)
	m.mu.Lock()
	m.searchCalls++
	defer m.mu.Unlock()
	items := make([]domain.Issue, 0)
	for _, iss := range m.issues {
		if iss.Site != hostname {
			continue
		}
		if len(projects) > 0 && !containsFold(projects, iss.Project) {
			continue
		}
		items = append(items, cloneIssue(iss))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
	return domain.SearchResult{JQL: jql, Site: hostname, Count: len(items), Items: items, IsLast: true}, nil
}

func (m *Memory) SearchPage(ctx context.Context, hostname, jql string, limit int, pageToken string) (domain.SearchResult, error) {
	page, err := m.Search(ctx, hostname, jql)
	if err != nil {
		return page, err
	}
	start := 0
	if pageToken != "" {
		start, err = strconv.Atoi(pageToken)
		if err != nil || start < 0 || start > len(page.Items) {
			return domain.SearchResult{}, domain.Usage("invalid search page token")
		}
	}
	if limit <= 0 || limit > 100 {
		return domain.SearchResult{}, domain.Usage("search limit must be 1..100")
	}
	total := len(page.Items)
	end := start + limit
	if end > total {
		end = total
	}
	page.Items = page.Items[start:end]
	page.Count = len(page.Items)
	page.IsLast = end == total
	if !page.IsLast {
		page.NextPageToken = strconv.Itoa(end)
	}
	return page, nil
}

func (m *Memory) Suggest(ctx context.Context, hostname, query, project string) (domain.IssueSuggestions, error) {
	page, err := m.Search(ctx, hostname, "project = "+project)
	if err != nil {
		return domain.IssueSuggestions{}, err
	}
	result := domain.IssueSuggestions{Site: hostname, Query: query, Project: project, Items: []domain.IssueSuggestion{}}
	for _, issue := range page.Items {
		if strings.Contains(strings.ToLower(issue.Key+" "+issue.Summary), strings.ToLower(query)) {
			result.Items = append(result.Items, domain.IssueSuggestion{Key: issue.Key, Summary: issue.Summary})
		}
	}
	return result, nil
}

func (m *Memory) SearchUsers(_ context.Context, hostname, query, project, issue string) (domain.UserSearchResult, error) {
	if m == nil {
		return domain.UserSearchResult{}, domain.Service("jira memory not configured")
	}
	query = strings.TrimSpace(query)
	project = strings.ToUpper(strings.TrimSpace(project))
	issue = strings.ToUpper(strings.TrimSpace(issue))
	result := domain.UserSearchResult{Site: hostname, Query: query, Project: project, Issue: issue, Users: []domain.JiraUser{}}
	m.mu.Lock()
	defer m.mu.Unlock()
	if issue != "" {
		if _, ok := m.issues[memKey(hostname, issue)]; !ok {
			return domain.UserSearchResult{}, domain.NotFound("issue not found")
		}
	}
	for _, user := range m.users {
		if user.Site != hostname {
			continue
		}
		if !strings.Contains(strings.ToLower(user.DisplayName), strings.ToLower(query)) && !strings.Contains(strings.ToLower(user.Email), strings.ToLower(query)) {
			continue
		}
		result.Users = append(result.Users, user)
	}
	result.Count = len(result.Users)
	return result, nil
}

func (m *Memory) Create(_ context.Context, hostname string, in domain.CreateIssue, dryRun bool) (domain.Issue, error) {
	if m == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	project := strings.ToUpper(strings.TrimSpace(in.Project))
	issuetype := strings.TrimSpace(in.IssueType)
	summary := strings.TrimSpace(in.Summary)
	if hostname == "" || project == "" || issuetype == "" || summary == "" {
		return domain.Issue{}, domain.Usage("project, type, and summary are required")
	}
	assignee := strings.TrimSpace(in.Assignee)
	if assignee == "" {
		assignee = domain.DefaultAssigneeAccountID()
	}
	preview := domain.Issue{
		Site:        hostname,
		Summary:     summary,
		Description: in.Description,
		Status:      "To Do",
		StatusID:    "1",
		IssueType:   issuetype,
		Parent:      in.Parent,
		Labels:      append([]string(nil), in.Labels...),
		Assignee:    assignee,
		Reporter:    domain.DefaultAssigneeAccountID(),
		Project:     project,
	}
	custom := map[string]any{}
	if len(in.Fields) > 0 {
		fields, err := rest.ConvertCustomFields(in.Fields, m.catalog)
		if err != nil {
			return domain.Issue{}, err
		}
		if err := applyFields(&preview, fields, custom); err != nil {
			return domain.Issue{}, err
		}
	}
	if dryRun {
		return preview, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.next[project]
	if n == 0 {
		n = 1
	}
	preview.Key = fmt.Sprintf("%s-%d", project, n)
	preview.BrowseURL = domain.BrowseURL(hostname, preview.Key)
	m.next[project] = n + 1
	m.putLocked(preview)
	if len(custom) > 0 {
		m.custom[memKey(hostname, preview.Key)] = custom
	}
	m.transitions[memKey(hostname, preview.Key)] = defaultTransitions(project, issuetype)
	return cloneIssue(preview), nil
}

// CreateFields returns the payload Create would send, built by the live
// adapter's own code against the seeded field schema.
func (m *Memory) CreateFields(_ context.Context, _ string, in domain.CreateIssue) (map[string]any, error) {
	if m == nil {
		return nil, domain.Service("jira memory not configured")
	}
	return rest.CreateFields(in, m.catalog)
}

// catalog is the seeded custom field schema.
func (m *Memory) catalog() (map[string]rest.FieldMeta, error) { return m.customMeta, nil }

func (m *Memory) Edit(_ context.Context, hostname, key string, fields map[string]any, dryRun bool) (domain.Issue, error) {
	if m == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	key = strings.ToUpper(strings.TrimSpace(key))
	if hostname == "" || key == "" {
		return domain.Issue{}, domain.Usage("hostname and issue key are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	iss, ok := m.issues[memKey(hostname, key)]
	if !ok {
		return domain.Issue{}, domain.NotFound("issue not found").WithHint("check the key and site")
	}
	fields, err := rest.ConvertCustomFields(fields, m.catalog)
	if err != nil {
		return domain.Issue{}, err
	}
	next := cloneIssue(iss)
	nextCustom := make(map[string]any, len(m.custom[memKey(hostname, key)]))
	for id, value := range m.custom[memKey(hostname, key)] {
		nextCustom[id] = value
	}
	if err := applyFields(&next, fields, nextCustom); err != nil {
		return domain.Issue{}, err
	}
	if dryRun {
		return next, nil
	}
	m.putLocked(next)
	m.custom[memKey(hostname, key)] = nextCustom
	return cloneIssue(next), nil
}

func (m *Memory) Comment(_ context.Context, hostname, key, body string, dryRun bool) error {
	if m == nil {
		return domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	key = strings.ToUpper(strings.TrimSpace(key))
	body = strings.TrimSpace(body)
	if hostname == "" || key == "" {
		return domain.Usage("hostname and issue key are required")
	}
	if body == "" {
		return domain.Usage("comment body is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	iss, ok := m.issues[memKey(hostname, key)]
	if !ok {
		return domain.NotFound("issue not found").WithHint("check the key and site")
	}
	if dryRun {
		return nil
	}
	iss.Comments = append(append([]string(nil), iss.Comments...), body)
	m.nextCommentID++
	if m.comments == nil {
		m.comments = map[string][]domain.JiraComment{}
	}
	m.comments[memKey(hostname, key)] = append(m.comments[memKey(hostname, key)], domain.JiraComment{ID: strconv.Itoa(m.nextCommentID), Body: body})
	m.putLocked(iss)
	return nil
}

func (m *Memory) CommentInternal(_ context.Context, hostname, key, body string, dryRun bool) error {
	if m == nil {
		return domain.Service("jira memory not configured")
	}
	if dryRun {
		return nil
	}
	hostname = strings.TrimSpace(hostname)
	key = strings.ToUpper(strings.TrimSpace(key))
	m.mu.Lock()
	defer m.mu.Unlock()
	req, ok := m.requests[memKey(hostname, key)]
	if !ok {
		return domain.NotFound("request not found")
	}
	req.Comments = append(req.Comments, domain.Comment{Body: body, Public: false})
	m.putRequestLocked(req)
	return nil
}

func (m *Memory) Transition(_ context.Context, hostname, key, name string, dryRun bool) (domain.Issue, error) {
	if m == nil {
		return domain.Issue{}, domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	key = strings.ToUpper(strings.TrimSpace(key))
	name = strings.TrimSpace(name)
	if hostname == "" || key == "" {
		return domain.Issue{}, domain.Usage("hostname and issue key are required")
	}
	if name == "" {
		return domain.Issue{}, domain.Usage("transition name is required").WithHint("atlas jira transition SDO-1 --name Done")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	iss, ok := m.issues[memKey(hostname, key)]
	if !ok {
		return domain.Issue{}, domain.NotFound("issue not found").WithHint("check the key and site")
	}
	names := m.transitions[memKey(hostname, key)]
	matched, ok := matchTransition(names, name)
	if !ok {
		return domain.Issue{}, domain.Usagef("unknown transition %q", name).WithHint(transitionHint(iss.Project, iss.IssueType, names))
	}
	next := cloneIssue(iss)
	next.Status = statusForTransition(matched)
	next.StatusID = fakeStatusID(next.Status)
	if dryRun {
		return next, nil
	}
	m.putLocked(next)
	return cloneIssue(next), nil
}

func (m *Memory) Link(_ context.Context, hostname, inward, outward, linkType string, dryRun bool) error {
	if m == nil {
		return domain.Service("jira memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	inward = strings.ToUpper(strings.TrimSpace(inward))
	outward = strings.ToUpper(strings.TrimSpace(outward))
	linkType = strings.TrimSpace(linkType)
	if linkType == "" {
		linkType = domain.DefaultLinkType
	}
	if hostname == "" || inward == "" || outward == "" {
		return domain.Usage("hostname and both issue keys are required")
	}
	if inward == outward {
		return domain.Usage("inward and outward keys must differ")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.issues[memKey(hostname, inward)]
	if !ok {
		return domain.NotFound("issue not found").WithHint("check the key and site")
	}
	b, ok := m.issues[memKey(hostname, outward)]
	if !ok {
		return domain.NotFound("issue not found").WithHint("check the key and site")
	}
	if dryRun {
		return nil
	}
	link := domain.IssueLink{Type: linkType, Inward: inward, Outward: outward}
	a.Links = append(append([]domain.IssueLink(nil), a.Links...), link)
	b.Links = append(append([]domain.IssueLink(nil), b.Links...), link)
	m.putLocked(a)
	m.putLocked(b)
	return nil
}

// IssueCount is the seeded plus persisted issue count (tests).
func (m *Memory) IssueCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.issues)
}

func (m *Memory) putLocked(iss domain.Issue) {
	if iss.BrowseURL == "" {
		iss.BrowseURL = domain.BrowseURL(iss.Site, iss.Key)
	}
	m.issues[memKey(iss.Site, iss.Key)] = iss
}

func (m *Memory) putPage(p domain.Page) {
	m.putPageLocked(p)
}

func (m *Memory) putPageLocked(p domain.Page) {
	if p.URL == "" {
		p.URL = domain.WikiPageURL(p.Site, p.Space, p.ID)
	}
	if p.ContentFormat == "" {
		p.ContentFormat = domain.DefaultBodyFormat
	}
	m.pages[memKey(p.Site, p.ID)] = p
}

// applyFields sets the fields the fake models on iss. customfield_* values
// are stored in custom, keyed by id, as Jira would return them.
func applyFields(iss *domain.Issue, fields map[string]any, custom map[string]any) error {
	if len(fields) == 0 {
		return domain.Usage("edit requires --fields JSON")
	}
	for k, v := range fields {
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "summary":
			s, err := asString(v)
			if err != nil {
				return err
			}
			iss.Summary = s
		case "description":
			s, err := asString(v)
			if err != nil {
				return err
			}
			iss.Description = s
		case "priority":
			s, err := asString(v)
			if err != nil {
				return err
			}
			iss.Priority = s
		case "labels":
			labels, err := asStringSlice(v)
			if err != nil {
				return err
			}
			iss.Labels = labels
		case "assignee":
			if account, ok := v.(map[string]any); ok {
				id, ok := account["accountId"].(string)
				if !ok || strings.TrimSpace(id) == "" {
					return domain.Usage("assignee must have an accountId")
				}
				iss.Assignee = strings.TrimSpace(id)
			} else {
				s, err := asString(v)
				if err != nil {
					return err
				}
				iss.Assignee = s
			}
		case "parent":
			parent, ok := v.(map[string]any)
			if !ok {
				return domain.Usage("parent must be an object with an issue key")
			}
			key, ok := parent["key"].(string)
			if !ok || strings.TrimSpace(key) == "" {
				return domain.Usage("parent must have an issue key")
			}
			iss.Parent = strings.ToUpper(strings.TrimSpace(key))
		case "status", "issuetype", "project", "key":
			return domain.Usagef("cannot edit %s via --fields", k)
		default:
			id := strings.ToLower(strings.TrimSpace(k))
			if !rest.IsCustomFieldID(id) {
				return domain.Usagef("unknown field %q", k)
			}
			custom[id] = v
		}
	}
	return nil
}

func asString(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case json.Number:
		return t.String(), nil
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10), nil
		}
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(t), nil
	default:
		return "", domain.Usagef("field value must be a string, got %T", v)
	}
}

func asStringSlice(v any) ([]string, error) {
	switch t := v.(type) {
	case []string:
		return append([]string(nil), t...), nil
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			s, err := asString(e)
			if err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, nil
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, nil
		}
		return []string{t}, nil
	default:
		return nil, domain.Usage("labels must be an array of strings")
	}
}

func matchTransition(names []string, want string) (string, bool) {
	for _, n := range names {
		if strings.EqualFold(n, want) {
			return n, true
		}
	}
	return "", false
}

func statusForTransition(name string) string {
	switch strings.ToLower(name) {
	case "done", "mark as done":
		return "Done"
	case "resolve":
		return "Completed"
	default:
		return name
	}
}

func defaultTransitions(_, _ string) []string {
	return []string{"Done"}
}

func transitionHint(_, _ string, names []string) string {
	if len(names) > 0 {
		return "available: " + strings.Join(names, ", ")
	}
	return "pass a transition name from the issue"
}

func cloneIssue(iss domain.Issue) domain.Issue {
	out := iss
	if iss.Labels != nil {
		out.Labels = append([]string(nil), iss.Labels...)
	}
	if iss.Comments != nil {
		out.Comments = append([]string(nil), iss.Comments...)
	}
	if iss.Links != nil {
		out.Links = append([]domain.IssueLink(nil), iss.Links...)
	}
	return out
}

func containsFold(keys []string, v string) bool {
	for _, k := range keys {
		if strings.EqualFold(k, v) {
			return true
		}
	}
	return false
}
