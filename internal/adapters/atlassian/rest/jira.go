package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

const jiraFields = "summary,description,status,issuetype,parent,priority,labels,assignee,reporter,created,updated,project,comment,issuelinks"

var jiraFieldList = []string{
	"summary", "description", "status", "issuetype", "parent", "priority", "labels",
	"assignee", "reporter", "created", "updated", "project", "comment", "issuelinks",
}

// Jira is the live Cloud REST client (https://<hostname>/rest/api/3).
type Jira struct {
	*Client
}

func (j Jira) Get(ctx context.Context, hostname, key string) (domain.Issue, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.Issue{}, err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(strings.ToUpper(strings.TrimSpace(key)))+"?fields="+jiraFields)
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.Issue{}, err
	}
	if code != http.StatusOK {
		return domain.Issue{}, MapStatus(code)
	}
	return issueFromREST(hostname, mustJSON(body)), nil
}

func (j Jira) Search(ctx context.Context, hostname, jql string) (domain.SearchResult, error) {
	return j.SearchPage(ctx, hostname, jql, 50, "")
}

func (j Jira) SearchPage(ctx context.Context, hostname, jql string, limit int, pageToken string) (domain.SearchResult, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.SearchResult{}, err
	}
	payload := map[string]any{
		"jql":        jql,
		"maxResults": limit,
		"fields":     []string{"id", "key"},
	}
	if pageToken != "" {
		payload["nextPageToken"] = pageToken
	}
	postURL := joinURL(j.origin(hostname), "/rest/api/3/search/jql")
	code, body, err := j.doJSON(ctx, http.MethodPost, postURL, cred, nil, payload)
	if err != nil {
		return domain.SearchResult{}, err
	}
	parsed := mustJSON(body)
	if err := jiraSearchAPIError(code, parsed); err != nil {
		return domain.SearchResult{}, err
	}
	items := issuesFromSearch(hostname, parsed)
	if code != http.StatusOK || len(items) == 0 {
		vals := url.Values{}
		vals.Set("jql", jql)
		vals.Set("maxResults", strconv.Itoa(limit))
		vals.Set("fields", "id,key")
		if pageToken != "" {
			vals.Set("nextPageToken", pageToken)
		}
		getURL := joinURL(j.origin(hostname), "/rest/api/3/search/jql") + "?" + vals.Encode()
		code, body, err = j.doJSON(ctx, http.MethodGet, getURL, cred, nil, nil)
		if err != nil {
			return domain.SearchResult{}, err
		}
		parsed = mustJSON(body)
		if err := jiraSearchAPIError(code, parsed); err != nil {
			return domain.SearchResult{}, err
		}
		items = issuesFromSearch(hostname, parsed)
	}
	items = j.hydrateSearch(ctx, hostname, items)
	next := str(parsed, "nextPageToken")
	isLast, ok := parsed["isLast"].(bool)
	if !ok {
		isLast = next == ""
	}
	return domain.SearchResult{JQL: jql, Site: hostname, Count: len(items), Items: items, NextPageToken: next, IsLast: isLast}, nil
}

func (j Jira) Suggest(ctx context.Context, hostname, query, project string) (domain.IssueSuggestions, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.IssueSuggestions{}, err
	}
	params := url.Values{"query": {query}}
	if project != "" {
		params.Set("currentJQL", "project = "+project)
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/picker") + "?" + params.Encode()
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.IssueSuggestions{}, err
	}
	if code != http.StatusOK {
		return domain.IssueSuggestions{}, MapStatus(code)
	}
	sections, ok := mustJSON(body)["sections"].([]any)
	if !ok {
		return domain.IssueSuggestions{}, domain.Service("Jira issue picker returned invalid JSON")
	}
	result := domain.IssueSuggestions{Site: hostname, Query: query, Project: project, Items: []domain.IssueSuggestion{}}
	seen := map[string]bool{}
	for _, rawSection := range sections {
		section, ok := rawSection.(map[string]any)
		if !ok {
			continue
		}
		for _, rawIssue := range asList(section["issues"]) {
			issue, ok := rawIssue.(map[string]any)
			if !ok {
				continue
			}
			key := str(issue, "key")
			if key == "" || seen[key] || (project != "" && !strings.HasPrefix(strings.ToUpper(key), project+"-")) {
				continue
			}
			seen[key] = true
			result.Items = append(result.Items, domain.IssueSuggestion{Key: key, Summary: str(issue, "summaryText")})
		}
	}
	return result, nil
}

