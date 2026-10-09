package rest

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	confluenceapp "github.com/masonhuemmer/atlas/internal/app/confluence"
	"github.com/masonhuemmer/atlas/internal/domain"
)

// Confluence is the live Cloud REST client (https://<hostname>/wiki/api/v2).
type Confluence struct {
	*Client
}

func (c Confluence) Get(ctx context.Context, hostname, pageID string) (domain.Page, error) {
	cred, err := c.credForHost(hostname)
	if err != nil {
		return domain.Page{}, err
	}
	u := joinURL(c.origin(hostname), "/wiki/api/v2/pages/"+q(pageID)+"?body-format=storage")
	code, body, err := c.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.Page{}, err
	}
	if code != http.StatusOK {
		return domain.Page{}, MapConfluenceStatus(code)
	}
	return pageFromREST(hostname, mustJSON(body)), nil
}

func (c Confluence) Search(ctx context.Context, hostname, cql string) (domain.PageSearchResult, error) {
	cred, err := c.credForHost(hostname)
	if err != nil {
		return domain.PageSearchResult{}, err
	}
	u := joinURL(c.origin(hostname), "/wiki/rest/api/content/search?cql="+q(cql)+"&limit=25&expand=body.storage,space,ancestors")
	code, body, err := c.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return domain.PageSearchResult{}, err
	}
	if code != http.StatusOK {
		return domain.PageSearchResult{}, MapConfluenceStatus(code)
	}
	m := mustJSON(body)
	var items []domain.Page
	for _, raw := range asList(m["results"]) {
		items = append(items, pageFromV1(hostname, asMap(raw)))
	}
	if items == nil {
		items = []domain.Page{}
	}
	return domain.PageSearchResult{CQL: cql, Site: hostname, Count: len(items), Items: items}, nil
}

func (c Confluence) Create(ctx context.Context, hostname string, in domain.CreatePage, dryRun bool) (domain.Page, error) {
	storageBody, err := confluenceapp.StorageBody(in.Body, in.BodyFormat)
	if err != nil {
		return domain.Page{}, err
	}
	preview := domain.Page{
		Site: hostname, Space: in.Space, ParentID: in.ParentID, Title: in.Title, Body: storageBody,
		ContentFormat: domain.StorageBodyFormat, Status: "current", Version: 1,
	}
	if dryRun {
		return preview, nil
	}
	cred, err := c.credForHost(hostname)
	if err != nil {
		return domain.Page{}, err
	}
	spaceID, err := c.lookupSpaceID(ctx, hostname, in.Space)
	if err != nil {
		return domain.Page{}, err
	}
	if in.ParentID != "" {
		parent, err := c.Get(ctx, hostname, in.ParentID)
		if err != nil {
			return domain.Page{}, err
		}
		if parent.Space != spaceID {
			return domain.Page{}, domain.Usage("parent page must be in the requested space")
		}
	}
	payload := map[string]any{
		"spaceId": spaceID,
		"status":  "current",
		"title":   in.Title,
		"body": map[string]any{
			"representation": "storage",
			"value":          storageBody,
		},
	}
	if in.ParentID != "" {
		payload["parentId"] = in.ParentID
	}
	u := joinURL(c.origin(hostname), "/wiki/api/v2/pages")
	code, body, err := c.doJSON(ctx, http.MethodPost, u, cred, nil, payload)
	if err != nil {
		return domain.Page{}, err
	}
	if code != http.StatusOK && code != http.StatusCreated {
		return domain.Page{}, MapConfluenceStatus(code)
	}
	p := pageFromREST(hostname, mustJSON(body))
	if p.Space == "" {
		p.Space = in.Space
	}
	p.Body = storageBody
	if p.ParentID == "" {
		p.ParentID = in.ParentID
	}
	return p, nil
}

func (c Confluence) Update(ctx context.Context, hostname, pageID, body, bodyFormat string, dryRun bool) (domain.Page, error) {
	storageBody, err := confluenceapp.StorageBody(body, bodyFormat)
	if err != nil {
		return domain.Page{}, err
	}
	cur, err := c.Get(ctx, hostname, pageID)
	if err != nil {
		return domain.Page{}, err
	}
	if dryRun {
		cur.Body = storageBody
		cur.Version++
		return cur, nil
	}
	cred, err := c.credForHost(hostname)
	if err != nil {
		return domain.Page{}, err
	}
	payload := map[string]any{
		"id":     pageID,
		"status": "current",
		"title":  cur.Title,
		"body": map[string]any{
			"representation": "storage",
			"value":          storageBody,
		},
		"version": map[string]any{"number": cur.Version + 1},
	}
	u := joinURL(c.origin(hostname), "/wiki/api/v2/pages/"+q(pageID))
	code, raw, err := c.doJSON(ctx, http.MethodPut, u, cred, nil, payload)
	if err != nil {
		return domain.Page{}, err
	}
	if code != http.StatusOK {
		return domain.Page{}, MapConfluenceStatus(code)
	}
	p := pageFromREST(hostname, mustJSON(raw))
	p.Body = storageBody
	if p.ParentID == "" {
		p.ParentID = cur.ParentID
	}
	return p, nil
}

