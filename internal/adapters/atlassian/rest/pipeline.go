package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// BitbucketPipelines shares workspace credentials and HTTP transport with PRs.
type BitbucketPipelines struct {
	*Client
}

func (b BitbucketPipelines) Stop(ctx context.Context, workspace, repo, id string) (domain.PipelineStop, error) {
	if err := b.request(ctx, workspace, repo, http.MethodPost, "/"+url.PathEscape(id)+"/stopPipeline", nil, nil); err != nil {
		return domain.PipelineStop{}, err
	}
	return domain.PipelineStop{UUID: id, Workspace: workspace, Repo: repo, StopRequested: true}, nil
}

func (b BitbucketPipelines) Run(ctx context.Context, workspace, repo string, in domain.RunPipeline) (domain.Pipeline, error) {
	target := map[string]any{"type": "pipeline_ref_target", "ref_type": "branch", "ref_name": in.Branch}
	if in.Custom != "" {
		target["selector"] = map[string]any{"type": "custom", "pattern": in.Custom}
	}
	var raw map[string]any
	if err := b.request(ctx, workspace, repo, http.MethodPost, "", map[string]any{"target": target}, &raw); err != nil {
		return domain.Pipeline{}, err
	}
	if str(raw, "uuid") == "" {
		return domain.Pipeline{}, domain.Service("Bitbucket did not return the started pipeline UUID")
	}
	return pipelineFromREST(workspace, repo, raw), nil
}

func (b BitbucketPipelines) Get(ctx context.Context, workspace, repo, id string) (domain.Pipeline, error) {
	var raw map[string]any
	if err := b.request(ctx, workspace, repo, http.MethodGet, "/"+url.PathEscape(id), nil, &raw); err != nil {
		return domain.Pipeline{}, err
	}
	if str(raw, "uuid") == "" {
		return domain.Pipeline{}, domain.Service("Bitbucket returned a pipeline without a UUID")
	}
	return pipelineFromREST(workspace, repo, raw), nil
}

func (b BitbucketPipelines) List(ctx context.Context, workspace, repo string, in domain.PipelineListOptions) (domain.PipelineList, error) {
	query := url.Values{"page": {strconv.Itoa(in.Page)}, "pagelen": {strconv.Itoa(in.Limit)}}
	if in.Branch != "" {
		query.Set("target.ref_type", "BRANCH")
		query.Set("target.ref_name", in.Branch)
	}
	if in.Status != "" {
		query.Set("status", in.Status)
	}
	var response struct {
		Values []map[string]any `json:"values"`
		Next   string           `json:"next"`
	}
	if err := b.request(ctx, workspace, repo, http.MethodGet, "?"+query.Encode(), nil, &response); err != nil {
		return domain.PipelineList{}, err
	}
	page := domain.PipelineList{Workspace: workspace, Repo: repo, Page: in.Page, PageLen: in.Limit, Next: response.Next, Items: []domain.Pipeline{}}
	for _, raw := range response.Values {
		page.Items = append(page.Items, pipelineFromREST(workspace, repo, raw))
	}
	page.Count = len(page.Items)
	return page, nil
}

func (b BitbucketPipelines) Steps(ctx context.Context, workspace, repo, id string, page, limit int) (domain.PipelineStepList, error) {
	query := url.Values{"page": {strconv.Itoa(page)}, "pagelen": {strconv.Itoa(limit)}}
	var response struct {
		Values []map[string]any `json:"values"`
		Next   string           `json:"next"`
	}
	if err := b.request(ctx, workspace, repo, http.MethodGet, "/"+url.PathEscape(id)+"/steps?"+query.Encode(), nil, &response); err != nil {
		return domain.PipelineStepList{}, err
	}
	result := domain.PipelineStepList{UUID: id, Workspace: workspace, Repo: repo, Page: page, PageLen: limit, Next: response.Next, Items: []domain.PipelineStep{}}
	for _, raw := range response.Values {
		state := asMap(raw["state"])
		result.Items = append(result.Items, domain.PipelineStep{
			UUID: str(raw, "uuid"), Name: str(raw, "name"), State: str(state, "name"),
			Result: str(asMap(state["result"]), "name"), StartedOn: str(raw, "started_on"), CompletedOn: str(raw, "completed_on"),
		})
	}
	result.Count = len(result.Items)
	return result, nil
}

