package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestConfluenceCreateConvertsMarkdownToStorage(t *testing.T) {
	const markdown = "| Field | Value |\n| --- | --- |\n| Action | Apply |"
	posted := false
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/spaces":
			_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"id": "123"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/wiki/api/v2/pages":
			posted = true
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			body := asMap(payload["body"])
			value := str(body, "value")
			if body["representation"] != "storage" || !strings.Contains(value, "<table>") || strings.Contains(value, "| --- | --- |") {
				t.Fatalf("Confluence received unrendered Markdown: %q", value)
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "456", "spaceId": "123", "title": "Runbook", "version": map[string]any{"number": 1}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	_, err := (Confluence{Client: c}).Create(context.Background(), "dev.example.atlassian.net", domain.CreatePage{Space: "DOCS", Title: "Runbook", Body: markdown}, false)
	if err != nil || !posted {
		t.Fatalf("create error %v, posted %v", err, posted)
	}
}

func TestConfluenceUpdateConvertsMarkdownToStorage(t *testing.T) {
	const markdown = "## Steps\n\nApply the change."
	updated := false
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/wiki/api/v2/pages/456":
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "456", "spaceId": "123", "title": "Runbook", "version": map[string]any{"number": 2}, "body": map[string]any{"storage": map[string]any{"value": "<p>old</p>"}}})
		case r.Method == http.MethodPut && r.URL.Path == "/wiki/api/v2/pages/456":
			updated = true
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			body := asMap(payload["body"])
			value := str(body, "value")
			if body["representation"] != "storage" || !strings.Contains(value, "<h2>Steps</h2>") || strings.Contains(value, "## Steps") {
				t.Fatalf("Confluence received unrendered Markdown: %q", value)
			}
			if asMap(payload["version"])["number"] != float64(3) {
				t.Fatalf("wrong version: %v", payload["version"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "456", "spaceId": "123", "title": "Runbook", "version": map[string]any{"number": 3}})
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	page, err := (Confluence{Client: c}).Update(context.Background(), "dev.example.atlassian.net", "456", markdown, domain.DefaultBodyFormat, false)
	if err != nil || !updated {
		t.Fatalf("update error %v, updated %v", err, updated)
	}
	if page.ContentFormat != domain.StorageBodyFormat || !strings.Contains(page.Body, "<h2>Steps</h2>") {
		t.Fatalf("%+v", page)
	}
}
