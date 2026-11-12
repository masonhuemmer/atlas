package atlassian

import (
	"context"
	"fmt"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func (p *PipelineAPI) GetSchedule(_ context.Context, workspace, repo, id string) (domain.PipelineSchedule, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.schedules {
		if s.Workspace == workspace && s.Repo == repo && s.UUID == id {
			return s, nil
		}
	}
	return domain.PipelineSchedule{}, domain.NotFound("schedule not found")
}

func (p *PipelineAPI) SetScheduleEnabled(_ context.Context, workspace, repo, id string, enabled bool) (domain.PipelineSchedule, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, s := range p.schedules {
		if s.Workspace == workspace && s.Repo == repo && s.UUID == id {
			p.schedules[i].Enabled = enabled
			return p.schedules[i], nil
		}
	}
	return domain.PipelineSchedule{}, domain.NotFound("schedule not found")
}

func (p *PipelineAPI) DeleteSchedule(_ context.Context, workspace, repo, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, s := range p.schedules {
		if s.Workspace == workspace && s.Repo == repo && s.UUID == id {
			p.schedules = append(p.schedules[:i], p.schedules[i+1:]...)
			return nil
		}
	}
	return domain.NotFound("schedule not found")
}

func (p *PipelineAPI) ListSchedules(_ context.Context, workspace, repo string, page, limit int) (domain.PipelineScheduleList, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	items := []domain.PipelineSchedule{}
	for _, s := range p.schedules {
		if s.Workspace == workspace && s.Repo == repo {
			items = append(items, s)
		}
	}
	result := domain.PipelineScheduleList{Workspace: workspace, Repo: repo, Page: page, PageLen: limit, Items: []domain.PipelineSchedule{}}
	start := (page - 1) * limit
	if start >= 0 && start < len(items) {
		end := min(start+limit, len(items))
		result.Items = items[start:end]
		result.Count = len(result.Items)
		if end < len(items) {
			result.Next = fmt.Sprintf("?page=%d", page+1)
		}
	}
	return result, nil
}

func (p *PipelineAPI) CreateSchedule(_ context.Context, workspace, repo string, in domain.CreatePipelineSchedule) (domain.PipelineSchedule, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextSchedule++
	s := domain.PipelineSchedule{UUID: fmt.Sprintf("{50000000-0000-0000-0000-%012d}", p.nextSchedule), Workspace: workspace, Repo: repo, Branch: in.Branch, SelectorType: "default", Cron: in.Cron, Enabled: in.Enabled}
	if in.Custom != "" {
		s.SelectorType = "custom"
		s.SelectorPattern = in.Custom
	}
	p.schedules = append(p.schedules, s)
	return s, nil
}
