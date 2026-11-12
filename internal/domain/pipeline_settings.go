package domain

// PipelineVariableTarget identifies the settings scope, independently of a run.
type PipelineVariableTarget struct {
	Scope       string `json:"scope"`
	Workspace   string `json:"workspace"`
	Repo        string `json:"repo,omitempty"`
	Environment string `json:"environment,omitempty"`
}

// PipelineVariable intentionally omits values, including unsecured values.
type PipelineVariable struct {
	PipelineVariableTarget
	UUID    string `json:"uuid"`
	Key     string `json:"key"`
	Secured bool   `json:"secured"`
}

type PipelineVariableList struct {
	PipelineVariableTarget
	Count   int                `json:"count"`
	Page    int                `json:"page"`
	PageLen int                `json:"pagelen"`
	Next    string             `json:"next,omitempty"`
	Items   []PipelineVariable `json:"items"`
}

// WritePipelineVariable supplies a complete replacement. Value is input only.
type WritePipelineVariable struct {
	Key     string
	Value   string `json:"-"`
	Secured bool
}

type PipelineSchedule struct {
	UUID            string `json:"uuid"`
	Workspace       string `json:"workspace"`
	Repo            string `json:"repo"`
	Branch          string `json:"branch"`
	SelectorType    string `json:"selector_type"`
	SelectorPattern string `json:"selector_pattern,omitempty"`
	Cron            string `json:"cron"`
	Enabled         bool   `json:"enabled"`
	CreatedOn       string `json:"created_on,omitempty"`
	UpdatedOn       string `json:"updated_on,omitempty"`
}

type PipelineScheduleList struct {
	Workspace string             `json:"workspace"`
	Repo      string             `json:"repo"`
	Count     int                `json:"count"`
	Page      int                `json:"page"`
	PageLen   int                `json:"pagelen"`
	Next      string             `json:"next,omitempty"`
	Items     []PipelineSchedule `json:"items"`
}

type CreatePipelineSchedule struct {
	Branch  string
	Custom  string
	Cron    string
	Enabled bool
}
