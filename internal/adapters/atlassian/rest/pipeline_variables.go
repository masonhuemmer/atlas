package rest

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func (b BitbucketPipelines) GetVariable(ctx context.Context, target domain.PipelineVariableTarget, id string) (domain.PipelineVariable, error) {
	// Deployment variables have no individual GET endpoint.
	if target.Scope == "deployment" {
		for page := 1; ; page++ {
			list, err := b.ListVariables(ctx, target, page, 100)
			if err != nil {
				return domain.PipelineVariable{}, err
			}
			for _, v := range list.Items {
				if v.UUID == id {
					return v, nil
				}
			}
			if list.Next == "" {
				return domain.PipelineVariable{}, domain.NotFound("pipeline variable not found")
			}
		}
	}
	endpoint, err := b.variableEndpoint(target, id)
	if err != nil {
		return domain.PipelineVariable{}, err
	}
	var raw map[string]any
	if err := b.requestURL(ctx, target.Workspace, http.MethodGet, endpoint, nil, &raw); err != nil {
		return domain.PipelineVariable{}, err
	}
	if str(raw, "uuid") == "" || str(raw, "key") == "" {
		return domain.PipelineVariable{}, domain.Service("Bitbucket did not return variable metadata")
	}
	return variableFromREST(target, raw), nil
}

func (b BitbucketPipelines) DeleteVariable(ctx context.Context, target domain.PipelineVariableTarget, id string) error {
	endpoint, err := b.variableEndpoint(target, id)
	if err != nil {
		return err
	}
	return b.requestURL(ctx, target.Workspace, http.MethodDelete, endpoint, nil, nil)
}

func (b BitbucketPipelines) CreateVariable(ctx context.Context, target domain.PipelineVariableTarget, in domain.WritePipelineVariable) (domain.PipelineVariable, error) {
	return b.writeVariable(ctx, target, "", http.MethodPost, in)
}

func (b BitbucketPipelines) UpdateVariable(ctx context.Context, target domain.PipelineVariableTarget, id string, in domain.WritePipelineVariable) (domain.PipelineVariable, error) {
	return b.writeVariable(ctx, target, id, http.MethodPut, in)
}

func (b BitbucketPipelines) writeVariable(ctx context.Context, target domain.PipelineVariableTarget, id, method string, in domain.WritePipelineVariable) (domain.PipelineVariable, error) {
	endpoint, err := b.variableEndpoint(target, id)
	if err != nil {
		return domain.PipelineVariable{}, err
	}
	payload := map[string]any{"key": in.Key, "value": in.Value, "secured": in.Secured}
	var raw map[string]any
	if err := b.requestURL(ctx, target.Workspace, method, endpoint, payload, &raw); err != nil {
		return domain.PipelineVariable{}, err
	}
	if str(raw, "uuid") == "" || str(raw, "key") == "" {
		return domain.PipelineVariable{}, domain.Service("Bitbucket did not return variable metadata")
	}
	return variableFromREST(target, raw), nil
}

func (b BitbucketPipelines) variableEndpoint(target domain.PipelineVariableTarget, id string) (string, error) {
	path := "/2.0/repositories/" + url.PathEscape(target.Workspace) + "/" + url.PathEscape(target.Repo)
	switch target.Scope {
	case "repository":
		path += "/pipelines_config/variables"
	case "workspace":
		path = "/2.0/workspaces/" + url.PathEscape(target.Workspace) + "/pipelines-config/variables"
	case "deployment":
		path += "/deployments_config/environments/" + url.PathEscape(target.Environment) + "/variables"
	default:
		return "", domain.Usage("unsupported pipeline variable scope")
	}
	if id != "" {
		path += "/" + url.PathEscape(id)
	}
	return joinURL(b.bitbucketOrigin(), path), nil
}

func (b BitbucketPipelines) ListVariables(ctx context.Context, target domain.PipelineVariableTarget, page, limit int) (domain.PipelineVariableList, error) {
	endpoint, err := b.variableEndpoint(target, "")
	if err != nil {
		return domain.PipelineVariableList{}, err
	}
	query := url.Values{"page": {strconv.Itoa(page)}, "pagelen": {strconv.Itoa(limit)}}
	var response struct {
		Values []map[string]any `json:"values"`
		Next   string           `json:"next"`
	}
	if err := b.requestURL(ctx, target.Workspace, http.MethodGet, endpoint+"?"+query.Encode(), nil, &response); err != nil {
		return domain.PipelineVariableList{}, err
	}
	result := domain.PipelineVariableList{PipelineVariableTarget: target, Page: page, PageLen: limit, Next: response.Next, Items: []domain.PipelineVariable{}}
	for _, raw := range response.Values {
		result.Items = append(result.Items, variableFromREST(target, raw))
	}
	result.Count = len(result.Items)
	return result, nil
}

func variableFromREST(target domain.PipelineVariableTarget, raw map[string]any) domain.PipelineVariable {
	secured, _ := raw["secured"].(bool)
	return domain.PipelineVariable{PipelineVariableTarget: target, UUID: str(raw, "uuid"), Key: str(raw, "key"), Secured: secured}
}
