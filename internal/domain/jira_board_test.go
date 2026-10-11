package domain

import "testing"

func TestResolveBoardPlacementUsesStatusIDsAndMembership(t *testing.T) {
	issue := Issue{Key: "SDO-1", Site: "example.atlassian.net", Summary: "work", Status: "Development", StatusID: "12"}
	board := Board{ID: 42, Name: "Team", Columns: []BoardColumn{
		{Name: "Ready", StatusIDs: []string{"11"}},
		{Name: "Doing", StatusIDs: []string{"12", "13"}},
		{Name: "Done", StatusIDs: []string{"14"}},
	}}
	p := ResolveBoardPlacement(issue, board, true)
	if p.State != PlacementInColumn || p.Column != "Doing" || p.Status != "Development" || p.StatusID != "12" || p.SchemaVersion != 1 {
		t.Fatalf("unexpected placement: %+v", p)
	}
	board.Columns[0], board.Columns[1] = board.Columns[1], board.Columns[0]
	p = ResolveBoardPlacement(issue, board, true)
	if p.Column != "Doing" {
		t.Fatalf("reordering changed placement: %+v", p)
	}
	p = ResolveBoardPlacement(issue, board, false)
	if p.State != PlacementNotOnBoard || p.Column != "" {
		t.Fatalf("excluded issue displayed a column: %+v", p)
	}
	issue.StatusID = "99"
	p = ResolveBoardPlacement(issue, board, true)
	if p.State != PlacementUnmapped || p.Column != "" {
		t.Fatalf("unmapped status displayed a column: %+v", p)
	}
}
