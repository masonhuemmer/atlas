package atlassian

import (
	"context"
	"hash/fnv"
	"sort"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func fakeStatusID(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "to do":
		return "1"
	case "in progress":
		return "2"
	case "done":
		return "3"
	default:
		return "9"
	}
}

func fakeBoardID(hostname, project string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(hostname + "/" + project))
	return int(h.Sum32() & 0x7fffffff)
}

func fakeBoard(hostname, project string) domain.Board {
	return domain.Board{
		ID: fakeBoardID(hostname, project), Site: hostname, Name: project + " Kanban", Type: "kanban",
		Columns: []domain.BoardColumn{
			{Name: "To Do", StatusIDs: []string{"1"}},
			{Name: "In Progress", StatusIDs: []string{"2"}},
			{Name: "Done", StatusIDs: []string{"3"}},
		},
	}
}

func (j JiraAPI) Boards(ctx context.Context, hostname, project string) ([]domain.Board, error) {
	if j.Memory == nil {
		return nil, domain.Service("jira memory not configured")
	}
	j.Memory.mu.Lock()
	defer j.Memory.mu.Unlock()
	projects := map[string]bool{}
	for _, issue := range j.Memory.issues {
		if issue.Site == hostname && (project == "" || strings.EqualFold(issue.Project, project)) {
			projects[issue.Project] = true
		}
	}
	out := []domain.Board{}
	for name := range projects {
		board := fakeBoard(hostname, name)
		board.Columns = nil
		out = append(out, board)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Name < out[k].Name })
	return out, nil
}

func (j JiraAPI) Placement(ctx context.Context, hostname, key string, boardID int) (domain.BoardPlacement, error) {
	issue, err := j.Get(ctx, hostname, key)
	if err != nil {
		return domain.BoardPlacement{}, err
	}
	boards, err := j.Boards(ctx, hostname, "")
	if err != nil {
		return domain.BoardPlacement{}, err
	}
	for _, candidate := range boards {
		if candidate.ID == boardID {
			project := strings.TrimSuffix(candidate.Name, " Kanban")
			return domain.ResolveBoardPlacement(issue, fakeBoard(hostname, project), issue.Project == project), nil
		}
	}
	return domain.BoardPlacement{}, domain.NotFound("board not found")
}

func (j JiraAPI) Transitions(ctx context.Context, hostname, key string, boardID int) (domain.JiraTransitions, error) {
	issue, err := j.Get(ctx, hostname, key)
	if err != nil {
		return domain.JiraTransitions{}, err
	}
	j.Memory.mu.Lock()
	names := append([]string(nil), j.Memory.transitions[memKey(hostname, strings.ToUpper(key))]...)
	j.Memory.mu.Unlock()
	result := domain.JiraTransitions{Key: issue.Key, Site: hostname, Items: []domain.JiraTransition{}}
	var board domain.Board
	if boardID > 0 {
		boards, err := j.Boards(ctx, hostname, "")
		if err != nil {
			return result, err
		}
		for _, b := range boards {
			if b.ID == boardID {
				board = fakeBoard(hostname, strings.TrimSuffix(b.Name, " Kanban"))
				break
			}
		}
		if board.ID == 0 {
			return result, domain.NotFound("board not found")
		}
	}
	for _, name := range names {
		to := statusForTransition(name)
		item := domain.JiraTransition{ID: name, Name: name, ToStatus: to, ToStatusID: fakeStatusID(to)}
		for _, col := range board.Columns {
			for _, id := range col.StatusIDs {
				if id == item.ToStatusID {
					item.Column = col.Name
				}
			}
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
