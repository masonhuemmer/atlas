package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/masonhuemmer/atlas/internal/adapters/atlassian/rest"
	"github.com/masonhuemmer/atlas/internal/app/auth"
	"github.com/masonhuemmer/atlas/internal/domain"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const pipelineTestID = "{11111111-1111-1111-1111-111111111111}"
const pipelineStepID = "{22222222-2222-2222-2222-222222222222}"

func pipelineHTTPDeps(t *testing.T, handler http.HandlerFunc) (Deps, func(...string) (int, string, string)) {
	t.Helper()
	d, out, errw := testDeps()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	if err := auth.PutWorkspace(d.Store, "builds", auth.Cred{Email: "build@example.com", Token: "pipeline-token"}); err != nil {
		t.Fatal(err)
	}
	d.Pipeline = rest.BitbucketPipelines{Client: &rest.Client{HTTP: server.Client(), Store: d.Store, BBBase: server.URL}}
	return d, func(args ...string) (int, string, string) {
		out.Reset()
		errw.Reset()
		code := Run(append([]string{"atlas", "pipeline"}, args...), d)
		return code, out.String(), errw.String()
	}
}

func TestPipelineListUsesWorkspaceFiltersAndPagination(t *testing.T) {
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/2.0/repositories/builds/service/pipelines" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		if email, token, ok := r.BasicAuth(); !ok || email != "build@example.com" || token != "pipeline-token" {
			t.Error("request did not use the selected workspace credential")
		}
		for key, want := range map[string]string{"target.ref_name": "feature/build", "status": "FAILED", "page": "2", "pagelen": "25"} {
			if got := r.URL.Query().Get(key); got != want {
				t.Errorf("query %s = %q, want %q", key, got, want)
			}
		}
		_, _ = w.Write([]byte(`{"page":2,"pagelen":25,"next":"https://api.bitbucket.org/2.0/repositories/builds/service/pipelines?page=3","values":[{"uuid":"{11111111-1111-1111-1111-111111111111}","build_number":42,"state":{"name":"COMPLETED","result":{"name":"FAILED"}},"target":{"ref_type":"branch","ref_name":"feature/build","commit":{"hash":"abc123"}},"created_on":"2026-11-12T12:00:00Z"}]}`))
	})
	code, out, errw := run("list", "--workspace", "builds", "--repo", "service", "--branch", "feature/build", "--status", "failed", "--page", "2", "--limit", "25")
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var got struct {
		Workspace string `json:"workspace"`
		Page      int    `json:"page"`
		Next      string `json:"next"`
		Items     []struct {
			UUID        string `json:"uuid"`
			State       string `json:"state"`
			Result      string `json:"result"`
			Branch      string `json:"ref_name"`
			BuildNumber int    `json:"build_number"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err, out)
	}
	if got.Workspace != "builds" || got.Page != 2 || got.Next == "" || len(got.Items) != 1 {
		t.Fatalf("unexpected page: %+v", got)
	}
	item := got.Items[0]
	if item.UUID != pipelineTestID || item.State != "COMPLETED" || item.Result != "FAILED" || item.Branch != "feature/build" || item.BuildNumber != 42 {
		t.Fatalf("unexpected pipeline: %+v", item)
	}
}

func TestPipelineGetShowsResultAndTarget(t *testing.T) {
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/2.0/repositories/builds/service/pipelines/"+pipelineTestID {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		_, _ = w.Write([]byte(`{"uuid":"{11111111-1111-1111-1111-111111111111}","build_number":42,"state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}},"target":{"ref_type":"branch","ref_name":"main","selector":{"type":"custom","pattern":"release"}},"links":{"html":{"href":"https://bitbucket.org/builds/service/pipelines/results/42"}}}`))
	})
	code, out, errw := run("get", "--workspace", "builds", "--repo", "service", "--id", pipelineTestID)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var got domain.Pipeline
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.UUID != pipelineTestID || got.Result != "SUCCESSFUL" || got.Custom != "release" || got.URL != "https://bitbucket.org/builds/service/pipelines/results/42" {
		t.Fatalf("unexpected run: %+v", got)
	}
}

