package rest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func (b BitbucketPipelines) GetSchedule(ctx context.Context, workspace, repo, id string) (domain.PipelineSchedule, error) {
	var raw map[string]any
	if err := b.requestURL(ctx, workspace, http.MethodGet, b.scheduleEndpoint(workspace, repo, id), nil, &raw); err != nil {
		return domain.PipelineSchedule{}, err
	}
	if str(raw, "uuid") == "" {
		return domain.PipelineSchedule{}, domain.Service("Bitbucket did not return schedule metadata")
	}
	return scheduleFromREST(workspace, repo, raw), nil
}

func (b BitbucketPipelines) SetScheduleEnabled(ctx context.Context, workspace, repo, id string, enabled bool) (domain.PipelineSchedule, error) {
	var raw map[string]any
	if err := b.requestURL(ctx, workspace, http.MethodPut, b.scheduleEndpoint(workspace, repo, id), map[string]any{"enabled": enabled}, &raw); err != nil {
		return domain.PipelineSchedule{}, err
	}
	if str(raw, "uuid") == "" {
		return domain.PipelineSchedule{}, domain.Service("Bitbucket did not return schedule metadata")
	}
	return scheduleFromREST(workspace, repo, raw), nil
}

func (b BitbucketPipelines) DeleteSchedule(ctx context.Context, workspace, repo, id string) error {
	return b.requestURL(ctx, workspace, http.MethodDelete, b.scheduleEndpoint(workspace, repo, id), nil, nil)
}

func (b BitbucketPipelines) scheduleEndpoint(workspace, repo, id string) string {
	path := "/2.0/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo) + "/pipelines_config/schedules"
	if id != "" {
		path += "/" + url.PathEscape(id)
	}
	return joinURL(b.bitbucketOrigin(), path)
}

func (b BitbucketPipelines) ListSchedules(ctx context.Context, workspace, repo string, page, limit int) (domain.PipelineScheduleList, error) {
	query := url.Values{"page": {strconv.Itoa(page)}, "pagelen": {strconv.Itoa(limit)}}
	var response struct {
		Values []map[string]any `json:"values"`
		Next   string           `json:"next"`
	}
	if err := b.requestURL(ctx, workspace, http.MethodGet, b.scheduleEndpoint(workspace, repo, "")+"?"+query.Encode(), nil, &response); err != nil {
		return domain.PipelineScheduleList{}, err
	}
	result := domain.PipelineScheduleList{Workspace: workspace, Repo: repo, Page: page, PageLen: limit, Next: response.Next, Items: []domain.PipelineSchedule{}}
	for _, raw := range response.Values {
		result.Items = append(result.Items, scheduleFromREST(workspace, repo, raw))
	}
	result.Count = len(result.Items)
	return result, nil
}

func (b BitbucketPipelines) CreateSchedule(ctx context.Context, workspace, repo string, in domain.CreatePipelineSchedule) (domain.PipelineSchedule, error) {
	selector := map[string]any{"type": "default"}
	if in.Custom != "" {
		selector = map[string]any{"type": "custom", "pattern": in.Custom}
	}
	payload := map[string]any{"enabled": in.Enabled, "cron_pattern": in.Cron, "target": map[string]any{"ref_type": "branch", "ref_name": in.Branch, "selector": selector}}
	var raw map[string]any
	if err := b.requestURL(ctx, workspace, http.MethodPost, b.scheduleEndpoint(workspace, repo, ""), payload, &raw); err != nil {
		return domain.PipelineSchedule{}, err
	}
	if str(raw, "uuid") == "" {
		return domain.PipelineSchedule{}, domain.Service("Bitbucket did not return schedule metadata")
	}
	return scheduleFromREST(workspace, repo, raw), nil
}

func scheduleFromREST(workspace, repo string, raw map[string]any) domain.PipelineSchedule {
	target := asMap(raw["target"])
	selector := asMap(target["selector"])
	enabled, _ := raw["enabled"].(bool)
	return domain.PipelineSchedule{UUID: str(raw, "uuid"), Workspace: workspace, Repo: repo, Branch: str(target, "ref_name"), SelectorType: str(selector, "type"), SelectorPattern: str(selector, "pattern"), Cron: str(raw, "cron_pattern"), Enabled: enabled, CreatedOn: str(raw, "created_on"), UpdatedOn: str(raw, "updated_on")}
}