func (j Jira) SearchUsers(ctx context.Context, hostname, query, project, issue string) (domain.UserSearchResult, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.UserSearchResult{}, err
	}
	query = strings.TrimSpace(query)
	project = strings.ToUpper(strings.TrimSpace(project))
	issue = strings.ToUpper(strings.TrimSpace(issue))
	params := url.Values{"query": {query}, "maxResults": {"50"}}
	path := "/rest/api/3/user/search"
	if project != "" || issue != "" {
		path = "/rest/api/3/user/assignable/search"
		if project != "" {
			params.Set("project", project)
		} else {
			params.Set("issueKey", issue)
		}
	}
	u := joinURL(j.origin(hostname), path) + "?" + params.Encode()
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.UserSearchResult{}, err
	}
	if code != http.StatusOK {
		return domain.UserSearchResult{}, MapStatus(code)
	}
	var raw []map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.UserSearchResult{}, domain.Service("jira user search returned invalid JSON")
	}
	result := domain.UserSearchResult{Site: hostname, Query: query, Project: project, Issue: issue, Users: []domain.JiraUser{}}
	for _, item := range raw {
		accountID := str(item, "accountId")
		if accountID == "" {
			continue
		}
		active, _ := item["active"].(bool)
		result.Users = append(result.Users, domain.JiraUser{
			Site: hostname, AccountID: accountID, DisplayName: str(item, "displayName"),
			Email: str(item, "emailAddress"), Active: active,
		})
	}
	result.Count = len(result.Users)
	return result, nil
}

func jiraSearchAPIError(code int, m map[string]any) error {
	msg := ""
	if msgs := asList(m["errorMessages"]); len(msgs) > 0 {
		msg, _ = msgs[0].(string)
	}
	if code != http.StatusOK && code != 0 {
		err := MapStatus(code)
		if msg != "" {
			if de, ok := err.(*domain.Error); ok {
				return de.WithHint(msg)
			}
		}
		return err
	}
	if msg != "" {
		return domain.Usage(msg)
	}
	return nil
}

func issuesFromSearch(hostname string, m map[string]any) []domain.Issue {
	raw := asList(m["issues"])
	items := make([]domain.Issue, 0, len(raw))
	for _, e := range raw {
		switch t := e.(type) {
		case map[string]any:
			iss := issueFromREST(hostname, t)
			if iss.Key == "" {
				iss.Key = str(t, "key")
			}
			if iss.Key == "" {
				iss.Key = issueIDString(t["id"])
			}
			if iss.Key != "" {
				items = append(items, iss)
			}
		case string:
			if t != "" {
				items = append(items, domain.Issue{Key: t, Site: hostname, BrowseURL: domain.BrowseURL(hostname, t)})
			}
		case float64:
			id := strconv.FormatInt(int64(t), 10)
			items = append(items, domain.Issue{Key: id, Site: hostname})
		}
	}
	return items
}

func issueIDString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	default:
		return ""
	}
}

func (j Jira) hydrateSearch(ctx context.Context, hostname string, items []domain.Issue) []domain.Issue {
	need := make([]string, 0, len(items))
	for _, iss := range items {
		if iss.Summary == "" || !strings.Contains(iss.Key, "-") {
			if iss.Key != "" {
				need = append(need, iss.Key)
			}
		}
	}
	fetched := map[string]domain.Issue{}
	if len(need) > 0 {
		if bulk, err := j.bulkGet(ctx, hostname, need); err == nil {
			for _, iss := range bulk {
				fetched[iss.Key] = iss
			}
		}
	}
	out := make([]domain.Issue, 0, len(items))
	for _, iss := range items {
		if iss.Summary != "" && strings.Contains(iss.Key, "-") {
			out = append(out, iss)
			continue
		}
		if got, ok := fetched[iss.Key]; ok {
			out = append(out, got)
			continue
		}
		got, err := j.Get(ctx, hostname, iss.Key)
		if err != nil {
			if iss.Key != "" {
				out = append(out, iss)
			}
			continue
		}
		out = append(out, got)
	}
	return out
}

func (j Jira) bulkGet(ctx context.Context, hostname string, ids []string) ([]domain.Issue, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return nil, err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/bulkfetch")
	code, body, err := j.doJSON(ctx, http.MethodPost, u, cred, nil, map[string]any{
		"issueIdsOrKeys": ids,
		"fields":         jiraFieldList,
	})
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, MapStatus(code)
	}
	var items []domain.Issue
	for _, raw := range asList(mustJSON(body)["issues"]) {
		items = append(items, issueFromREST(hostname, asMap(raw)))
	}
	return items, nil
}

