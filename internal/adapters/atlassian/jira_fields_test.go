package atlassian

import (
	"context"
	"reflect"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestMemoryEditStoresCustomFields(t *testing.T) {
	mem := Seed()
	api := JiraAPI{Memory: mem}
	hostname := host("sesamidevel")
	fields := map[string]any{"customfield_10538": "first\n\nsecond", "customfield_10602": []any{map[string]any{"value": "PROD"}}}
	if _, err := api.Edit(context.Background(), hostname, "SDO-1", fields, true); err != nil {
		t.Fatal(err)
	}
	before, _ := api.GetCustom(context.Background(), hostname, "SDO-1", []string{"customfield_10538", "customfield_10602"}, false, false)
	if len(before.CustomFields) != 1 || before.CustomFields[0].Text != "Alex Example ; Avery Example" {
		t.Fatalf("dry-run stored custom fields: %+v", before.CustomFields)
	}
	if _, err := api.Edit(context.Background(), hostname, "SDO-1", fields, false); err != nil {
		t.Fatal(err)
	}
	after, _ := api.GetCustom(context.Background(), hostname, "SDO-1", []string{"customfield_10538", "customfield_10602"}, false, false)
	want := []domain.CustomField{
		{ID: "customfield_10538", Name: "Notification of Changes", Type: "textarea", Text: "first\nsecond"},
		{ID: "customfield_10602", Name: "Environments", Type: "multiselect", Text: "PROD"},
	}
	if !reflect.DeepEqual(after.CustomFields, want) {
		t.Fatalf("%+v", after.CustomFields)
	}
	if _, err := api.Edit(context.Background(), hostname, "SDO-1", map[string]any{"bogus": "x"}, false); domain.ExitOf(err) != domain.ExitUsage {
		t.Fatalf("unknown plain field must stay a usage error: %v", err)
	}
}

func TestMemoryCreateStoresCustomFieldsAndPreviewsPayload(t *testing.T) {
	mem := Seed()
	api := JiraAPI{Memory: mem}
	hostname := host("sesamidevel")
	in := domain.CreateIssue{
		Project: "SDO", IssueType: "Task", Summary: "with fields",
		Fields: map[string]any{"customfield_10538": "note", "customfield_10481": map[string]any{"accountId": "acct-alex", "displayName": "Alex Example"}},
	}
	payload, err := api.CreateFields(context.Background(), hostname, in)
	if err != nil {
		t.Fatal(err)
	}
	doc, ok := payload["customfield_10538"].(map[string]any)
	if !ok || doc["type"] != "doc" {
		t.Fatalf("textarea string must preview as ADF: %v", payload["customfield_10538"])
	}
	if payload["summary"] != "with fields" {
		t.Fatalf("%v", payload)
	}
	before := mem.IssueCount()
	created, err := api.Create(context.Background(), hostname, in, false)
	if err != nil || mem.IssueCount() != before+1 {
		t.Fatalf("%+v %v", created, err)
	}
	got, _ := api.GetCustom(context.Background(), hostname, created.Key, nil, true, false)
	if len(got.CustomFields) != 2 || got.CustomFields[0].Text != "Alex Example" || got.CustomFields[1].Text != "note" {
		t.Fatalf("%+v", got.CustomFields)
	}
}
