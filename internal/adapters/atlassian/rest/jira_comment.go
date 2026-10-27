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

func (j Jira) Comments(ctx context.Context, hostname, key string, startAt, limit int) (domain.JiraCommentPage, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.JiraCommentPage{}, err
	}
	key = strings.ToUpper(strings.TrimSpace(key))
	params := url.Values{"startAt": {strconv.Itoa(startAt)}, "maxResults": {strconv.Itoa(limit)}}
	u := jiraCommentURL(j.origin(hostname), key, "") + "?" + params.Encode()
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.JiraCommentPage{}, err
	}
	if code != http.StatusOK {
		return domain.JiraCommentPage{}, MapStatus(code)
	}
	var response struct {
		Comments   []map[string]any `json:"comments"`
		Total      int              `json:"total"`
		StartAt    int              `json:"startAt"`
		MaxResults int              `json:"maxResults"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return domain.JiraCommentPage{}, domain.Service("Jira comments returned invalid JSON")
	}
	page := domain.JiraCommentPage{
		Key: key, Site: hostname, Total: response.Total, StartAt: response.StartAt,
		Limit: response.MaxResults, Comments: make([]domain.JiraComment, 0, len(response.Comments)),
	}
	for _, raw := range response.Comments {
		page.Comments = append(page.Comments, jiraCommentFromREST(raw))
	}
	page.IsLast = len(page.Comments) == 0 || page.StartAt+len(page.Comments) >= page.Total
	if !page.IsLast {
		page.NextStartAt = page.StartAt + len(page.Comments)
	}
	return page, nil
}

func (j Jira) GetComment(ctx context.Context, hostname, key, id string) (domain.JiraComment, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.JiraComment{}, err
	}
	u := jiraCommentURL(j.origin(hostname), key, id)
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.JiraComment{}, err
	}
	if code != http.StatusOK {
		return domain.JiraComment{}, jiraCommentStatus(code)
	}
	return parseJiraComment(body, id)
}

func (j Jira) EditComment(ctx context.Context, hostname, key, id, body string) (domain.JiraComment, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.JiraComment{}, err
	}
	u := jiraCommentURL(j.origin(hostname), key, id)
	code, response, err := j.doJSON(ctx, http.MethodPut, u, cred, nil, map[string]any{"body": adfFromMarkdown(body)})
	if err != nil {
		return domain.JiraComment{}, err
	}
	if code != http.StatusOK {
		return domain.JiraComment{}, jiraCommentStatus(code)
	}
	return parseJiraComment(response, id)
}

func (j Jira) DeleteComment(ctx context.Context, hostname, key, id string) error {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return err
	}
	u := jiraCommentURL(j.origin(hostname), key, id)
	code, _, err := j.doJSON(ctx, http.MethodDelete, u, cred, nil, nil)
	if err != nil {
		return err
	}
	if code != http.StatusNoContent {
		return jiraCommentStatus(code)
	}
	return nil
}

func jiraCommentURL(origin, key, id string) string {
	u := joinURL(origin, "/rest/api/3/issue/"+q(strings.ToUpper(strings.TrimSpace(key)))+"/comment")
	if id != "" {
		u += "/" + q(id)
	}
	return u
}

func jiraCommentFromREST(raw map[string]any) domain.JiraComment {
	author := asMap(raw["author"])
	return domain.JiraComment{
		ID: str(raw, "id"), Body: markdownFromADF(raw["body"]),
		Author: str(author, "displayName"), AuthorAccountID: str(author, "accountId"),
		Created: str(raw, "created"), Updated: str(raw, "updated"),
	}
}

func parseJiraComment(body []byte, id string) (domain.JiraComment, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return domain.JiraComment{}, domain.Service("Jira comment returned invalid JSON")
	}
	comment := jiraCommentFromREST(raw)
	if comment.ID != id {
		return domain.JiraComment{}, domain.Service("Jira returned a different comment ID")
	}
	return comment, nil
}

func jiraCommentStatus(code int) error {
	if code == http.StatusNotFound {
		return domain.NotFound("comment not found")
	}
	return MapStatus(code)
}