func (c Confluence) Move(ctx context.Context, hostname, pageID, parentID string, dryRun bool) (domain.Page, error) {
	if pageID == "" || parentID == "" || pageID == parentID {
		return domain.Page{}, domain.Usage("move requires distinct page and parent IDs")
	}
	if dryRun {
		return domain.Page{ID: pageID, Site: hostname, ParentID: parentID}, nil
	}
	source, err := c.Get(ctx, hostname, pageID)
	if err != nil {
		return domain.Page{}, err
	}
	parent, err := c.Get(ctx, hostname, parentID)
	if err != nil {
		return domain.Page{}, err
	}
	if source.Space != parent.Space {
		return domain.Page{}, domain.Usage("page and parent must be in the same space")
	}
	if source.ParentID == parentID {
		return source, nil
	}
	cred, err := c.credForHost(hostname)
	if err != nil {
		return domain.Page{}, err
	}
	u := joinURL(c.origin(hostname), "/wiki/rest/api/content/"+q(pageID)+"/move/append/"+q(parentID))
	code, _, err := c.doJSON(ctx, http.MethodPut, u, cred, nil, nil)
	if err != nil {
		return domain.Page{}, err
	}
	if code != http.StatusOK {
		if code == http.StatusBadRequest {
			return domain.Page{}, domain.Usage("Confluence rejected the page move").WithHint("check that the parent is not a descendant of the page")
		}
		return domain.Page{}, MapConfluenceStatus(code)
	}
	moved, err := c.Get(ctx, hostname, pageID)
	if err != nil || moved.ParentID != parentID {
		return domain.Page{}, domain.Service("move was accepted but the new parent could not be verified").WithHint("check the page with atlas confluence get before retrying")
	}
	return moved, nil
}

func (c Confluence) lookupSpaceID(ctx context.Context, hostname, space string) (string, error) {
	if _, err := strconv.Atoi(strings.TrimSpace(space)); err == nil {
		return strings.TrimSpace(space), nil
	}
	cred, err := c.credForHost(hostname)
	if err != nil {
		return "", err
	}
	u := joinURL(c.origin(hostname), "/wiki/api/v2/spaces?keys="+q(strings.ToUpper(space)))
	code, body, err := c.doJSON(ctx, http.MethodGet, u, cred, nil, nil)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", MapConfluenceStatus(code)
	}
	results := asList(mustJSON(body)["results"])
	if len(results) == 0 {
		return "", domain.Usagef("unknown space %q", space)
	}
	id := str(asMap(results[0]), "id")
	if id == "" {
		return "", domain.Usagef("unknown space %q", space)
	}
	return id, nil
}

func pageFromREST(hostname string, m map[string]any) domain.Page {
	id := str(m, "id")
	space := str(m, "spaceId")
	if space == "" {
		space = str(asMap(m["space"]), "key")
	}
	body := ""
	if b := asMap(m["body"]); b != nil {
		if st := asMap(b["storage"]); st != nil {
			body = str(st, "value")
		} else if st := asMap(b["value"]); false {
			_ = st
		} else {
			body = str(b, "value")
		}
	}
	ver := 0
	if v := asMap(m["version"]); v != nil {
		if n, ok := v["number"].(float64); ok {
			ver = int(n)
		}
	}
	p := domain.Page{
		ID: id, Site: hostname, Space: space, ParentID: str(m, "parentId"), Title: str(m, "title"),
		Body: body, ContentFormat: domain.StorageBodyFormat, Status: str(m, "status"),
		URL: domain.WikiPageURL(hostname, space, id), Version: ver,
	}
	if p.Status == "" {
		p.Status = "current"
	}
	return p
}

func pageFromV1(hostname string, m map[string]any) domain.Page {
	id := str(m, "id")
	space := str(asMap(m["space"]), "key")
	parentID := str(m, "parentId")
	if ancestors := asList(m["ancestors"]); parentID == "" && len(ancestors) > 0 {
		parentID = str(asMap(ancestors[len(ancestors)-1]), "id")
	}
	body := ""
	if b := asMap(m["body"]); b != nil {
		if st := asMap(b["storage"]); st != nil {
			body = str(st, "value")
		}
	}
	return domain.Page{
		ID: id, Site: hostname, Space: space, ParentID: parentID, Title: str(m, "title"),
		Body: body, ContentFormat: domain.StorageBodyFormat, Status: str(m, "status"),
		URL: domain.WikiPageURL(hostname, space, id),
	}
}

// MapConfluenceStatus maps Confluence HTTP statuses to exit classes.
func MapConfluenceStatus(code int) error {
	return classify(code, "confluence", "page not found")
}
