package atlassian

import (
	"context"
	"sort"
	"strconv"
	"strings"

	confluenceapp "github.com/masonhuemmer/atlas/internal/app/confluence"
	"github.com/masonhuemmer/atlas/internal/domain"
)

// ConfluenceAPI is the in-process adapter used by ATLAS_FAKE and tests.
type ConfluenceAPI struct {
	Memory *Memory
}

func (c ConfluenceAPI) Get(ctx context.Context, hostname, pageID string) (domain.Page, error) {
	if c.Memory == nil {
		return domain.Page{}, domain.Service("confluence memory not configured")
	}
	return c.Memory.GetPage(ctx, hostname, pageID)
}

func (c ConfluenceAPI) Search(ctx context.Context, hostname, cql string) (domain.PageSearchResult, error) {
	if c.Memory == nil {
		return domain.PageSearchResult{}, domain.Service("confluence memory not configured")
	}
	return c.Memory.SearchPages(ctx, hostname, cql)
}

func (c ConfluenceAPI) Create(ctx context.Context, hostname string, in domain.CreatePage, dryRun bool) (domain.Page, error) {
	if c.Memory == nil {
		return domain.Page{}, domain.Service("confluence memory not configured")
	}
	return c.Memory.CreatePage(ctx, hostname, in, dryRun)
}

func (c ConfluenceAPI) Update(ctx context.Context, hostname, pageID, body, bodyFormat string, dryRun bool) (domain.Page, error) {
	if c.Memory == nil {
		return domain.Page{}, domain.Service("confluence memory not configured")
	}
	return c.Memory.UpdatePage(ctx, hostname, pageID, body, bodyFormat, dryRun)
}

func (m *Memory) GetPage(_ context.Context, hostname, pageID string) (domain.Page, error) {
	if m == nil {
		return domain.Page{}, domain.Service("confluence memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	pageID = strings.TrimSpace(pageID)
	if hostname == "" || pageID == "" {
		return domain.Page{}, domain.Usage("hostname and page id are required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pages[memKey(hostname, pageID)]
	if !ok {
		return domain.Page{}, domain.NotFound("page not found").WithHint("check the page id and site")
	}
	return clonePage(p), nil
}

func (m *Memory) SearchPages(_ context.Context, hostname, cql string) (domain.PageSearchResult, error) {
	if m == nil {
		return domain.PageSearchResult{}, domain.Service("confluence memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return domain.PageSearchResult{}, domain.Usage("hostname is required")
	}
	spaces := domain.CQLSpaces(cql)
	titleNeedle := cqlTitleNeedle(cql)
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]domain.Page, 0)
	for _, p := range m.pages {
		if p.Site != hostname {
			continue
		}
		if len(spaces) > 0 && !containsFold(spaces, p.Space) {
			continue
		}
		if titleNeedle != "" && !strings.Contains(strings.ToLower(p.Title), strings.ToLower(titleNeedle)) {
			continue
		}
		items = append(items, clonePage(p))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return domain.PageSearchResult{CQL: cql, Site: hostname, Count: len(items), Items: items}, nil
}

func (m *Memory) CreatePage(_ context.Context, hostname string, in domain.CreatePage, dryRun bool) (domain.Page, error) {
	if m == nil {
		return domain.Page{}, domain.Service("confluence memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	space := strings.ToUpper(strings.TrimSpace(in.Space))
	title := strings.TrimSpace(in.Title)
	body := in.Body
	body, err := confluenceapp.StorageBody(body, in.BodyFormat)
	if err != nil {
		return domain.Page{}, err
	}
	if hostname == "" || space == "" || title == "" {
		return domain.Page{}, domain.Usage("space and title are required")
	}
	if _, err := m.resolveSpaceKey(space); err != nil {
		return domain.Page{}, err
	}
	preview := domain.Page{
		Site:          hostname,
		Space:         space,
		Title:         title,
		Body:          body,
		ContentFormat: domain.StorageBodyFormat,
		Status:        "current",
		Version:       1,
	}
	if dryRun {
		return preview, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	n := m.nextPage
	if n == 0 {
		n = 1
	}
	preview.ID = strconv.Itoa(n)
	preview.URL = domain.WikiPageURL(hostname, space, preview.ID)
	m.nextPage = n + 1
	m.putPageLocked(preview)
	return clonePage(preview), nil
}

func (m *Memory) UpdatePage(_ context.Context, hostname, pageID, body, bodyFormat string, dryRun bool) (domain.Page, error) {
	if m == nil {
		return domain.Page{}, domain.Service("confluence memory not configured")
	}
	hostname = strings.TrimSpace(hostname)
	pageID = strings.TrimSpace(pageID)
	if hostname == "" || pageID == "" {
		return domain.Page{}, domain.Usage("hostname and page id are required")
	}
	if strings.TrimSpace(body) == "" {
		return domain.Page{}, domain.Usage("update requires --body")
	}
	body, err := confluenceapp.StorageBody(body, bodyFormat)
	if err != nil {
		return domain.Page{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.pages[memKey(hostname, pageID)]
	if !ok {
		return domain.Page{}, domain.NotFound("page not found").WithHint("check the page id and site")
	}
	next := clonePage(p)
	next.Body = body
	next.ContentFormat = domain.StorageBodyFormat
	next.Version = p.Version + 1
	if dryRun {
		return next, nil
	}
	m.putPageLocked(next)
	return clonePage(next), nil
}

// PageCount is the seeded plus persisted page count (tests).
func (m *Memory) PageCount() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.pages)
}

func (m *Memory) resolveSpaceKey(space string) (string, error) {
	space = strings.ToUpper(strings.TrimSpace(space))
	if space == "" {
		return "", domain.Usage("space is required")
	}
	if m == nil {
		return "", domain.Service("confluence memory not configured")
	}
	if id, ok := m.spaces[space]; ok {
		return id, nil
	}
	if _, err := strconv.Atoi(space); err == nil {
		return space, nil
	}
	return "", domain.Usagef("unknown space %q", space).WithHint("use space key CCAB")
}

func clonePage(p domain.Page) domain.Page {
	return p
}

func cqlTitleNeedle(cql string) string {
	lower := strings.ToLower(cql)
	i := strings.Index(lower, "title")
	if i < 0 {
		return ""
	}
	rest := cql[i:]
	tilde := strings.Index(rest, "~")
	if tilde < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[tilde+1:])
	if rest == "" {
		return ""
	}
	quote := rest[0]
	if quote == '"' || quote == '\'' {
		end := strings.IndexRune(rest[1:], rune(quote))
		if end < 0 {
			return strings.TrimSpace(rest[1:])
		}
		return strings.TrimSpace(rest[1 : 1+end])
	}
	fields := strings.FieldsFunc(rest, func(r rune) bool {
		return r == ' ' || r == ')' || r == '&'
	})
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
