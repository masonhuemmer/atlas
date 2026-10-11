package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestJiraGetSDO1(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "get", "SDO-1"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var iss domain.Issue
	if err := json.Unmarshal(out.Bytes(), &iss); err != nil {
		t.Fatal(err, out.String())
	}
	if iss.Key != "SDO-1" {
		t.Fatalf("key %q", iss.Key)
	}
	if iss.Site != "sesamidevel.atlassian.net" {
		t.Fatalf("site %q", iss.Site)
	}
	if iss.BrowseURL != "https://sesamidevel.atlassian.net/browse/SDO-1" {
		t.Fatalf("browse %q", iss.BrowseURL)
	}
	if iss.Project != "SDO" || iss.Summary == "" {
		t.Fatalf("%+v", iss)
	}
}

func TestJiraGetUnknownKey(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "jira", "get", "SDO-999"}, d)
	if code != domain.ExitNotFound {
		t.Fatal(code, errw.String())
	}
	var e map[string]any
	if err := json.Unmarshal(errw.Bytes(), &e); err != nil {
		t.Fatal(errw.String())
	}
	if e["class"] != "not_found" {
		t.Fatalf("%v", e)
	}
}

func TestJiraSearchCABHitsSesamiIOOnly(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "search", "--jql", "project = CAB"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var page domain.SearchResult
	if err := json.Unmarshal(out.Bytes(), &page); err != nil {
		t.Fatal(err, out.String())
	}
	if page.Site != "sesami-io.atlassian.net" {
		t.Fatalf("site %q", page.Site)
	}
	if page.Count == 0 {
		t.Fatal(page)
	}
	for _, iss := range page.Items {
		if iss.Site != "sesami-io.atlassian.net" {
			t.Fatalf("leaked %q", iss.Site)
		}
		if iss.Project != "CAB" {
			t.Fatalf("project %q", iss.Project)
		}
		if !strings.HasPrefix(iss.Key, "CAB-") {
			t.Fatalf("key %q", iss.Key)
		}
	}
}

func TestJiraSuggestReturnsPickerItems(t *testing.T) {
	d, out, errw := testDeps()
	code := Run([]string{"atlas", "jira", "suggest", "--query", "SDO", "--project", "SDO"}, d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var result domain.IssueSuggestions
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Project != "SDO" || len(result.Items) == 0 || !strings.HasPrefix(result.Items[0].Key, "SDO-") {
		t.Fatalf("%+v", result)
	}
}

func TestJiraSearchRejectsInvalidPageLimit(t *testing.T) {
	d, _, _ := testDeps()
	if code := Run([]string{"atlas", "jira", "search", "--jql", "project = SDO", "--limit", "101"}, d); code != domain.ExitUsage {
		t.Fatalf("exit %d", code)
	}
}

func TestJiraSuggestRejectsJQLInProject(t *testing.T) {
	d, _, _ := testDeps()
	if code := Run([]string{"atlas", "jira", "suggest", "--query", "a", "--project", "SDO OR project = CAB"}, d); code != domain.ExitUsage {
		t.Fatalf("exit %d", code)
	}
}

func TestJiraSearchCrossCloudJQLIsUsage(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "jira", "search", "--jql", "project in (SDO, CAB)"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
	var e map[string]any
	if err := json.Unmarshal(errw.Bytes(), &e); err != nil {
		t.Fatal(errw.String())
	}
	if e["class"] != "usage" {
		t.Fatalf("%v", e)
	}
}

func TestJiraSearchSiteGardaIsUsage(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "jira", "search", "--site", "garda", "--jql", "project = EOS"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
	s := errw.String()
	if !strings.Contains(s, "jsm") {
		t.Fatal(s)
	}
	var e map[string]any
	if err := json.Unmarshal(errw.Bytes(), &e); err != nil {
		t.Fatal(s)
	}
	if e["class"] != "usage" {
		t.Fatalf("%v", e)
	}
	hint, _ := e["hint"].(string)
	if !strings.Contains(hint, "atlas jsm") {
		t.Fatal(hint)
	}
}

func TestJiraGetSiteGardaIsUsage(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "jira", "get", "--site", "garda", "EOS-1"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw.String())
	}
	if !strings.Contains(errw.String(), "jsm") {
		t.Fatal(errw.String())
	}
}
