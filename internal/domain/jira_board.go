package domain

// Board is the ordered Jira Software column configuration for one board.
type Board struct {
	ID      int           `json:"id"`
	Site    string        `json:"site"`
	Name    string        `json:"name"`
	Type    string        `json:"type,omitempty"`
	Columns []BoardColumn `json:"columns,omitempty"`
}

type BoardColumn struct {
	Name      string   `json:"name"`
	StatusIDs []string `json:"status_ids"`
}

const (
	PlacementInColumn   = "in_column"
	PlacementUnmapped   = "unmapped_status"
	PlacementNotOnBoard = "not_on_board"
)

// BoardPlacement keeps Jira status distinct from the board column displayed by a client.
type BoardPlacement struct {
	SchemaVersion int    `json:"schema_version"`
	BoardID       int    `json:"board_id"`
	BoardName     string `json:"board_name"`
	Site          string `json:"site"`
	Key           string `json:"key"`
	Summary       string `json:"summary"`
	BrowseURL     string `json:"browse_url"`
	Assignee      string `json:"assignee,omitempty"`
	Status        string `json:"status"`
	StatusID      string `json:"status_id"`
	State         string `json:"state"`
	Column        string `json:"column,omitempty"`
	OnBoard       bool   `json:"on_board"`
}

func ResolveBoardPlacement(issue Issue, board Board, onBoard bool) BoardPlacement {
	p := BoardPlacement{
		SchemaVersion: 1, BoardID: board.ID, BoardName: board.Name,
		Site: issue.Site, Key: issue.Key, Summary: issue.Summary,
		BrowseURL: issue.BrowseURL, Assignee: issue.Assignee,
		Status: issue.Status, StatusID: issue.StatusID, OnBoard: onBoard,
		State: PlacementUnmapped,
	}
	for _, col := range board.Columns {
		for _, id := range col.StatusIDs {
			if id == issue.StatusID && id != "" {
				if onBoard {
					p.State, p.Column = PlacementInColumn, col.Name
				} else {
					p.State = PlacementNotOnBoard
				}
				return p
			}
		}
	}
	return p
}

type JiraTransition struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	ToStatus   string `json:"to_status"`
	ToStatusID string `json:"to_status_id"`
	Column     string `json:"column,omitempty"`
}

type JiraTransitions struct {
	Key   string           `json:"key"`
	Site  string           `json:"site"`
	Items []JiraTransition `json:"items"`
}
