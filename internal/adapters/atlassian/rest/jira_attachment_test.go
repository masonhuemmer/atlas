package rest

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestJiraAttachmentsAndStreamDownload(t *testing.T) {
	content := bytes.Repeat([]byte("attachment data"), 400000)
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, password, ok := r.BasicAuth(); !ok || password != "tok" {
			t.Error("missing authorization")
		}
		switch r.URL.Path {
		case "/rest/api/3/issue/ABC-1":
			if r.URL.Query().Get("fields") != "attachment" {
				t.Error(r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"fields":{"attachment":[{"id":"123","filename":"notes.txt","size":6000000,"mimeType":"text/plain","created":"2026-10-01"}]}}`))
		case "/rest/api/3/attachment/content/123":
			if r.URL.Query().Get("redirect") != "false" {
				t.Error(r.URL.RawQuery)
			}
			_, _ = w.Write(content)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	j := Jira{Client: c}
	items, err := j.Attachments(context.Background(), "dev.example.atlassian.net", "ABC-1")
	if err != nil || len(items) != 1 || items[0].ID != "123" || items[0].MIMEType != "text/plain" {
		t.Fatalf("attachments: %+v, %v", items, err)
	}
	var got bytes.Buffer
	n, err := j.DownloadAttachment(context.Background(), "dev.example.atlassian.net", "123", &got)
	if err != nil || n != int64(len(content)) || !bytes.Equal(got.Bytes(), content) {
		t.Fatalf("download bytes=%d, err=%v, size=%d", n, err, got.Len())
	}
}

func TestJiraDownloadRejectsRedirect(t *testing.T) {
	sawRedirectTarget := false
	c, _ := liveClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/target") {
			sawRedirectTarget = true
			return
		}
		http.Redirect(w, r, "/target", http.StatusSeeOther)
	}))
	var got bytes.Buffer
	_, err := (Jira{Client: c}).DownloadAttachment(context.Background(), "dev.example.atlassian.net", "123", &got)
	if domain.ClassOf(err) != domain.ClassService || sawRedirectTarget || got.Len() != 0 {
		t.Fatalf("err=%v redirected=%v bytes=%d", err, sawRedirectTarget, got.Len())
	}
}
