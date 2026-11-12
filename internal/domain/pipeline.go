package domain

// Pipeline is a Bitbucket Cloud run. State describes progress; Result is set
// when the run completes (for example SUCCESSFUL, FAILED, or STOPPED).
type Pipeline struct {
	UUID        string `json:"uuid"`
	Workspace   string `json:"workspace"`
	Repo        string `json:"repo"`
	BuildNumber int    `json:"build_number"`
	State       string `json:"state"`
	Result      string `json:"result,omitempty"`
	RefType     string `json:"ref_type,omitempty"`
	RefName     string `json:"ref_name,omitempty"`
	Commit      string `json:"commit,omitempty"`
	Custom      string `json:"custom,omitempty"`
	CreatedOn   string `json:"created_on,omitempty"`
	CompletedOn string `json:"completed_on,omitempty"`
	URL         string `json:"url,omitempty"`
}

type PipelineList struct {
	Workspace string     `json:"workspace"`
	Repo      string     `json:"repo"`
	Count     int        `json:"count"`
	Page      int        `json:"page"`
	PageLen   int        `json:"pagelen"`
	Next      string     `json:"next,omitempty"`
	Items     []Pipeline `json:"items"`
}

type PipelineListOptions struct {
	Branch string
	Status string
	Page   int
	Limit  int
}

type PipelineStep struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name,omitempty"`
	State       string `json:"state"`
	Result      string `json:"result,omitempty"`
	StartedOn   string `json:"started_on,omitempty"`
	CompletedOn string `json:"completed_on,omitempty"`
}

type PipelineStepList struct {
	UUID      string         `json:"uuid"`
	Workspace string         `json:"workspace"`
	Repo      string         `json:"repo"`
	Count     int            `json:"count"`
	Page      int            `json:"page"`
	PageLen   int            `json:"pagelen"`
	Next      string         `json:"next,omitempty"`
	Items     []PipelineStep `json:"items"`
}

type PipelineLog struct {
	UUID      string `json:"uuid"`
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	Step      string `json:"step"`
	Log       string `json:"log"`
}

// RunPipeline selects an existing branch definition, optionally a custom one.
type RunPipeline struct {
	Branch string
	Custom string
}

// PipelineStop acknowledges a stop request; it does not claim completion.
type PipelineStop struct {
	UUID          string `json:"uuid"`
	Workspace     string `json:"workspace"`
	Repo          string `json:"repo"`
	StopRequested bool   `json:"stop_requested"`
}
