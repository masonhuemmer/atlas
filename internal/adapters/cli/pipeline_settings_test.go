package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const variableTestID = "{33333333-3333-3333-3333-333333333333}"
const environmentTestID = "{44444444-4444-4444-4444-444444444444}"
const scheduleTestID = "{55555555-5555-5555-5555-555555555555}"

func TestPipelineVariableListScopesAndHidesValues(t *testing.T) {
	for _, tc := range []struct {
		scope, path string
		flags       []string
	}{
		{"repository", "/2.0/repositories/builds/service/pipelines_config/variables", []string{"--repo", "service"}},
		{"workspace", "/2.0/workspaces/builds/pipelines-config/variables", nil},
		{"deployment", "/2.0/repositories/builds/service/deployments_config/environments/" + environmentTestID + "/variables", []string{"--repo", "service", "--environment", environmentTestID}},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != tc.path || r.URL.Query().Get("page") != "2" || r.URL.Query().Get("pagelen") != "10" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				if _, token, ok := r.BasicAuth(); !ok || token != "pipeline-token" {
					t.Error("wrong workspace credential")
				}
				_, _ = w.Write([]byte(`{"page":2,"pagelen":10,"values":[{"uuid":"{33333333-3333-3333-3333-333333333333}","key":"API_KEY","value":"must-never-appear","secured":true}],"next":"next-page"}`))
			})
			args := append([]string{"variable-list", "--workspace", "builds", "--scope", tc.scope, "--page", "2", "--limit", "10"}, tc.flags...)
			code, out, errw := run(args...)
			if code != domain.ExitOK {
				t.Fatal(code, errw)
			}
			if strings.Contains(out+errw, "must-never-appear") || strings.Contains(out, `"value"`) {
				t.Fatal("variable value exposed", out)
			}
			var got struct {
				Scope string `json:"scope"`
				Count int    `json:"count"`
				Next  string `json:"next"`
				Items []struct {
					UUID    string `json:"uuid"`
					Key     string `json:"key"`
					Secured bool   `json:"secured"`
				} `json:"items"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil {
				t.Fatal(err)
			}
			if got.Scope != tc.scope || got.Count != 1 || got.Next != "next-page" || got.Items[0].UUID != variableTestID || got.Items[0].Key != "API_KEY" || !got.Items[0].Secured {
				t.Fatalf("unexpected variables: %+v", got)
			}
		})
	}
}

func TestPipelineVariableWritesUseFileAndHideValues(t *testing.T) {
	value := "literal-secret\n"
	file := filepath.Join(t.TempDir(), "value")
	if err := os.WriteFile(file, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"variable-create", "variable-update"} {
		t.Run(verb, func(t *testing.T) {
			requests := 0
			_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				method, path := "POST", "/2.0/repositories/builds/service/pipelines_config/variables"
				if verb == "variable-update" {
					method = "PUT"
					path += "/" + variableTestID
				}
				if r.Method != method || r.URL.Path != path {
					t.Errorf("unexpected write: %s %s", r.Method, r.URL)
				}
				var body struct {
					Key, Value string
					Secured    bool
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Key != "API_KEY" || body.Value != value || !body.Secured {
					t.Errorf("incorrect variable metadata or value")
				}
				if verb == "variable-create" {
					w.WriteHeader(http.StatusCreated)
				}
				_, _ = w.Write([]byte(`{"uuid":"{33333333-3333-3333-3333-333333333333}","key":"API_KEY","value":"literal-secret\n","secured":true}`))
			})
			args := []string{verb, "--workspace", "builds", "--repo", "service", "--key", "API_KEY", "--value-file", file}
			if verb == "variable-update" {
				args = append(args, "--id", variableTestID)
			}
			code, out, errw := run(append(append([]string(nil), args...), "--dry-run")...)
			if code != domain.ExitOK || requests != 0 || !strings.Contains(out, `"dry_run":true`) {
				t.Fatal(code, out, errw, requests)
			}
			if strings.Contains(out+errw, "literal-secret") {
				t.Fatal("preview exposed value")
			}
			code, out, errw = run(args...)
			if code != domain.ExitOK || requests != 1 {
				t.Fatal(code, out, errw, requests)
			}
			if strings.Contains(out+errw, "literal-secret") || strings.Contains(out, `"value"`) {
				t.Fatal("write exposed value")
			}
		})
	}
}

func TestPipelineVariableGetAndDelete(t *testing.T) {
	for _, scope := range []string{"repository", "workspace", "deployment"} {
		t.Run(scope, func(t *testing.T) {
			deleted := false
			_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "DELETE" {
					deleted = true
					if !strings.HasSuffix(r.URL.Path, "/"+variableTestID) {
						t.Errorf("wrong deletion: %s", r.URL)
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if scope == "deployment" {
					if r.URL.Query().Get("page") == "1" {
						_, _ = w.Write([]byte(`{"values":[],"next":"second-page"}`))
						return
					}
					_, _ = w.Write([]byte(`{"values":[{"uuid":"{33333333-3333-3333-3333-333333333333}","key":"HOST","value":"hidden-value","secured":false}]}`))
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/"+variableTestID) {
					t.Errorf("wrong lookup: %s", r.URL)
				}
				_, _ = w.Write([]byte(`{"uuid":"{33333333-3333-3333-3333-333333333333}","key":"HOST","value":"hidden-value","secured":false}`))
			})
			flags := []string{"--workspace", "builds", "--scope", scope, "--id", variableTestID}
			if scope != "workspace" {
				flags = append(flags, "--repo", "service")
			}
			if scope == "deployment" {
				flags = append(flags, "--environment", environmentTestID)
			}
			code, out, errw := run(append([]string{"variable-get"}, flags...)...)
			if code != domain.ExitOK || !strings.Contains(out, `"key":"HOST"`) || strings.Contains(out, "hidden-value") {
				t.Fatal(code, out, errw)
			}
			code, _, errw = run(append(append([]string{"variable-delete"}, flags...), "--dry-run")...)
			if code != domain.ExitOK || deleted {
				t.Fatal(code, errw, deleted)
			}
			code, out, errw = run(append([]string{"variable-delete"}, flags...)...)
			if code != domain.ExitOK || !deleted || !strings.Contains(out, `"deleted":true`) {
				t.Fatal(code, out, errw, deleted)
			}
		})
	}
}

func TestPipelineScheduleListAndCreate(t *testing.T) {
	posts := 0
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/builds/service/pipelines_config/schedules" {
			t.Errorf("wrong schedule path: %s", r.URL)
		}
		if r.Method == "GET" {
			_, _ = w.Write([]byte(`{"values":[],"next":"second-page"}`))
			return
		}
		posts++
		var body struct {
			Enabled bool   `json:"enabled"`
			Cron    string `json:"cron_pattern"`
			Target  struct {
				Branch   string                         `json:"ref_name"`
				RefType  string                         `json:"ref_type"`
				Selector struct{ Type, Pattern string } `json:"selector"`
			} `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if r.Method != "POST" || body.Cron != "0 0 0 * * ? *" || !body.Enabled || body.Target.Branch != "main" || body.Target.RefType != "branch" || body.Target.Selector.Type != "custom" || body.Target.Selector.Pattern != "nightly" {
			t.Errorf("incorrect schedule body: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"{55555555-5555-5555-5555-555555555555}","enabled":true,"cron_pattern":"0 0 0 * * ? *","target":{"ref_name":"main","selector":{"type":"custom","pattern":"nightly"}}}`))
	})
	code, out, errw := run("schedule-list", "--workspace", "builds", "--repo", "service", "--page", "2")
	if code != domain.ExitOK || !strings.Contains(out, `"next":"second-page"`) {
		t.Fatal(code, out, errw)
	}
	args := []string{"schedule-create", "--workspace", "builds", "--repo", "service", "--branch", "main", "--custom", "nightly", "--cron", "0 0 0 * * ? *"}
	code, out, errw = run(append(append([]string(nil), args...), "--dry-run")...)
	if code != domain.ExitOK || posts != 0 || !strings.Contains(out, `"dry_run":true`) {
		t.Fatal(code, out, errw, posts)
	}
	code, out, errw = run(args...)
	if code != domain.ExitOK || posts != 1 || !strings.Contains(out, scheduleTestID) {
		t.Fatal(code, out, errw, posts)
	}
}

