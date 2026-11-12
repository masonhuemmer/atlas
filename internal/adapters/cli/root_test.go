package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian"
	"github.com/masonhuemmer/atlas/internal/adapters/keychain"
	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/config"
	"github.com/masonhuemmer/atlas/internal/domain"
)

func testDeps() (Deps, *bytes.Buffer, *bytes.Buffer) {
	out, errw := &bytes.Buffer{}, &bytes.Buffer{}
	cfg, err := config.LoadFake()
	if err != nil {
		panic(err)
	}
	mem := atlassian.Seed()
	d := Deps{
		Config:     cfg,
		Store:      &keychain.Fake{},
		Login:      auth.FakeLogin(),
		Jira:       atlassian.JiraAPI{Memory: mem},
		Confluence: atlassian.ConfluenceAPI{Memory: mem},
		PR:         atlassian.PRAPI{Memory: mem},
		Pipeline:   atlassian.SeedPipelines(),
		JSM:        atlassian.JSMAPI{Memory: mem},
		Stdout:     out,
		Stderr:     errw,
	}
	return d, out, errw
}

func TestHelpNoSession(t *testing.T) {
	d, out, _ := testDeps()
	code := Run([]string{"atlas", "--help"}, d)
	if code != 0 {
		t.Fatal(code)
	}
	s := out.String()
	for _, want := range []string{"auth", "site", "jira", "confluence", "pr", "jsm", "mcp", "JSON", "3", "4", "5", "6"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
}

func TestJSONHumanMutex(t *testing.T) {
	d, _, errw := testDeps()
	code := Run([]string{"atlas", "--json", "--human", "auth", "status"}, d)
	if code != domain.ExitUsage {
		t.Fatal(code, errw)
	}
}
