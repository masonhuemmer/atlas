package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestConfluenceSearchCCABInfersSesamiIO(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "search", "--cql", "space = CCAB AND type = page"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var page domain.PageSearchResult
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err, out.String())
	}
	if page.Site != "sesami-io.atlassian.net" {
		t.Fatalf("site %q", page.Site)
	}
	if page.Count == 0 {
		t.Fatal(page)
	}
	found := false
	for _, it := range page.Items {
		if it.Site != "sesami-io.atlassian.net" {
			t.Fatalf("leaked %q", it.Site)
		}
		if it.Space != "CCAB" {
			t.Fatalf("space %q", it.Space)
		}
		if it.Title == "CAB-109" {
			found = true
		}
	}
	if !found {
		t.Fatalf("seed page missing %+v", page.Items)
	}
}

func TestConfluenceSearchTitleCAB109(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "search", "--cql", `space = CCAB AND type = page AND title ~ "CAB-109"`}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var page domain.PageSearchResult
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err, out.String())
	}
	if page.Count != 1 || page.Items[0].Title != "CAB-109" {
		t.Fatalf("%+v", page)
	}
}

func TestConfluenceSearchNoSpaceIsUsage(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "search", "--cql", `type = page AND title ~ "CAB-109"`}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
}

func TestConfluenceGetSeedPage(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "get", "100", "--site", "sesami-io"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var page domain.Page
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err, out.String())
	}
	if page.ID != "100" || page.Title != "CAB-109" || page.Space != "CCAB" {
		t.Fatalf("%+v", page)
	}
	if page.Site != "sesami-io.atlassian.net" {
		t.Fatalf("site %q", page.Site)
	}
	if page.ContentFormat != domain.StorageBodyFormat {
		t.Fatalf("format %q", page.ContentFormat)
	}
}

func TestConfluenceGetUnknownIsNotFound(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "get", "999", "--site", "sesami-io"}, d)
	if code != domain.ExitNotFound {
		t.Fatal(code, errw.String())
	}
}

func TestConfluenceDeleteIsUsage(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "confluence", "delete", "100"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
	s := errw.String()
	if !strings.Contains(s, "unknown confluence verb") {
		t.Fatal(s)
	}
	var e map[string]any
	if err := json.Unmarshal(errw.Bytes(), &e); err != nil {
		t.Fatal(s)
	}
	if e["class"] != "usage" {
		t.Fatalf("%v", e)
	}
}
