package pr

import (
	"context"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// Store is Bitbucket Cloud REST 2.0 on one workspace. Fake memory and live REST both implement it.
// Workspace default comes from catalog defaults.workspace. SSH and git stay out.
type Store interface {
	Get(ctx context.Context, workspace, repo string, id int) (domain.PullRequest, error)
	List(ctx context.Context, workspace, repo string) (domain.PullRequestList, error)
	Create(ctx context.Context, workspace, repo string, in domain.CreatePullRequest, dryRun bool) (domain.PullRequest, error)
	Edit(ctx context.Context, workspace, repo string, id int, in domain.EditPullRequest, dryRun bool) (domain.PullRequest, error)
	Comment(ctx context.Context, workspace, repo string, id int, body string, dryRun bool) error
	Approve(ctx context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequestApproval, error)
	Merge(ctx context.Context, workspace, repo string, id int, dryRun bool) (domain.PullRequest, error)
	Diff(ctx context.Context, workspace, repo string, id int) (domain.PullRequestDiff, error)
}
