package cli

import (
	"reflect"
	"testing"
)

func TestFlagMapToArgs(t *testing.T) {
	got, err := FlagMapToArgs("jira", "search", nil, map[string]any{"jql": "project = CAB", "site": "sesami-io"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"atlas", "jira", "search", "--jql=project = CAB", "--site=sesami-io"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%v != %v", got, want)
	}
}

func TestJiraUsersIsReadOnly(t *testing.T) {
	args, err := buildReadArgs("jira", "users", nil, map[string]any{"query": "Alex", "project": "SDO"})
	if err != nil || len(args) != 5 {
		t.Fatalf("%v %v", args, err)
	}
	if isWrite("jira", "users") {
		t.Fatal("user lookup is a write")
	}
}

func TestWriteGateInjectsDryRun(t *testing.T) {
	writes := [][2]string{
		{"jira", "create"},
		{"jira", "edit"},
		{"jira", "comment"},
		{"jira", "transition"},
		{"jira", "link"},
		{"confluence", "create"},
		{"confluence", "update"},
		{"confluence", "move"},
		{"pr", "create"},
		{"pr", "edit"},
		{"pr", "comment"},
		{"pr", "merge"},
		{"jsm", "create"},
		{"jsm", "comment"},
		{"jsm", "transition"},
	}
	for _, w := range writes {
		if !isWrite(w[0], w[1]) {
			t.Fatalf("isWrite %s %s", w[0], w[1])
		}
		args, err := buildWriteArgs(w[0], w[1], []string{"SDO-1"}, map[string]any{"dry-run": false}, false)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, a := range args {
			if a == "--dry-run=true" {
				found = true
			}
		}
		if !found {
			t.Fatal(args)
		}
		args, err = buildWriteArgs(w[0], w[1], nil, nil, true)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range args {
			if a == "--dry-run=true" {
				t.Fatal(args)
			}
		}
	}
	if isWrite("jira", "get") || isWrite("jira", "search") || isWrite("confluence", "get") || isWrite("confluence", "search") || isWrite("confluence", "delete") || isWrite("pr", "get") || isWrite("pr", "list") || isWrite("pr", "diff") || isWrite("pr", "delete") || isWrite("jsm", "desks") || isWrite("jsm", "types") || isWrite("jsm", "list") || isWrite("jsm", "get") {
		t.Fatal("reads and delete are not writes")
	}
	args, err := buildReadArgs("jira", "get", []string{"SDO-1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range args {
		if a == "--dry-run=true" {
			t.Fatal(args)
		}
	}
}

func TestReadAllowlist(t *testing.T) {
	reads := [][2]string{
		{"auth", "status"},
		{"site", "list"},
		{"site", "resolve"},
		{"jira", "get"},
		{"jira", "search"},
		{"confluence", "get"},
		{"confluence", "search"},
		{"pr", "get"},
		{"pr", "list"},
		{"pr", "diff"},
		{"jsm", "desks"},
		{"jsm", "types"},
		{"jsm", "list"},
		{"jsm", "get"},
	}
	for _, r := range reads {
		if !isRead(r[0], r[1]) || isWrite(r[0], r[1]) {
			t.Fatalf("%s %s", r[0], r[1])
		}
	}
	for _, v := range [][2]string{{"jira", "comment"}, {"pr", "merge"}, {"confluence", "delete"}, {"auth", "login"}, {"auth", "logout"}, {"mcp", "serve"}} {
		if isRead(v[0], v[1]) {
			t.Fatalf("%s %s is not a read", v[0], v[1])
		}
	}
}
