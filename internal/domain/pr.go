package domain

import "strconv"

// DefaultTargetBranch is used when atlas pr create omits --target.
const DefaultTargetBranch = "main"

// PullRequest is one Bitbucket Cloud pull request.
// Workspace comes from catalog defaults when --workspace is omitted.
// State is OPEN, MERGED, or DECLINED.
type PullRequest struct {
	ID          int      `json:"id"`
	Workspace   string   `json:"workspace"`
	Repo        string   `json:"repo"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Source      string   `json:"source"`
	Target      string   `json:"target"`
	State       string   `json:"state"`
	URL         string   `json:"url,omitempty"`
	Comments    []string `json:"comments,omitempty"`
	Reviewers   []string `json:"reviewers,omitempty"`
}

// PullRequestList is one-repo PR output.
type PullRequestList struct {
	Workspace string        `json:"workspace"`
	Repo      string        `json:"repo"`
	Count     int           `json:"count"`
	Items     []PullRequest `json:"items"`
}

// PullRequestDiff is the unified diff for one PR.
type PullRequestDiff struct {
	ID        int    `json:"id"`
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	Diff      string `json:"diff"`
}

// PullRequestApproval records the authenticated user's approval of a PR.
type PullRequestApproval struct {
	ID        int    `json:"id"`
	Workspace string `json:"workspace"`
	Repo      string `json:"repo"`
	URL       string `json:"url"`
	Approved  bool   `json:"approved"`
}

// CreatePullRequest is the REST field set we own for atlas pr create.
// Minimum is title + source branch. Target defaults to DefaultTargetBranch.
type CreatePullRequest struct {
	Title       string
	Source      string
	Target      string
	Description string
	Reviewers   []string
}

// EditPullRequest uses nil for an unchanged field. An empty description clears it.
type EditPullRequest struct {
	Title       *string
	Description *string
}

// PRURL is https://bitbucket.org/<workspace>/<repo>/pull-requests/<id>.
func PRURL(workspace, repo string, id int) string {
	return "https://bitbucket.org/" + workspace + "/" + repo + "/pull-requests/" + strconv.Itoa(id)
}
