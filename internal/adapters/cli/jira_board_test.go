package cli

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestJiraBoardCommandsExposeJSONContract(t *testing.T) {
	d, out, errw := testDeps()
	if code := Run([]string{"atlas", "jira", "boards", "--project", "SDO"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var boards []domain.Board
	if err := json.Unmarshal(out.Bytes(), &boards); err != nil || len(boards) == 0 {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if code := Run([]string{"atlas", "jira", "board", "placement", "SDO-1", "--board", strconv.Itoa(boards[0].ID)}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var placement domain.BoardPlacement
	if err := json.Unmarshal(out.Bytes(), &placement); err != nil || placement.SchemaVersion != 1 || placement.Key != "SDO-1" || placement.Column == "" {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if code := Run([]string{"atlas", "jira", "transitions", "SDO-1", "--board", strconv.Itoa(boards[0].ID)}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var transitions domain.JiraTransitions
	if err := json.Unmarshal(out.Bytes(), &transitions); err != nil || transitions.Key != "SDO-1" || len(transitions.Items) == 0 {
		t.Fatal(err, out.String())
	}
	out.Reset()
	if code := Run([]string{"atlas", "jira", "transition", "SDO-1", "--id", transitions.Items[0].ID, "--dry-run"}, d); code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var dry map[string]any
	if err := json.Unmarshal(out.Bytes(), &dry); err != nil || dry["id"] != transitions.Items[0].ID || dry["dry_run"] != true {
		t.Fatal(err, out.String())
	}
}

func TestMCPAllowsBoardReads(t *testing.T) {
	for _, tc := range []struct {
		verb string
		args []string
	}{
		{verb: "boards"},
		{verb: "board", args: []string{"placement", "SDO-1"}},
		{verb: "transitions", args: []string{"SDO-1"}},
	} {
		if _, err := buildReadArgs("jira", tc.verb, tc.args, map[string]any{"site": "sesamidevel"}); err != nil {
			t.Fatalf("%s: %v", tc.verb, err)
		}
	}
	if _, err := buildReadArgs("jira", "board", []string{"move", "SDO-1"}, nil); err == nil {
		t.Fatal("atlas_read accepted a non-placement board action")
	}
}
