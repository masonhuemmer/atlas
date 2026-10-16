package jira

import (
	"context"
	"io"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// Store is licensed Jira on one hostname. Fake memory and live REST both implement it.
type Store interface {
	Get(ctx context.Context, hostname, key string) (domain.Issue, error)
	Attachments(ctx context.Context, hostname, key string) ([]domain.JiraAttachment, error)
	DownloadAttachment(ctx context.Context, hostname, id string, dst io.Writer) (int64, error)
	Search(ctx context.Context, hostname, jql string) (domain.SearchResult, error)
	SearchPage(ctx context.Context, hostname, jql string, limit int, pageToken string) (domain.SearchResult, error)
	Suggest(ctx context.Context, hostname, query, project string) (domain.IssueSuggestions, error)
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
