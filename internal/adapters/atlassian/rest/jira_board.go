package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func (j Jira) Boards(ctx context.Context, hostname, project string) ([]domain.Board, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return nil, err
	}
	boards := []domain.Board{}
	for start := 0; ; {
		params := url.Values{"startAt": {fmt.Sprint(start)}, "maxResults": {"50"}}
		if project != "" {
			params.Set("projectKeyOrId", strings.ToUpper(strings.TrimSpace(project)))
		}
		u := joinURL(j.origin(hostname), "/rest/agile/1.0/board") + "?" + params.Encode()
		code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
		if err != nil {
			return nil, err
		}
		if code != http.StatusOK {
			return nil, MapStatus(code)
		}
		page, err := boardObject(body)
		if err != nil {
			return nil, err
		}
		if _, ok := page["values"].([]any); !ok {
			return nil, domain.Service("jira board list omitted values")
		}
		values := asList(page["values"])
		for _, raw := range values {
			v := asMap(raw)
			id, _ := v["id"].(float64)
			if id > 0 {
				boards = append(boards, domain.Board{ID: int(id), Site: hostname, Name: str(v, "name"), Type: str(v, "type")})
			}
		}
		isLast, hasLast := page["isLast"].(bool)
		if isLast || len(values) == 0 || (!hasLast && len(values) < 50) {
			return boards, nil
		}
		start += len(values)
	}
}

func (j Jira) boardConfiguration(ctx context.Context, hostname string, boardID int) (domain.Board, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.Board{}, err
	}
	u := joinURL(j.origin(hostname), fmt.Sprintf("/rest/agile/1.0/board/%d/configuration", boardID))
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.Board{}, err
	}
	if code != http.StatusOK {
		return domain.Board{}, MapStatus(code)
	}
	raw, err := boardObject(body)
	if err != nil {
		return domain.Board{}, err
	}
	board := domain.Board{ID: boardID, Site: hostname, Name: str(raw, "name"), Columns: []domain.BoardColumn{}}
	for _, item := range asList(asMap(raw["columnConfig"])["columns"]) {
		col := asMap(item)
		mapped := domain.BoardColumn{Name: str(col, "name"), StatusIDs: []string{}}
		for _, status := range asList(col["statuses"]) {
			if id := str(asMap(status), "id"); id != "" {
				mapped.StatusIDs = append(mapped.StatusIDs, id)
			}
		}
		board.Columns = append(board.Columns, mapped)
	}
	if board.Name == "" || len(board.Columns) == 0 {
		return domain.Board{}, domain.Service("jira board configuration omitted its name or columns")
	}
	return board, nil
}

func (j Jira) Placement(ctx context.Context, hostname, key string, boardID int) (domain.BoardPlacement, error) {
	issue, err := j.Get(ctx, hostname, key)
	if err != nil {
		return domain.BoardPlacement{}, err
	}
	board, err := j.boardConfiguration(ctx, hostname, boardID)
	if err != nil {
		return domain.BoardPlacement{}, err
	}
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.BoardPlacement{}, err
	}
	params := url.Values{"jql": {"key = " + issue.Key}, "maxResults": {"2"}, "fields": {"key"}}
	u := joinURL(j.origin(hostname), fmt.Sprintf("/rest/software/1.0/board/%d/issue", boardID)) + "?" + params.Encode()
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.BoardPlacement{}, err
	}
	if code != http.StatusOK {
		return domain.BoardPlacement{}, MapStatus(code)
	}
	page, err := boardObject(body)
	if err != nil {
		return domain.BoardPlacement{}, err
	}
	if _, ok := page["issues"].([]any); !ok {
		return domain.BoardPlacement{}, domain.Service("jira board issue query omitted issues")
	}
	onBoard := false
	for _, item := range asList(page["issues"]) {
		if strings.EqualFold(str(asMap(item), "key"), issue.Key) {
			onBoard = true
			break
		}
	}
	return domain.ResolveBoardPlacement(issue, board, onBoard), nil
}

func (j Jira) Transitions(ctx context.Context, hostname, key string, boardID int) (domain.JiraTransitions, error) {
	cred, err := j.credForHost(hostname)
	if err != nil {
		return domain.JiraTransitions{}, err
	}
	key = strings.ToUpper(strings.TrimSpace(key))
	var board domain.Board
	if boardID > 0 {
		board, err = j.boardConfiguration(ctx, hostname, boardID)
		if err != nil {
			return domain.JiraTransitions{}, err
		}
	}
	u := joinURL(j.origin(hostname), "/rest/api/3/issue/"+q(key)+"/transitions")
	code, body, err := j.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.JiraTransitions{}, err
	}
	if code != http.StatusOK {
		return domain.JiraTransitions{}, MapStatus(code)
	}
	page, err := boardObject(body)
	if err != nil {
		return domain.JiraTransitions{}, err
	}
	if _, ok := page["transitions"].([]any); !ok {
		return domain.JiraTransitions{}, domain.Service("jira transitions response omitted transitions")
	}
	result := domain.JiraTransitions{Key: key, Site: hostname, Items: []domain.JiraTransition{}}
	for _, raw := range asList(page["transitions"]) {
		item := asMap(raw)
		to := asMap(item["to"])
		tr := domain.JiraTransition{ID: str(item, "id"), Name: str(item, "name"), ToStatus: str(to, "name"), ToStatusID: str(to, "id")}
		for _, col := range board.Columns {
			for _, id := range col.StatusIDs {
				if tr.ToStatusID != "" && id == tr.ToStatusID {
					tr.Column = col.Name
				}
			}
		}
		result.Items = append(result.Items, tr)
	}
	return result, nil
}

func boardObject(body []byte) (map[string]any, error) {
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil || result == nil {
		return nil, domain.Service("jira board API returned invalid JSON")
	}
	return result, nil
}
