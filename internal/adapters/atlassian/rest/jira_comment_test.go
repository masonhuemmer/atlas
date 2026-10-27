package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestJiraCommentListGetEditDelete(t *testing.T) {
	const host = "dev.example.atlassian.net"
	stored := "Original"
	deleted := false
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); !ok || user != "a@b.c" || password != "tok" {
			t.Error("missing authorization")
		}
		if r.URL.Path == "/rest/api/3/issue/ABC-1/comment" {
			if r.Method != http.MethodGet || r.URL.Query().Get("startAt") != "0" || r.URL.Query().Get("maxResults") != "1" {
				t.Errorf("unexpected list request: %s %s", r.Method, r.URL.String())
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"startAt": 0, "maxResults": 1, "total": 2, "comments": []any{
				map[string]any{"id": "123", "body": map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{map[string]any{"type": "text", "text": stored}}}}}, "author": map[string]any{"displayName": "Mason", "accountId": "account-1"}},
			}})
			return
		}
		if r.URL.Path != "/rest/api/3/issue/ABC-1/comment/123" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		if deleted {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
		case http.MethodPut:
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Error(err)
			}
			if _, hasVisibility := payload["visibility"]; hasVisibility {
				t.Error("edit unexpectedly changed visibility")
			}
			stored = markdownFromADF(payload["body"])
		case http.MethodDelete:
			deleted = true
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "123", "body": stored})
	}))
	j := Jira{Client: c}
	page, err := j.Comments(context.Background(), host, "abc-1", 0, 1)
	if err != nil || len(page.Comments) != 1 || page.Comments[0].ID != "123" || page.Comments[0].AuthorAccountID != "account-1" || page.NextStartAt != 1 || page.IsLast {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	original, err := j.GetComment(context.Background(), host, "ABC-1", "123")
	if err != nil || original.Body != "Original" {
		t.Fatalf("original=%+v err=%v", original, err)
	}
	updated, err := j.EditComment(context.Background(), host, "ABC-1", "123", "Edited **bold**")
	if err != nil || updated.Body != "Edited **bold**" {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	if err := j.DeleteComment(context.Background(), host, "ABC-1", "123"); err != nil {
		t.Fatal(err)
	}
	_, err = j.GetComment(context.Background(), host, "ABC-1", "123")
	if domain.ClassOf(err) != domain.ClassNotFound {
		t.Fatalf("deleted comment error=%v", err)
	}
}

func TestJiraCommentPermissionError(t *testing.T) {
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/comment/123") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		t.Error(r.URL.Path)
	}))
	j := Jira{Client: c}
	_, editErr := j.EditComment(context.Background(), "dev.example.atlassian.net", "ABC-1", "123", "new")
	deleteErr := j.DeleteComment(context.Background(), "dev.example.atlassian.net", "ABC-1", "123")
	if domain.ClassOf(editErr) != domain.ClassAuth || domain.ClassOf(deleteErr) != domain.ClassAuth {
		t.Fatalf("edit=%v delete=%v", editErr, deleteErr)
	}
}

func TestJiraEditCommentRefusesUnsupportedADFBeforeWrite(t *testing.T) {
	putCalls := 0
	rich := map[string]any{"type": "doc", "version": 1, "content": []any{map[string]any{
		"type": "paragraph", "content": []any{
			map[string]any{"type": "text", "text": "Hello "},
			map[string]any{"type": "mention", "attrs": map[string]any{"id": "user-1", "text": "@Mason"}},
			map[string]any{"type": "emoji", "attrs": map[string]any{"shortName": ":grinning:", "text": "😀"}},
		},
	}}}
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCalls++
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "123", "body": rich})
	}))
	j := Jira{Client: c}
	current, err := j.GetComment(context.Background(), "dev.example.atlassian.net", "ABC-1", "123")
	if err != nil || current.UnsupportedContent != "mention" {
		t.Fatalf("current=%+v err=%v", current, err)
	}
	_, err = j.EditComment(context.Background(), "dev.example.atlassian.net", "ABC-1", "123", "replacement")
	if domain.ClassOf(err) != domain.ClassUsage || putCalls != 0 {
		t.Fatalf("edit=%v PUT calls=%d", err, putCalls)
	}
}

func TestUnsupportedCommentADFMarks(t *testing.T) {
	supported := map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{
		map[string]any{"type": "text", "text": "bold", "marks": []any{map[string]any{"type": "strong"}}},
	}}}}
	if got := unsupportedCommentADF(supported); got != "" {
		t.Fatalf("supported markup flagged: %s", got)
	}
	unsupported := map[string]any{"type": "doc", "content": []any{map[string]any{"type": "paragraph", "content": []any{
		map[string]any{"type": "text", "text": "color", "marks": []any{map[string]any{"type": "textColor"}}},
	}}}}
	if got := unsupportedCommentADF(unsupported); got != "textColor mark" {
		t.Fatalf("unsupported mark: %s", got)
	}
}