func TestPipelineScheduleGetDisableAndDelete(t *testing.T) {
	puts, deletes := 0, 0
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/builds/service/pipelines_config/schedules/"+scheduleTestID {
			t.Errorf("unexpected schedule: %s", r.URL)
		}
		if r.Method == "DELETE" {
			deletes++
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == "PUT" {
			puts++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body) != 1 || body["enabled"] != false {
				t.Errorf("update must only set enabled=false: %v", body)
			}
		}
		_, _ = w.Write([]byte(`{"uuid":"{55555555-5555-5555-5555-555555555555}","enabled":false,"cron_pattern":"0 0 0 * * ? *","target":{"ref_name":"main","selector":{"type":"custom","pattern":"nightly"}}}`))
	})
	flags := []string{"--workspace", "builds", "--repo", "service", "--id", scheduleTestID}
	code, out, errw := run(append([]string{"schedule-get"}, flags...)...)
	if code != domain.ExitOK || !strings.Contains(out, `"branch":"main"`) {
		t.Fatal(code, out, errw)
	}
	args := append(append([]string{"schedule-update"}, flags...), "--enabled=false")
	code, out, errw = run(append(append([]string(nil), args...), "--dry-run")...)
	if code != domain.ExitOK || puts != 0 || !strings.Contains(out, `"enabled":false`) {
		t.Fatal(code, out, errw, puts)
	}
	code, out, errw = run(args...)
	if code != domain.ExitOK || puts != 1 || !strings.Contains(out, `"enabled":false`) {
		t.Fatal(code, out, errw, puts)
	}
	code, _, errw = run(append(append([]string{"schedule-delete"}, flags...), "--dry-run")...)
	if code != domain.ExitOK || deletes != 0 {
		t.Fatal(code, errw, deletes)
	}
	code, out, errw = run(append([]string{"schedule-delete"}, flags...)...)
	if code != domain.ExitOK || deletes != 1 || !strings.Contains(out, `"deleted":true`) {
		t.Fatal(code, out, errw, deletes)
	}
}

