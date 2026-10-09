package domain

// DefaultBodyFormat is the input format for Confluence create/update.
const DefaultBodyFormat = "markdown"

// StorageBodyFormat is the format Confluence returns and stores.
const StorageBodyFormat = "storage"

// Page is one Confluence page on a single cloud.
// Site is the hostname. URL is the wiki page link.
type Page struct {
	ID            string `json:"id"`
	Site          string `json:"site"`
	Space         string `json:"space"`
	ParentID      string `json:"parent_id,omitempty"`
	Title         string `json:"title"`
	Body          string `json:"body,omitempty"`
	ContentFormat string `json:"content_format,omitempty"`
	Status        string `json:"status,omitempty"`
	URL           string `json:"url,omitempty"`
	Version       int    `json:"version,omitempty"`
}

// PageSearchResult is one-site CQL output.
type PageSearchResult struct {
	CQL   string `json:"cql"`
	Site  string `json:"site"`
	Count int    `json:"count"`
	Items []Page `json:"items"`
}

// CreatePage is the REST field set we own for atlas confluence create.
// BodyFormat defaults to markdown. Space accepts a space key or numeric id.
type CreatePage struct {
	Space      string
	ParentID   string
	Title      string
	Body       string
	BodyFormat string
}

// WikiPageURL is https://<hostname>/wiki/spaces/<SPACE>/pages/<id>.
func WikiPageURL(hostname, space, id string) string {
	return "https://" + hostname + "/wiki/spaces/" + space + "/pages/" + id
}
