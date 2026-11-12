package pipeline

import (
	"context"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// Store manages Bitbucket Cloud runs using the selected workspace credential.
type Store interface {
	List(context.Context, string, string, domain.PipelineListOptions) (domain.PipelineList, error)
	Get(context.Context, string, string, string) (domain.Pipeline, error)
	Steps(context.Context, string, string, string, int, int) (domain.PipelineStepList, error)
	Logs(context.Context, string, string, string, string) (domain.PipelineLog, error)
	Run(context.Context, string, string, domain.RunPipeline) (domain.Pipeline, error)
	Stop(context.Context, string, string, string) (domain.PipelineStop, error)
	ListVariables(context.Context, domain.PipelineVariableTarget, int, int) (domain.PipelineVariableList, error)
	CreateVariable(context.Context, domain.PipelineVariableTarget, domain.WritePipelineVariable) (domain.PipelineVariable, error)
	UpdateVariable(context.Context, domain.PipelineVariableTarget, string, domain.WritePipelineVariable) (domain.PipelineVariable, error)
	GetVariable(context.Context, domain.PipelineVariableTarget, string) (domain.PipelineVariable, error)
	DeleteVariable(context.Context, domain.PipelineVariableTarget, string) error
	ListSchedules(context.Context, string, string, int, int) (domain.PipelineScheduleList, error)
	CreateSchedule(context.Context, string, string, domain.CreatePipelineSchedule) (domain.PipelineSchedule, error)
	GetSchedule(context.Context, string, string, string) (domain.PipelineSchedule, error)
	SetScheduleEnabled(context.Context, string, string, string, bool) (domain.PipelineSchedule, error)
	DeleteSchedule(context.Context, string, string, string) error
}