func TestMCPPipelineSettingsWriteGateAndFalseFlags(t *testing.T) {
	file := filepath.Join(t.TempDir(), "value")
	if err := os.WriteFile(file, []byte("mcp-private-value"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, verb := range []string{"variable-create", "variable-update", "variable-delete", "schedule-create", "schedule-update", "schedule-delete"} {
		t.Run(verb, func(t *testing.T) {
			writes := 0
			d, _ := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				writes++
				if r.Method == "GET" {
					t.Error("write unexpectedly issued GET")
				}
				if r.Method == "DELETE" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if strings.HasPrefix(verb, "variable-") {
					if body["secured"] != false || body["value"] != "mcp-private-value" {
						t.Error("MCP did not preserve secured=false or value")
					}
					_, _ = w.Write([]byte(`{"uuid":"{33333333-3333-3333-3333-333333333333}","key":"HOST","value":"mcp-private-value","secured":false}`))
					return
				}
				if body["enabled"] != false {
					t.Error("MCP did not preserve enabled=false")
				}
				_, _ = w.Write([]byte(`{"uuid":"{55555555-5555-5555-5555-555555555555}","enabled":false}`))
			})
			client := connectMCP(t, d)
			flags := map[string]any{"workspace": "builds", "repo": "service", "dry-run": false}
			switch verb {
			case "variable-create", "variable-update":
				flags["key"] = "HOST"
				flags["value-file"] = file
				flags["secured"] = false
			case "schedule-create":
				flags["branch"] = "main"
				flags["cron"] = "0 0 0 * * ? *"
				flags["enabled"] = false
			case "schedule-update":
				flags["enabled"] = false
			}
			if strings.HasSuffix(verb, "update") || strings.HasSuffix(verb, "delete") {
				if strings.HasPrefix(verb, "variable-") {
					flags["id"] = variableTestID
				} else {
					flags["id"] = scheduleTestID
				}
			}
			res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_read", Arguments: readIn{Namespace: "pipeline", Verb: verb, Flags: flags}})
			if err != nil || !res.IsError || writes != 0 {
				t.Fatal("read gate failed", err, writes)
			}
			res, err = client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "pipeline", Verb: verb, Flags: flags}})
			if err != nil || res.IsError || writes != 0 || !strings.Contains(toolText(t, res), `"dry_run":true`) {
				t.Fatal("write preview failed", err, writes, toolText(t, res))
			}
			res, err = client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "pipeline", Verb: verb, Flags: flags, WriteOptIn: true}})
			if err != nil || res.IsError || writes != 1 || strings.Contains(toolText(t, res), "mcp-private-value") {
				t.Fatal("write failed or exposed value", err, writes)
			}
		})
	}
}