func TestPipelineStepsShowIndividualResults(t *testing.T) {
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/builds/service/pipelines/"+pipelineTestID+"/steps" || r.URL.Query().Get("page") != "2" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		_, _ = w.Write([]byte(`{"values":[{"uuid":"{22222222-2222-2222-2222-222222222222}","name":"Unit tests","state":{"name":"COMPLETED","result":{"name":"FAILED"}},"started_on":"2026-11-12T12:00:00Z","completed_on":"2026-11-12T12:01:00Z"}]}`))
	})
	code, out, errw := run("steps", "--workspace", "builds", "--repo", "service", "--id", pipelineTestID, "--page", "2")
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var got struct {
		UUID  string `json:"uuid"`
		Items []struct {
			UUID   string `json:"uuid"`
			Name   string `json:"name"`
			Result string `json:"result"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.UUID != pipelineTestID || len(got.Items) != 1 || got.Items[0].UUID != pipelineStepID || got.Items[0].Name != "Unit tests" || got.Items[0].Result != "FAILED" {
		t.Fatalf("unexpected steps: %+v", got)
	}
}

func TestPipelineLogsPreserveLargeOutput(t *testing.T) {
	log := strings.Repeat("build output\n", 350000)
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/builds/service/pipelines/"+pipelineTestID+"/steps/"+pipelineStepID+"/log" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(log))
	})
	args := []string{"logs", "--workspace", "builds", "--repo", "service", "--id", pipelineTestID, "--step", pipelineStepID}
	code, out, errw := run(args...)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var got struct {
		Log string `json:"log"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Log != log {
		t.Fatalf("log truncated: got %d, want %d bytes", len(got.Log), len(log))
	}
	code, out, errw = run(append(args, "--human")...)
	if code != domain.ExitOK || out != log {
		t.Fatalf("human log output: exit=%d bytes=%d error=%s", code, len(out), errw)
	}
}

func TestPipelineRunPreviewAndCustomTarget(t *testing.T) {
	requests := 0
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "POST" || r.URL.Path != "/2.0/repositories/builds/service/pipelines" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		target, _ := body["target"].(map[string]any)
		selector, _ := target["selector"].(map[string]any)
		if target["type"] != "pipeline_ref_target" || target["ref_type"] != "branch" || target["ref_name"] != "release/test" || selector["type"] != "custom" || selector["pattern"] != "Deploy preview" {
			t.Errorf("unexpected target: %#v", target)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"{11111111-1111-1111-1111-111111111111}","build_number":43,"state":{"name":"PENDING"}}`))
	})
	args := []string{"run", "--workspace", "builds", "--repo", "service", "--branch", "release/test", "--custom", "Deploy preview"}
	code, out, errw := run(append(append([]string(nil), args...), "--dry-run")...)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var preview map[string]any
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatal(err)
	}
	if requests != 0 || preview["dry_run"] != true || preview["branch"] != "release/test" || preview["custom"] != "Deploy preview" {
		t.Fatalf("bad preview: requests=%d preview=%v", requests, preview)
	}
	code, out, errw = run(args...)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var got domain.Pipeline
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if requests != 1 || got.UUID != pipelineTestID || got.State != "PENDING" {
		t.Fatalf("unexpected run: %+v requests=%d", got, requests)
	}
}

func TestPipelineStopAcknowledgesRequest(t *testing.T) {
	requests := 0
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != "POST" || r.URL.Path != "/2.0/repositories/builds/service/pipelines/"+pipelineTestID+"/stopPipeline" {
			t.Errorf("unexpected stop request: %s %s", r.Method, r.URL)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	args := []string{"stop", "--workspace", "builds", "--repo", "service", "--id", pipelineTestID}
	code, out, errw := run(append(append([]string(nil), args...), "--dry-run")...)
	if code != domain.ExitOK || requests != 0 {
		t.Fatal(code, out, errw, requests)
	}
	code, out, errw = run(args...)
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
	var got struct {
		UUID          string `json:"uuid"`
		StopRequested bool   `json:"stop_requested"`
		State         string `json:"state"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if !got.StopRequested || got.UUID != pipelineTestID || got.State != "" || requests != 1 {
		t.Fatalf("bad acknowledgement: %+v requests=%d", got, requests)
	}
}

func TestMCPPipelineWritesRequireOptIn(t *testing.T) {
	for _, verb := range []string{"run", "stop"} {
		t.Run(verb, func(t *testing.T) {
			requests := 0
			d, _ := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.Method != "POST" {
					t.Errorf("unexpected method: %s", r.Method)
				}
				if verb == "stop" {
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"uuid":"{11111111-1111-1111-1111-111111111111}","state":{"name":"PENDING"}}`))
			})
			client := connectMCP(t, d)
			flags := map[string]any{"workspace": "builds", "repo": "service", "dry-run": false}
			if verb == "run" {
				flags["branch"] = "main"
			} else {
				flags["id"] = pipelineTestID
			}
			res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_read", Arguments: readIn{Namespace: "pipeline", Verb: verb, Flags: flags}})
			if err != nil || !res.IsError || requests != 0 {
				t.Fatal("read tool allowed pipeline write", err, requests)
			}
			res, err = client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "pipeline", Verb: verb, Flags: flags}})
			if err != nil || res.IsError || requests != 0 || !strings.Contains(toolText(t, res), `"dry_run":true`) {
				t.Fatal("write was not a preview", err, requests, toolText(t, res))
			}
			res, err = client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_write", Arguments: writeIn{Namespace: "pipeline", Verb: verb, Flags: flags, WriteOptIn: true}})
			if err != nil || res.IsError || requests != 1 {
				t.Fatal(err, requests, toolText(t, res))
			}
		})
	}
}

func TestPipelineFakeRunCanBeRetrievedAndStopped(t *testing.T) {
	d, out, errw := testDeps()
	base := []string{"atlas", "pipeline"}
	code := Run(append(base, "run", "--repo", "demo", "--branch", "main"), d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var created domain.Pipeline
	if err := json.Unmarshal(out.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	code = Run(append(base, "stop", "--repo", "demo", "--id", created.UUID), d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	out.Reset()
	code = Run(append(base, "get", "--repo", "demo", "--id", created.UUID), d)
	if code != domain.ExitOK {
		t.Fatal(code, errw.String())
	}
	var stopped domain.Pipeline
	if err := json.Unmarshal(out.Bytes(), &stopped); err != nil {
		t.Fatal(err)
	}
	if stopped.UUID != created.UUID || stopped.State != "COMPLETED" || stopped.Result != "STOPPED" {
		t.Fatalf("unexpected stopped run: %+v", stopped)
	}
}

func TestPipelineInvalidInputMakesNoRequest(t *testing.T) {
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("invalid input reached Bitbucket: %s", r.URL) })
	cases := [][]string{
		{"list", "--page", "0"}, {"list", "--limit", "101"}, {"list", "--status", "NOT_A_STATUS"},
		{"get"}, {"get", "--id", "../other"}, {"logs", "--id", pipelineTestID},
		{"run"}, {"run", "--branch", " "}, {"run", "--branch", "main", "--custom", " "},
		{"stop", "--id", "42"}, {"list", "unexpected-argument"}, {"list", "--repo", ".."},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			args = append([]string{args[0], "--workspace", "builds", "--repo", "service"}, args[1:]...)
			code, _, errw := run(args...)
			if code != domain.ExitUsage {
				t.Fatalf("expected usage error, got %d: %s", code, errw)
			}
		})
	}
}

func TestPipelineWorkspaceCredentialDoesNotFallBackToSite(t *testing.T) {
	d, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("request used an unrelated credential") })
	if err := auth.PutSite(d.Store, "sesamidevel", auth.Cred{Email: "site@example.com", Token: "site-token"}); err != nil {
		t.Fatal(err)
	}
	code, _, errw := run("list", "--workspace", "missing", "--repo", "service")
	if code != domain.ExitAuth || !strings.Contains(errw, "auth login --workspace missing") {
		t.Fatal(code, errw)
	}
}

func TestPipelineErrorsRemainClassified(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		exit   int
	}{
		{"bad_request", 400, "", domain.ExitUsage}, {"unauthorized", 401, "", domain.ExitAuth},
		{"forbidden", 403, "", domain.ExitAuth}, {"not_found", 404, "", domain.ExitNotFound},
		{"rate_limit", 429, "", domain.ExitService}, {"server_error", 500, "", domain.ExitService},
		{"invalid_json", 200, "{", domain.ExitService}, {"missing_uuid", 200, "{}", domain.ExitService},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})
			code, _, errw := run("get", "--workspace", "builds", "--repo", "service", "--id", pipelineTestID)
			if code != tc.exit {
				t.Fatalf("got %d want %d: %s", code, tc.exit, errw)
			}
		})
	}
}

func TestMCPPipelineReadCommands(t *testing.T) {
	for _, verb := range []string{"list", "get", "steps", "logs"} {
		t.Run(verb, func(t *testing.T) {
			d, _ := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Errorf("read issued %s", r.Method)
				}
				switch verb {
				case "list", "steps":
					_, _ = w.Write([]byte(`{"values":[]}`))
				case "get":
					_, _ = w.Write([]byte(`{"uuid":"{11111111-1111-1111-1111-111111111111}","state":{"name":"IN_PROGRESS"}}`))
				case "logs":
					_, _ = w.Write([]byte("step output\n"))
				}
			})
			client := connectMCP(t, d)
			flags := map[string]any{"workspace": "builds", "repo": "service"}
			if verb != "list" {
				flags["id"] = pipelineTestID
			}
			if verb == "logs" {
				flags["step"] = pipelineStepID
			}
			res, err := client.CallTool(context.Background(), &mcp.CallToolParams{Name: "atlas_read", Arguments: readIn{Namespace: "pipeline", Verb: verb, Flags: flags}})
			if err != nil || res.IsError {
				t.Fatal(err, toolText(t, res))
			}
			var result map[string]any
			if err := json.Unmarshal([]byte(toolText(t, res)), &result); err != nil || result["workspace"] != "builds" {
				t.Fatal(err, result)
			}
		})
	}
}

func TestPipelineLogsRedirectWithoutForwardingCredentials(t *testing.T) {
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("workspace credential forwarded to log storage")
		}
		_, _ = w.Write([]byte("signed log\n"))
	}))
	defer storage.Close()
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if _, token, ok := r.BasicAuth(); !ok || token != "pipeline-token" {
			t.Error("missing initial workspace authentication")
		}
		http.Redirect(w, r, storage.URL+"/signed-log", http.StatusTemporaryRedirect)
	})
	code, out, errw := run("logs", "--workspace", "builds", "--repo", "service", "--id", pipelineTestID, "--step", pipelineStepID, "--human")
	if code != domain.ExitOK || out != "signed log\n" {
		t.Fatal(code, out, errw)
	}
}

func TestPipelineIncompleteLogIsAnError(t *testing.T) {
	_, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("short log"))
	})
	code, out, errw := run("logs", "--workspace", "builds", "--repo", "service", "--id", pipelineTestID, "--step", pipelineStepID)
	if code != domain.ExitService || out != "" || !strings.Contains(errw, "complete pipeline log") {
		t.Fatal(code, out, errw)
	}
}

func TestPipelineDefaultWorkspaceAndBranchRun(t *testing.T) {
	d, run := pipelineHTTPDeps(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/repositories/sesamiio/service/pipelines" {
			t.Errorf("default workspace: %s", r.URL)
		}
		if _, token, ok := r.BasicAuth(); !ok || token != "default-token" {
			t.Error("incorrect default workspace credential")
		}
		var body struct {
			Target map[string]any `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Target["ref_name"] != "main" || body.Target["type"] != "pipeline_ref_target" || body.Target["selector"] != nil {
			t.Errorf("unexpected default branch target: %v", body.Target)
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"uuid":"{11111111-1111-1111-1111-111111111111}"}`))
	})
	if err := auth.PutWorkspace(d.Store, domain.DefaultWorkspace(), auth.Cred{Email: "default@example.com", Token: "default-token"}); err != nil {
		t.Fatal(err)
	}
	code, _, errw := run("run", "--repo", "service", "--branch", "main")
	if code != domain.ExitOK {
		t.Fatal(code, errw)
	}
}