func (j Jira) Create(ctx context.Context, hostname string, in domain.CreateIssue, dryRun bool) (domain.Issue, error) {
	preview := domain.Issue{
		Site: hostname, Project: in.Project, IssueType: in.IssueType,
		Parent: in.Parent, Summary: in.Summary, Description: in.Description, Labels: in.Labels,
		Assignee: in.Assignee, Status: "To Do",
	}
	if dryRun {
		return preview, nil
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.Issue{}, err
	}
	fields, err := CreateFields(in, j.fieldCatalog(ctx, hostname))
	if err != nil {
		return domain.Issue{}, err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue")
	code, body, err := j.doJSON(ctx, http.MethodPost, u, cred, nil, map[string]any{"fields": fields})
	if err != nil {
		return domain.Issue{}, err
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return domain.Issue{}, jiraWriteError(code, body)
	}
	key := str(mustJSON(body), "key")
	if key == "" {
		return domain.Issue{}, domain.Service("jira create returned no key")
	}
	created, err := j.Get(ctx, hostname, key)
	if err != nil {
		return domain.Issue{}, err
	}
	if in.Parent != "" && !strings.EqualFold(created.Parent, in.Parent) {
		return domain.Issue{}, domain.Service("jira created issue without the requested parent")
	}
	return created, nil
}

// CreateFields returns the `fields` object Create would send, without
// sending it. It reads GET /rest/api/3/field only when a customfield_* string
// needs its schema, and never writes.
func (j Jira) CreateFields(ctx context.Context, hostname string, in domain.CreateIssue) (map[string]any, error) {
	return CreateFields(in, j.fieldCatalog(ctx, hostname))
}

func (j Jira) Edit(ctx context.Context, hostname, key string, fields map[string]any, dryRun bool) (domain.Issue, error) {
	if dryRun {
		iss, err := j.Get(ctx, hostname, key)
		if err != nil {
			return domain.Issue{}, err
		}
		return iss, nil
	}
	fields, err := prepareWriteFields(fields, j.fieldCatalog(ctx, hostname))
	if err != nil {
		return domain.Issue{}, err
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.Issue{}, err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(strings.ToUpper(strings.TrimSpace(key))))
	code, body, err := j.doJSON(ctx, http.MethodPut, u, cred, nil, map[string]any{"fields": fields})
	if err != nil {
		return domain.Issue{}, err
	}
	if code != http.StatusNoContent && code != http.StatusOK {
		return domain.Issue{}, jiraWriteError(code, body)
	}
	edited, err := j.Get(ctx, hostname, key)
	if err != nil {
		return domain.Issue{}, err
	}
	if parent := str(asMap(fields["parent"]), "key"); parent != "" && !strings.EqualFold(edited.Parent, parent) {
		return domain.Issue{}, domain.Service("jira did not update the sub-task parent")
	}
	return edited, nil
}

func (j Jira) Comment(ctx context.Context, hostname, key, body string, dryRun bool) error {
	if dryRun {
		return nil
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(strings.ToUpper(strings.TrimSpace(key)))+"/comment")
	code, _, err := j.doJSON(ctx, http.MethodPost, u, cred, nil, map[string]any{"body": adfFromMarkdown(body)})
	if err != nil {
		return err
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return MapStatus(code)
	}
	return nil
}

func (j Jira) CommentInternal(ctx context.Context, hostname, key, body string, dryRun bool) error {
	if dryRun {
		return nil
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return err
	}
	requestURL := joinURL(j.origin(hostname), "/rest/servicedeskapi/request/"+q(strings.ToUpper(strings.TrimSpace(key))))
	code, _, err := j.doJSON(ctx, http.MethodGet, requestURL, cred, jsmHeaders(), nil)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return MapJSMStatus(code)
	}
	code, response, err := j.doJSON(ctx, http.MethodPost, requestURL+"/comment", cred, jsmHeaders(), map[string]any{"body": body, "public": false})
	if err != nil {
		return err
	}
	if code != http.StatusCreated && code != http.StatusOK {
		return MapJSMStatus(code)
	}
	public, ok := mustJSON(response)["public"].(bool)
	if !ok {
		return domain.Service("internal comment may have been created, but visibility could not be verified; check the issue before retrying")
	}
	if public {
		return domain.Service("internal comment was created as public; review the issue immediately")
	}
	return nil
}

func (j Jira) Transition(ctx context.Context, hostname, key, name string, dryRun bool) (domain.Issue, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.Issue{}, err
	}
	key = strings.ToUpper(strings.TrimSpace(key))
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(key)+"/transitions")
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.Issue{}, err
	}
	if code != http.StatusOK {
		return domain.Issue{}, MapStatus(code)
	}
	id := ""
	for _, raw := range asList(mustJSON(body)["transitions"]) {
		t := asMap(raw)
		if strings.EqualFold(str(t, "name"), name) {
			id = str(t, "id")
			break
		}
	}
	if id == "" {
		return domain.Issue{}, domain.Usagef("unknown transition %q", name).WithHint("atlas jira transition KEY --name NAME")
	}
	return j.applyTransitionID(ctx, hostname, key, id, dryRun)
}

