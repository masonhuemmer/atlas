package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestJiraBoardPlacementAndTransitionsLive(t *testing.T) {
	onBoard := true
	client := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/ABC-1":
			_, _ = w.Write([]byte(`{"key":"ABC-1","fields":{"summary":"Task","status":{"id":"12","name":"Development"},"project":{"key":"ABC"}}}`))
		case "/rest/agile/1.0/board/42/configuration":
			_, _ = w.Write([]byte(`{"id":42,"name":"Team Kanban","columnConfig":{"columns":[{"name":"Ready","statuses":[{"id":"11"}]},{"name":"Doing","statuses":[{"id":"12"},{"id":"13"}]},{"name":"Done","statuses":[{"id":"14"}]}]}}`))
		case "/rest/software/1.0/board/42/issue":
			if r.URL.Query().Get("jql") != "key = ABC-1" {
				t.Errorf("board JQL: %s", r.URL.RawQuery)
			}
			if onBoard {
				_, _ = w.Write([]byte(`{"issues":[{"key":"ABC-1"}]}`))
			} else {
				_, _ = w.Write([]byte(`{"issues":[]}`))
			}
		case "/rest/api/3/issue/ABC-1/transitions":
			_, _ = w.Write([]byte(`{"transitions":[{"id":"71","name":"Start work","to":{"id":"12","name":"Development"}},{"id":"72","name":"Finish","to":{"id":"14","name":"Closed"}}]}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	j := Jira{Client: client}
	p, err := j.Placement(context.Background(), "dev.example.atlassian.net", "ABC-1", 42)
	if err != nil || p.State != domain.PlacementInColumn || p.Column != "Doing" || p.StatusID != "12" || !p.OnBoard {
		t.Fatalf("placement %+v: %v", p, err)
	}
	onBoard = false
	p, err = j.Placement(context.Background(), "dev.example.atlassian.net", "ABC-1", 42)
	if err != nil || p.State != domain.PlacementNotOnBoard || p.Column != "" {
		t.Fatalf("excluded placement %+v: %v", p, err)
	}
	tr, err := j.Transitions(context.Background(), "dev.example.atlassian.net", "ABC-1", 42)
	if err != nil || len(tr.Items) != 2 || tr.Items[0].Column != "Doing" || tr.Items[1].Column != "Done" {
		t.Fatalf("transitions %+v: %v", tr, err)
	}
}

func TestJiraBoardListAndPermissionFailure(t *testing.T) {
	client := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/agile/1.0/board":
			if r.URL.Query().Get("projectKeyOrId") != "ABC" {
				t.Errorf("project filter: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"isLast":true,"values":[{"id":42,"name":"Team Kanban","type":"kanban"}]}`))
		case "/rest/api/3/issue/ABC-1":
			_, _ = w.Write([]byte(`{"key":"ABC-1","fields":{"status":{"id":"12","name":"Development"}}}`))
		case "/rest/agile/1.0/board/42/configuration":
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	j := Jira{Client: client}
	boards, err := j.Boards(context.Background(), "dev.example.atlassian.net", "ABC")
	if err != nil || len(boards) != 1 || boards[0].ID != 42 {
		t.Fatalf("boards %+v: %v", boards, err)
	}
	_, err = j.Placement(context.Background(), "dev.example.atlassian.net", "ABC-1", 42)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "permission") && domain.ExitOf(err) == domain.ExitOK {
		t.Fatalf("permission failure presented as placement: %v", err)
	}
}

func TestJiraBoardIssueResponseMustConfirmMembership(t *testing.T) {
	client := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/ABC-1":
			_, _ = w.Write([]byte(`{"key":"ABC-1","fields":{"status":{"id":"12","name":"Development"}}}`))
		case "/rest/agile/1.0/board/42/configuration":
			_, _ = w.Write([]byte(`{"name":"Team","columnConfig":{"columns":[{"name":"Doing","statuses":[{"id":"12"}]}]}}`))
		case "/rest/software/1.0/board/42/issue":
			_, _ = w.Write([]byte(`{"warning":"partial response"}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	_, err := (Jira{Client: client}).Placement(context.Background(), "dev.example.atlassian.net", "ABC-1", 42)
	if err == nil {
		t.Fatal("missing issues was reported as not on board")
	}
}

func TestJiraBoardsPagesUsingReturnedCount(t *testing.T) {
	client := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(`{"isLast":false,"values":[{"id":42,"name":"First"}]}`))
		case "1":
			_, _ = w.Write([]byte(`{"isLast":true,"values":[{"id":43,"name":"Second"}]}`))
		default:
			t.Errorf("unexpected page %s", r.URL.RawQuery)
		}
	}))
	boards, err := (Jira{Client: client}).Boards(context.Background(), "dev.example.atlassian.net", "ABC")
	if err != nil || len(boards) != 2 || boards[1].ID != 43 {
		t.Fatalf("boards %+v: %v", boards, err)
	}
}

func TestTransitionIDPostsTheSelectedTransition(t *testing.T) {
	posted := ""
	client := noSocketClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/3/issue/ABC-1/transitions":
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte(`{"transitions":[{"id":"71","name":"Move","to":{"id":"12","name":"Doing"}},{"id":"72","name":"Move","to":{"id":"14","name":"Done"}}]}`))
			} else {
				var body struct {
					Transition struct {
						ID string `json:"id"`
					} `json:"transition"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				posted = body.Transition.ID
				w.WriteHeader(http.StatusNoContent)
			}
		case "/rest/api/3/issue/ABC-1":
			_, _ = w.Write([]byte(`{"key":"ABC-1","fields":{"status":{"id":"14","name":"Done"}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	issue, err := (Jira{Client: client}).TransitionID(context.Background(), "dev.example.atlassian.net", "ABC-1", "72", false)
	if err != nil || posted != "72" || issue.StatusID != "14" {
		t.Fatalf("wrong transition: posted=%q issue=%+v err=%v", posted, issue, err)
	}
}
