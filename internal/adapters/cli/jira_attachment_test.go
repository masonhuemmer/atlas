package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/app/jira"
	"github.com/masonhuemmer/atlas/internal/domain"
)

type attachmentStore struct {
	jira.Store
	content string
	fail    bool
	calls   int
}

func (s *attachmentStore) Attachments(context.Context, string, string) ([]domain.JiraAttachment, error) {
	return []domain.JiraAttachment{{ID: "123", Filename: "notes.txt", Size: int64(len(s.content))}}, nil
}

func (s *attachmentStore) DownloadAttachment(_ context.Context, _, _ string, dst io.Writer) (int64, error) {
	s.calls++
	n, err := io.WriteString(dst, s.content)
	if s.fail {
		return int64(n), domain.Service("download failed")
	}
	return int64(n), err
}

func TestJiraAttachmentDownloadSafety(t *testing.T) {
	d, out, errw := testDeps()
	stub := &attachmentStore{Store: d.Jira, content: "test attachment"}
	d.Jira = stub
	path := filepath.Join(t.TempDir(), "notes.txt")
	args := []string{"atlas", "jira", "download", "SDO-1", "--attachment", "123", "--output", path}
	if code := Run(append(append([]string{}, args...), "--dry-run"), d); code != 0 {
		t.Fatal(code, errw.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) || stub.calls != 0 {
		t.Fatalf("dry run wrote file or fetched content: %v, calls %d", err, stub.calls)
	}
	out.Reset()
	if code := Run(args, d); code != 0 {
		t.Fatal(code, errw.String())
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != stub.content {
		t.Fatalf("content %q, err %v", got, err)
	}
	if code := Run(args, d); code != domain.ExitUsage {
		t.Fatalf("overwrite exit %d", code)
	}
	got, _ = os.ReadFile(path)
	if string(got) != stub.content || stub.calls != 1 {
		t.Fatalf("overwrote file or fetched again: %q, calls %d", got, stub.calls)
	}
	wrong := append(append([]string{}, args[:4]...), "--attachment", "999", "--output", filepath.Join(t.TempDir(), "wrong"))
	if code := Run(wrong, d); code != domain.ExitNotFound || stub.calls != 1 {
		t.Fatalf("unlisted attachment: exit %d calls %d", code, stub.calls)
	}
	stub.fail = true
	partial := filepath.Join(t.TempDir(), "partial")
	if code := Run([]string{"atlas", "jira", "download", "SDO-1", "--attachment", "123", "--output", partial}, d); code != domain.ExitService {
		t.Fatalf("partial exit %d", code)
	}
	if _, err := os.Stat(partial); !os.IsNotExist(err) {
		t.Fatalf("partial file remains: %v", err)
	}
	if !strings.Contains(out.String(), "dry_run") {
		t.Fatal("no preview output")
	}
}

func TestJiraAttachmentsListsFiles(t *testing.T) {
	d, out, errw := testDeps()
	d.Jira = &attachmentStore{Store: d.Jira, content: "hello"}
	if code := Run([]string{"atlas", "jira", "attachments", "SDO-1"}, d); code != 0 {
		t.Fatal(code, errw.String())
	}
	if !strings.Contains(out.String(), `"filename":"notes.txt"`) {
		t.Fatal(out.String())
	}
	if !isRead("jira", "attachments") || !isWrite("jira", "download") || isRead("jira", "download") {
		t.Fatal("incorrect MCP command classification")
	}
	if gate := applyWriteGate("jira", "download", map[string]any{"output": "file"}, false); gate["dry-run"] != true {
		t.Fatalf("download bypassed MCP dry run: %v", gate)
	}
}
