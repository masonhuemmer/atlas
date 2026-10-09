package confluence

import (
	"context"

	"github.com/masonhuemmer/atlas/internal/domain"
)

// Store is licensed Confluence on one hostname. Fake memory and live REST both implement it.
type Store interface {
	Get(ctx context.Context, hostname, pageID string) (domain.Page, error)
	Search(ctx context.Context, hostname, cql string) (domain.PageSearchResult, error)
	Create(ctx context.Context, hostname string, in domain.CreatePage, dryRun bool) (domain.Page, error)
	Update(ctx context.Context, hostname, pageID, body, bodyFormat string, dryRun bool) (domain.Page, error)
	Move(ctx context.Context, hostname, pageID, parentID string, dryRun bool) (domain.Page, error)
}
