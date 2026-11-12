package atlassian

import (
	"context"
	"fmt"

	"github.com/masonhuemmer/atlas/internal/domain"
)

func (p *PipelineAPI) ListVariables(_ context.Context, target domain.PipelineVariableTarget, page, limit int) (domain.PipelineVariableList, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	items := []domain.PipelineVariable{}
	for _, v := range p.variables {
		if v.PipelineVariableTarget == target {
			items = append(items, v)
		}
	}
	result := domain.PipelineVariableList{PipelineVariableTarget: target, Page: page, PageLen: limit, Items: []domain.PipelineVariable{}}
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

func (p *PipelineAPI) CreateVariable(_ context.Context, target domain.PipelineVariableTarget, in domain.WritePipelineVariable) (domain.PipelineVariable, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, v := range p.variables {
		if v.PipelineVariableTarget == target && v.Key == in.Key {
			return domain.PipelineVariable{}, domain.Usage("variable key already exists")
		}
	}
	p.nextVariable++
	v := domain.PipelineVariable{PipelineVariableTarget: target, UUID: fmt.Sprintf("{30000000-0000-0000-0000-%012d}", p.nextVariable), Key: in.Key, Secured: in.Secured}
	p.variables = append(p.variables, v)
	return v, nil
}

func (p *PipelineAPI) GetVariable(_ context.Context, target domain.PipelineVariableTarget, id string) (domain.PipelineVariable, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, v := range p.variables {
		if v.PipelineVariableTarget == target && v.UUID == id {
			return v, nil
		}
	}
	return domain.PipelineVariable{}, domain.NotFound("variable not found")
}

func (p *PipelineAPI) DeleteVariable(_ context.Context, target domain.PipelineVariableTarget, id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, v := range p.variables {
		if v.PipelineVariableTarget == target && v.UUID == id {
			p.variables = append(p.variables[:i], p.variables[i+1:]...)
			return nil
		}
	}
	return domain.NotFound("variable not found")
}

func (p *PipelineAPI) UpdateVariable(_ context.Context, target domain.PipelineVariableTarget, id string, in domain.WritePipelineVariable) (domain.PipelineVariable, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, v := range p.variables {
		if v.PipelineVariableTarget == target && v.UUID == id {
			p.variables[i].Key = in.Key
			p.variables[i].Secured = in.Secured
			return p.variables[i], nil
		}
	}
	return domain.PipelineVariable{}, domain.NotFound("variable not found")
}
