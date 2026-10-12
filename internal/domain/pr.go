package domain

import "strconv"

// DefaultTargetBranch is used when atlas pr create omits --target.
const DefaultTargetBranch = "main"

// PullRequest is one Bitbucket Cloud pull request.
// Workspace comes from catalog defaults when --workspace is omitted.
// State is OPEN, MERGED, or DECLINED.
type PullRequest struct {
	ID           int             `json:"id"`
	Workspace    string          `json:"workspace"`
	Repo         string          `json:"repo"`
	Title        string          `json:"title"`
	Description  string          `json:"description,omitempty"`
	Source       string          `json:"source"`
	Target       string          `json:"target"`
	State        string          `json:"state"`
	URL          string          `json:"url,omitempty"`
	Comments     []string        `json:"comments,omitempty"`
	Reviewers    []string        `json:"reviewers,omitempty"`
	Author       *PRUser         `json:"author,omitempty"`
	Participants []PRParticipant `json:"participants,omitempty"`
}

// PRUser identifies a Bitbucket Cloud user without relying on a display name.
type PRUser struct {
	UUID        string `json:"uuid,omitempty"`
	AccountID   string `json:"account_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

// PRParticipant records a user's approval state on a pull request.
type PRParticipant struct {
	User     PRUser `json:"user"`
	Approved bool   `json:"approved"`
	Role     string `json:"role,omitempty"`
}

// PullRequestComment is one global, inline, or reply comment.
type PullRequestComment struct {
	ID     int     `json:"id"`
	Body   string  `json:"body"`
	Author *PRUser `json:"author,omitempty"`
}

// PullRequestCommentPage is one page of PR comments.
type PullRequestCommentPage struct {
	ID        int                  `json:"id"`
	Workspace string               `json:"workspace"`
	Repo      string               `json:"repo"`
	Comments  []PullRequestComment `json:"comments"`
	NextPage  int                  `json:"next_page,omitempty"`
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