func (j Jira) TransitionID(ctx context.Context, hostname, key, id string, dryRun bool) (domain.Issue, error) {
	key = strings.ToUpper(strings.TrimSpace(key))
	id = strings.TrimSpace(id)
	transitions, err := j.Transitions(ctx, hostname, key, 0)
	if err != nil {
		return domain.Issue{}, err
	}
	for _, tr := range transitions.Items {
		if tr.ID == id {
			return j.applyTransitionID(ctx, hostname, key, id, dryRun)
		}
	}
	return domain.Issue{}, domain.Usagef("unknown transition id %q", id)
}

func (j Jira) applyTransitionID(ctx context.Context, hostname, key, id string, dryRun bool) (domain.Issue, error) {
	if dryRun {
		return j.Get(ctx, hostname, key)
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.Issue{}, err
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(strings.ToUpper(strings.TrimSpace(key)))+"/transitions")
	code, _, err := j.doJSON(ctx, http.MethodPost, u, cred, nil, map[string]any{"transition": map[string]any{"id": id}})
	if err != nil {
		return domain.Issue{}, err
	}
	if code != http.StatusNoContent && code != http.StatusOK {
		return domain.Issue{}, MapStatus(code)
	}
	return j.Get(ctx, hostname, key)
}

func (j Jira) Link(ctx context.Context, hostname, inward, outward, linkType string, dryRun bool) error {
	if dryRun {
		return nil
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return err
	}
	if linkType == "" {
		linkType = domain.DefaultLinkType
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issueLink")
	payload := map[string]any{
		"type":         map[string]any{"name": linkType},
		"inwardIssue":  map[string]any{"key": strings.ToUpper(inward)},
		"outwardIssue": map[string]any{"key": strings.ToUpper(outward)},
	}
	code, _, err := j.doJSON(ctx, http.MethodPost, u, cred, nil, payload)
	if err != nil {
		return err
	}
	if code != http.StatusCreated && code != http.StatusOK && code != http.StatusNoContent {
		return MapStatus(code)
	}
	return nil
}

func issueFromREST(hostname string, m map[string]any) domain.Issue {
	key := str(m, "key")
	f := asMap(m["fields"])
	iss := domain.Issue{
		Key:         key,
		Site:        hostname,
		BrowseURL:   domain.BrowseURL(hostname, key),
		Summary:     str(f, "summary"),
		Description: markdownFromADF(f["description"]),
		Status:      nestedName(f, "status"),
		StatusID:    str(asMap(f["status"]), "id"),
		IssueType:   nestedName(f, "issuetype"),
		Parent:      str(asMap(f["parent"]), "key"),
		Priority:    nestedName(f, "priority"),
		Assignee:    displayUser(asMap(f["assignee"])),
		Reporter:    displayUser(asMap(f["reporter"])),
		Created:     str(f, "created"),
		Updated:     str(f, "updated"),
		Project:     str(asMap(f["project"]), "key"),
	}
	for _, raw := range asList(f["labels"]) {
		if s, ok := raw.(string); ok {
			iss.Labels = append(iss.Labels, s)
		}
	}
	if c := asMap(f["comment"]); c != nil {
		for _, raw := range asList(c["comments"]) {
			cm := asMap(raw)
			if t := textFromADF(cm["body"]); t != "" {
				iss.Comments = append(iss.Comments, t)
			}
		}
	}
	for _, raw := range asList(f["issuelinks"]) {
		l := asMap(raw)
		typ := nestedName(l, "type")
		if in := asMap(l["inwardIssue"]); in != nil {
			iss.Links = append(iss.Links, domain.IssueLink{Type: typ, Inward: str(in, "key"), Outward: key})
		}
		if out := asMap(l["outwardIssue"]); out != nil {
			iss.Links = append(iss.Links, domain.IssueLink{Type: typ, Inward: key, Outward: str(out, "key")})
		}
	}
	return iss
}

func displayUser(m map[string]any) string {
	if m == nil {
		return ""
	}
	if s := str(m, "displayName"); s != "" {
		return s
	}
	return str(m, "accountId")
}

// MapStatus maps Jira HTTP statuses to exit classes.
// 401/403 → auth, 404 → not_found, 429/5xx → service.
func MapStatus(code int) error {
	return classify(code, "jira", "issue not found")
}