func TestMCPPipelineVariableRejectsTransportStdin(t *testing.T) {
	d, _ := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) { t.Error("MCP stdin request reached Bitbucket") })
	stdin := strings.NewReader("do-not-consume-mcp-transport")
	d.Stdin = stdin
	client := connectMCP(t, d)
	for _, value := range []any{"-", []string{"-"}} {
		res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "pipeline", Verb: "variable-create", Flags: map[string]any{"workspace": "builds", "repo": "service", "key": "KEY", "value-file": value}, WriteOptIn: true}})
		if err != nil || !res.IsError || !strings.Contains(toolText(t, res), "stdin") || stdin.Len() != len("do-not-consume-mcp-transport") {
			t.Fatal("transport stdin was not rejected", err)
		}
	}
}

func TestMCPPipelineSettingsReadsAndHelp(t *testing.T) {
	for _, verb := range []string{"variable-list", "variable-get", "schedule-list", "schedule-get"} {
		t.Run(verb, func(t *testing.T) {
			d, _ := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("read used %s", r.Method)
				}
				switch verb {
				case "variable-list", "schedule-list":
					_, _ = w.Write([]byte(`{"values":[]}`))
				case "variable-get":
					_, _ = w.Write([]byte(`{"uuid":"{33333333-3333-3333-3333-333333333333}","key":"KEY","value":"read-hidden","secured":false}`))
				case "schedule-get":
					_, _ = w.Write([]byte(`{"uuid":"{55555555-5555-5555-5555-555555555555}","enabled":true}`))
				}
			})
			client := connectMCP(t, d)
			flags := map[string]any{"workspace": "builds", "repo": "service"}
			if verb == "variable-get" {
				flags["id"] = variableTestID
			}
			if verb == "schedule-get" {
				flags["id"] = scheduleTestID
			}
			res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_read", Arguments: readIn{Namespace: "pipeline", Verb: verb, Flags: flags}})
			if err != nil || res.IsError || strings.Contains(toolText(t, res), "read-hidden") {
				t.Fatal(err, toolText(t, res))
			}
			res, err = client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_help", Arguments: helpIn{Namespace: "pipeline", Verb: verb}})
			if err != nil || res.IsError || !strings.Contains(toolText(t, res), verb) {
				t.Fatal(err, toolText(t, res))
			}
		})
	}
}

func TestPipelineSettingsInvalidInputMakesNoRequest(t *testing.T) {
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("invalid input reached Bitbucket: %s", r.URL) })
	for _, args := range [][]string{
		{"variable-list", "--scope", "workspace", "--repo", "service"},
		{"variable-list", "--scope", "deployment", "--repo", "service"},
		{"variable-list", "--scope", "repository", "--repo", "service", "--environment", environmentTestID},
		{"variable-list", "--scope", "other"},
		{"variable-list", "--repo", "service", "--page", "0"},
		{"variable-create", "--repo", "service", "--key", "1BAD", "--value-file", "missing"},
		{"variable-update", "--repo", "service", "--id", variableTestID, "--key", "KEY"},
		{"variable-delete", "--repo", "service", "--id", ".."},
		{"schedule-create", "--repo", "service", "--branch", "main", "--cron", "0 0 * * *"},
		{"schedule-create", "--repo", "service", "--cron", "0 0 0 * * ? *"},
		{"schedule-update", "--repo", "service", "--id", scheduleTestID},
		{"schedule-update", "--repo", "service", "--id", scheduleTestID, "--enabled=false", "--cron", "0 0 0 * * ? *"},
		{"schedule-delete", "--repo", "../service", "--id", scheduleTestID},
		{"schedule-get", "--repo", "service", "--id", "not-a-uuid"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, _, errw := run(append(args, "--workspace", "builds")...)
			if code != domain.ExitUsage {
				t.Fatal(code, errw)
			}
		})
	}
}

