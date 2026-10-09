package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestConfluenceCreateUnderParent(t *testing.T) {
	posted := false
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/spaces":
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": "123"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/pages/100":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "100", "spaceId": "123", "title": "Parent"})
		case r.Method == http.MethodPost && r.URL.Path == "/wiki/api/v2/pages":
			posted = true
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if payload["parentId"] != "100" || payload["spaceId"] != "123" {
				t.Fatalf("wrong placement: %v", payload)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "101", "spaceId": "123", "parentId": "100", "title": "Child"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	page, err := (Confluence{Client: c}).Create(context.Background(), "dev.example.atlassian.net", domain.CreatePage{Space: "DOCS", ParentID: "100", Title: "Child", Body: "text"}, false)
	if err != nil || !posted || page.ParentID != "100" {
		t.Fatalf("page %+v, posted %v, error %v", page, posted, err)
	}
}

func TestConfluenceCreateRejectsParentInDifferentSpace(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/spaces":
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": "123"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/pages/100":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "100", "spaceId": "999"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	_, err := (Confluence{Client: c}).Create(context.Background(), "dev.example.atlassian.net", domain.CreatePage{Space: "DOCS", ParentID: "100", Title: "Child", Body: "text"}, false)
	if domain.ExitOf(err) != domain.ExitUsage {
		t.Fatalf("error %v", err)
	}
}

func TestConfluenceMoveUsesAppendWithoutBody(t *testing.T) {
	getSource := 0
	moved := false
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/pages/101":
			getSource++
			parent := "99"
			if moved {
				parent = "100"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "101", "spaceId": "123", "parentId": parent, "title": "Child", "body": map[string]any{"storage": map[string]any{"value": "<p>keep</p>"}}})
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/pages/100":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "100", "spaceId": "123", "title": "Parent"})
		case r.Method == http.MethodPut && r.URL.Path == "/wiki/rest/api/content/101/move/append/100":
			body, err := io.ReadAll(r.Body)
			if err != nil || len(body) != 0 {
				t.Fatalf("move sent page body %q: %v", body, err)
			}
			moved = true
			_ = json.NewEncoder(w).Encode(map[string]any{"pageId": "101"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	page, err := (Confluence{Client: c}).Move(context.Background(), "dev.example.atlassian.net", "101", "100", false)
	if err != nil || !moved || getSource != 2 || page.ParentID != "100" || page.Body != "<p>keep</p>" {
		t.Fatalf("page %+v, moved %v, reads %d, error %v", page, moved, getSource, err)
	}
}

func TestConfluenceMoveRejectsDifferentSpace(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/wiki/api/v2/pages/101":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "101", "spaceId": "123"})
		case "/wiki/api/v2/pages/100":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "100", "spaceId": "999"})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	_, err := (Confluence{Client: c}).Move(context.Background(), "dev.example.atlassian.net", "101", "100", false)
	if domain.ExitOf(err) != domain.ExitUsage {
		t.Fatalf("error %v", err)
	}
}

func TestConfluenceParentDryRunsDoNotCallAPI(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("dry run sent %s %s", r.Method, r.URL.Path)
	}))
	api := Confluence{Client: c}
	created, err := api.Create(context.Background(), "dev.example.atlassian.net", domain.CreatePage{Space: "DOCS", ParentID: "100", Title: "Child", Body: "text"}, true)
	if err != nil || created.ParentID != "100" {
		t.Fatalf("create preview %+v, error %v", created, err)
	}
	moved, err := api.Move(context.Background(), "dev.example.atlassian.net", "101", "100", true)
	if err != nil || moved.ParentID != "100" {
		t.Fatalf("move preview %+v, error %v", moved, err)
	}
}
