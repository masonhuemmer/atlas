package jira

import (
	"context"
	"io"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// Store is licensed Jira on one hostname. Fake memory and live REST both implement it.
type Store interface {
	Get(ctx context.Context, hostname, key string) (domain.Issue, error)
	// GetCustom reads what Get leaves out: ids and all select custom fields,
	// history adds the status transitions from the changelog.
	GetCustom(ctx context.Context, hostname, key string, ids []string, all, history bool) (domain.IssueCustom, error)
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
	// CreateFields returns the `fields` object Create would send. It may read
	// the site's field schema, and never writes.
	CreateFields(ctx context.Context, hostname string, in domain.CreateIssue) (map[string]any, error)
	Edit(ctx context.Context, hostname, key string, fields map[string]any, dryRun bool) (domain.Issue, error)
	Comment(ctx context.Context, hostname, key, body string, dryRun bool) error
	CommentInternal(ctx context.Context, hostname, key, body string, dryRun bool) error
	Comments(ctx context.Context, hostname, key string, startAt, limit int) (domain.JiraCommentPage, error)
	GetComment(ctx context.Context, hostname, key, id string) (domain.JiraComment, error)
	EditComment(ctx context.Context, hostname, key, id, body string) (domain.JiraComment, error)
	DeleteComment(ctx context.Context, hostname, key, id string) error
	Transition(ctx context.Context, hostname, key, name string, dryRun bool) (domain.Issue, error)
	TransitionID(ctx context.Context, hostname, key, id string, dryRun bool) (domain.Issue, error)
	Link(ctx context.Context, hostname, inward, outward, linkType string, dryRun bool) error
}