func (b BitbucketPipelines) Logs(ctx context.Context, workspace, repo, id, step string) (domain.PipelineLog, error) {
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return domain.PipelineLog{}, err
	}
	u := b.endpoint(workspace, repo, "/"+url.PathEscape(id)+"/steps/"+url.PathEscape(step)+"/log")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return domain.PipelineLog{}, domain.Usage(err.Error())
	}
	req.SetBasicAuth(cred.Email, cred.Token)
	req.Header.Set("Accept", "text/plain")
	// Logs may redirect to signed storage URLs. Authenticate only the original
	// origin, even when net/http would otherwise trust one of its subdomains.
	client := *b.httpc()
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		if next.URL.Scheme == req.URL.Scheme && next.URL.Host == req.URL.Host {
			next.SetBasicAuth(cred.Email, cred.Token)
		} else {
			next.Header.Del("Authorization")
		}
		return nil
	}
	res, err := client.Do(req)
	if err != nil {
		return domain.PipelineLog{}, domain.Service("could not download pipeline log")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return domain.PipelineLog{}, pipelineError(res.StatusCode, workspace)
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return domain.PipelineLog{}, domain.Service("could not read complete pipeline log")
	}
	return domain.PipelineLog{UUID: id, Workspace: workspace, Repo: repo, Step: step, Log: string(body)}, nil
}

func (b BitbucketPipelines) endpoint(workspace, repo, suffix string) string {
	return joinURL(b.bitbucketOrigin(), "/2.0/repositories/"+url.PathEscape(workspace)+"/"+url.PathEscape(repo)+"/pipelines"+suffix)
}

func (b BitbucketPipelines) request(ctx context.Context, workspace, repo, method, suffix string, payload, result any) error {
	return b.requestURL(ctx, workspace, method, b.endpoint(workspace, repo, suffix), payload, result)
}

func (b BitbucketPipelines) requestURL(ctx context.Context, workspace, method, endpoint string, payload, result any) error {
	cred, err := b.workspaceCred(workspace)
	if err != nil {
		return err
	}
	code, body, err := b.doJSON(ctx, method, endpoint, cred, nil, payload)
	if err != nil {
		return err
	}
	if code < 200 || code >= 300 {
		variableEndpoint := strings.Contains(endpoint, "/pipelines_config/variables") || strings.Contains(endpoint, "/pipelines-config/variables") || strings.Contains(endpoint, "/deployments_config/environments/")
		if (code == http.StatusUnauthorized || code == http.StatusForbidden) && method != http.MethodGet && variableEndpoint {
			return domain.Auth("not authorized to manage Bitbucket pipeline variables").WithHint("check repository/workspace access and admin:pipeline:bitbucket scope; atlas auth login --workspace " + workspace)
		}
		return pipelineError(code, workspace)
	}
	if result != nil && json.Unmarshal(body, result) != nil {
		return domain.Service("invalid JSON in Bitbucket pipeline response")
	}
	return nil
}

func pipelineError(code int, workspace string) error {
	switch code {
	case http.StatusBadRequest:
		return domain.Usage("Bitbucket rejected the pipeline request").WithHint("check the command flags and Bitbucket pipeline configuration")
	case http.StatusUnauthorized, http.StatusForbidden:
		return domain.Auth("not authorized for Bitbucket pipelines").WithHint("check repository access and read:pipeline:bitbucket / write:pipeline:bitbucket scopes; atlas auth login --workspace " + workspace)
	case http.StatusNotFound:
		return domain.NotFound("Bitbucket pipeline resource not found")
	case http.StatusConflict:
		return domain.Usage("pipeline setting conflicts with an existing setting")
	default:
		return domain.Service("Bitbucket pipelines service error (HTTP " + strconv.Itoa(code) + ")")
	}
}

func pipelineFromREST(workspace, repo string, raw map[string]any) domain.Pipeline {
	target := asMap(raw["target"])
	state := asMap(raw["state"])
	build, _ := raw["build_number"].(float64)
	custom := ""
	if selector := asMap(target["selector"]); str(selector, "type") == "custom" {
		custom = str(selector, "pattern")
	}
	return domain.Pipeline{
		UUID: str(raw, "uuid"), Workspace: workspace, Repo: repo, BuildNumber: int(build),
		State: str(state, "name"), Result: str(asMap(state["result"]), "name"),
		RefType: str(target, "ref_type"), RefName: str(target, "ref_name"),
		Commit: str(asMap(target["commit"]), "hash"), Custom: custom,
		CreatedOn: str(raw, "created_on"), CompletedOn: str(raw, "completed_on"),
		URL: str(asMap(asMap(raw["links"])["html"]), "href"),
	}
}
