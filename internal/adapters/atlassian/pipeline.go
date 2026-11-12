package atlassian

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"sync"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// PipelineAPI is the in-process fake. Each run has one synthetic step.
type PipelineAPI struct {
	mu           sync.Mutex
	runs         []domain.Pipeline
	variables    []domain.PipelineVariable
	nextVariable int
	schedules    []domain.PipelineSchedule
	nextSchedule int
}

func SeedPipelines() *PipelineAPI {
	p := &PipelineAPI{}
	_, _ = p.Run(context.Background(), domain.DefaultWorkspace(), "atlas", domain.RunPipeline{Branch: "main"})
	return p
}

func (p *PipelineAPI) Run(_ context.Context, workspace, repo string, in domain.RunPipeline) (domain.Pipeline, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	build := 1
	for _, run := range p.runs {
		if run.Workspace == workspace && run.Repo == repo && run.BuildNumber >= build {
			build = run.BuildNumber + 1
		}
	}
	run := domain.Pipeline{
		UUID:      fmt.Sprintf("{00000000-0000-0000-0000-%012d}", len(p.runs)+1),
		Workspace: workspace, Repo: repo, BuildNumber: build, State: "PENDING",
		RefType: "branch", RefName: in.Branch, Custom: in.Custom,
	}
	p.runs = append(p.runs, run)
	return run, nil
}

func (p *PipelineAPI) Get(_ context.Context, workspace, repo, id string) (domain.Pipeline, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, run := range p.runs {
		if run.Workspace == workspace && run.Repo == repo && run.UUID == id {
			return run, nil
		}
	}
	return domain.Pipeline{}, domain.NotFound("pipeline not found")
}

func (p *PipelineAPI) List(_ context.Context, workspace, repo string, in domain.PipelineListOptions) (domain.PipelineList, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	items := []domain.Pipeline{}
	for i := len(p.runs) - 1; i >= 0; i-- {
		run := p.runs[i]
		status := run.State
		if run.State == "COMPLETED" {
			status = run.Result
			if status == "SUCCESSFUL" {
				status = "PASSED"
			}
		}
		if run.Workspace == workspace && run.Repo == repo && (in.Branch == "" || in.Branch == run.RefName) && (in.Status == "" || in.Status == status) {
			items = append(items, run)
		}
	}
	page := domain.PipelineList{Workspace: workspace, Repo: repo, Page: in.Page, PageLen: in.Limit, Items: []domain.Pipeline{}}
	start := (in.Page - 1) * in.Limit
	if start < 0 || start >= len(items) {
		return page, nil
	}
	end := min(start+in.Limit, len(items))
	page.Items = items[start:end]
	page.Count = len(page.Items)
	if end < len(items) {
		query := url.Values{"page": {strconv.Itoa(in.Page + 1)}, "pagelen": {strconv.Itoa(in.Limit)}}
		if in.Branch != "" {
			query.Set("target.ref_name", in.Branch)
		}
		if in.Status != "" {
			query.Set("status", in.Status)
		}
		page.Next = "https://api.bitbucket.org/2.0/repositories/" + url.PathEscape(workspace) + "/" + url.PathEscape(repo) + "/pipelines?" + query.Encode()
	}
	return page, nil
}

func (p *PipelineAPI) Steps(ctx context.Context, workspace, repo, id string, page, limit int) (domain.PipelineStepList, error) {
	run, err := p.Get(ctx, workspace, repo, id)
	if err != nil {
		return domain.PipelineStepList{}, err
	}
	result := domain.PipelineStepList{UUID: id, Workspace: workspace, Repo: repo, Page: page, PageLen: limit, Items: []domain.PipelineStep{}}
	if page == 1 {
		result.Items = append(result.Items, domain.PipelineStep{UUID: id, Name: "Simulated build", State: run.State, Result: run.Result})
		result.Count = 1
	}
	return result, nil
}

func (p *PipelineAPI) Logs(ctx context.Context, workspace, repo, id, step string) (domain.PipelineLog, error) {
	if _, err := p.Get(ctx, workspace, repo, id); err != nil {
		return domain.PipelineLog{}, err
	}
	if step != id {
		return domain.PipelineLog{}, domain.NotFound("step not found")
	}
	return domain.PipelineLog{UUID: id, Workspace: workspace, Repo: repo, Step: step, Log: "Simulated pipeline log (ATLAS_FAKE=1)\n"}, nil
}

func (p *PipelineAPI) Stop(_ context.Context, workspace, repo, id string) (domain.PipelineStop, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, run := range p.runs {
		if run.Workspace == workspace && run.Repo == repo && run.UUID == id {
			p.runs[i].State, p.runs[i].Result = "COMPLETED", "STOPPED"
			return domain.PipelineStop{UUID: id, Workspace: workspace, Repo: repo, StopRequested: true}, nil
		}
	}
	return domain.PipelineStop{}, domain.NotFound("pipeline not found")
}
