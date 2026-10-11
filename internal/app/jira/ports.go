package jira

import (
	"context"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// Store is licensed Jira on one hostname. Fake memory and live REST both implement it.
type Store interface {
	Get(ctx context.Context, hostname, key string) (domain.Issue, error)
	Search(ctx context.Context, hostname, jql string) (domain.SearchResult, error)
	SearchUsers(ctx context.Context, hostname, query, project, issue string) (domain.UserSearchResult, error)
	Boards(ctx context.Context, hostname, project string) ([]domain.Board, error)
	Placement(ctx context.Context, hostname, key string, boardID int) (domain.BoardPlacement, error)
	Transitions(ctx context.Context, hostname, key string, boardID int) (domain.JiraTransitions, error)
	Create(ctx context.Context, hostname string, in domain.CreateIssue, dryRun bool) (domain.Issue, error)
	Edit(ctx context.Context, hostname, key string, fields map[string]any, dryRun bool) (domain.Issue, error)
	Comment(ctx context.Context, hostname, key, body string, dryRun bool) error
	CommentInternal(ctx context.Context, hostname, key, body string, dryRun bool) error
	Transition(ctx context.Context, hostname, key, name string, dryRun bool) (domain.Issue, error)
	TransitionID(ctx context.Context, hostname, key, id string, dryRun bool) (domain.Issue, error)
	Link(ctx context.Context, hostname, inward, outward, linkType string, dryRun bool) error
}