func TestPipelineVariableErrorNeverEchoesValue(t *testing.T) {
	file := filepath.Join(t.TempDir(), "value")
	value := "private-setting-value"
	if err := os.WriteFile(file, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{400, 403, 409} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code); _, _ = w.Write([]byte(value)) })
			exit, out, errw := run("variable-create", "--workspace", "builds", "--repo", "service", "--key", "KEY", "--value-file", file)
			want := domain.ExitUsage
			if code == 403 {
				want = domain.ExitAuth
			}
			if exit != want || strings.Contains(out+errw, value) {
				t.Fatal(exit, out, errw)
			}
			if code == 403 && !strings.Contains(errw, "admin:pipeline:bitbucket") {
				t.Fatal("missing variable administration scope hint", errw)
			}
		})
	}
}

func TestPipelineSettingsFakeRoundTrip(t *testing.T) {
	d, out, errw := testDeps()
	run := func(args ...string) int {
		out.Reset()
		errw.Reset()
		return Run(append([]string{"atlas", "pipeline"}, args...), d)
	}
	d.Stdin = strings.NewReader("fake-secret")
	if code := run("variable-create", "--scope", "workspace", "--key", "KEY", "--value-file", "-"); code != 0 {
		t.Fatal(code, errw.String())
	}
	var variable domain.PipelineVariable
	if err := json.Unmarshal(out.Bytes(), &variable); err != nil || !variable.Secured {
		t.Fatal(err, out.String())
	}
	d.Stdin = strings.NewReader("replacement")
	if code := run("variable-update", "--scope", "workspace", "--id", variable.UUID, "--key", "NEW_KEY", "--value-file", "-", "--secured=false"); code != 0 {
		t.Fatal(code, errw.String())
	}
	if code := run("variable-get", "--scope", "workspace", "--id", variable.UUID); code != 0 || !strings.Contains(out.String(), `"key":"NEW_KEY"`) || !strings.Contains(out.String(), `"secured":false`) {
		t.Fatal(code, out.String(), errw.String())
	}
	if code := run("variable-list", "--repo", "service"); code != 0 || !strings.Contains(out.String(), `"count":0`) {
		t.Fatal("variable leaked between scopes", code, out.String())
	}
	if code := run("variable-delete", "--scope", "workspace", "--id", variable.UUID); code != 0 {
		t.Fatal(code, errw.String())
	}
	if code := run("variable-get", "--scope", "workspace", "--id", variable.UUID); code != domain.ExitNotFound {
		t.Fatal(code, errw.String())
	}
	if code := run("schedule-create", "--repo", "service", "--branch", "main", "--cron", "0 0 0 * * ? *"); code != 0 {
		t.Fatal(code, errw.String())
	}
	var schedule domain.PipelineSchedule
	if err := json.Unmarshal(out.Bytes(), &schedule); err != nil || !schedule.Enabled || schedule.SelectorType != "default" {
		t.Fatal(err, out.String())
	}
	if code := run("schedule-update", "--repo", "service", "--id", schedule.UUID, "--enabled=false"); code != 0 {
		t.Fatal(code, errw.String())
	}
	if code := run("schedule-get", "--repo", "service", "--id", schedule.UUID); code != 0 || !strings.Contains(out.String(), `"enabled":false`) {
		t.Fatal(code, out.String(), errw.String())
	}
	if code := run("schedule-delete", "--repo", "service", "--id", schedule.UUID); code != 0 {
		t.Fatal(code, errw.String())
	}
	if code := run("schedule-get", "--repo", "service", "--id", schedule.UUID); code != domain.ExitNotFound {
		t.Fatal(code, errw.String())
	}
}
