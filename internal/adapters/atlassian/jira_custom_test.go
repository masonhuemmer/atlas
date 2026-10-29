package atlassian

import (
	"context"
	"reflect"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func TestMemoryGetCustomServesSeedIssue(t *testing.T) {
	api := JiraAPI{Memory: Seed()}
	got, err := api.GetCustom(context.Background(), host("sesamidevel"), "sdo-1", nil, true, true)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(got.CustomFields))
	for _, f := range got.CustomFields {
		ids = append(ids, f.ID)
	}
	if !reflect.DeepEqual(ids, []string{"customfield_10481", "customfield_10538", "customfield_10601"}) {
		t.Fatalf("ids %v", ids)
	}
	if len(got.StatusHistory) != 2 || got.StatusHistory[0].To != "In Progress" {
		t.Fatalf("history %+v", got.StatusHistory)
	}
}

func TestMemoryGetCustomSelection(t *testing.T) {
	api := JiraAPI{Memory: Seed()}
	got, err := api.GetCustom(context.Background(), host("sesamidevel"), "SDO-1", []string{"customfield_10601", "customfield_10602"}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.CustomFields) != 1 || got.CustomFields[0].Text != "EMEA / Frankfurt" || got.StatusHistory != nil {
		t.Fatalf("%+v", got)
	}
	got, err = api.GetCustom(context.Background(), host("sesamidevel"), "SDP-1", nil, true, true)
	if err != nil || len(got.CustomFields) != 0 || len(got.StatusHistory) != 0 {
		t.Fatalf("an issue without custom data returns nothing: %+v, %v", got, err)
	}
}

func TestMemoryGetCustomNotFound(t *testing.T) {
	api := JiraAPI{Memory: Seed()}
	for _, tc := range []struct{ host, key string }{
		{host("sesamidevel"), "SDO-999"},
		{host("sesami-io"), "SDO-1"},
	} {
		if _, err := api.GetCustom(context.Background(), tc.host, tc.key, nil, true, true); domain.ExitOf(err) != domain.ExitNotFound {
			t.Errorf("%s %s: %v", tc.host, tc.key, err)
		}
	}
}
