package domain

// DefaultLinkType is the Jira issue-link type atlas jira link uses when --type is omitted.
const DefaultLinkType = "Relates"

// IssueLink is one directed issue link on a single cloud.
// For type Blocks, inward is the blocker and outward is the blocked issue.
type IssueLink struct {
	Type    string `json:"type"`
	Inward  string `json:"inward"`
	Outward string `json:"outward"`
}

// Issue is one licensed Jira issue on a single cloud.
// Site is the hostname. BrowseURL is https://<hostname>/browse/<KEY>.
type Issue struct {
	Key         string      `json:"key"`
	Site        string      `json:"site"`
	BrowseURL   string      `json:"browse_url"`
	Summary     string      `json:"summary"`
	Description string      `json:"description,omitempty"`
	Status      string      `json:"status"`
	StatusID    string      `json:"status_id,omitempty"`
	IssueType   string      `json:"issuetype"`
	Parent      string      `json:"parent,omitempty"`
	Priority    string      `json:"priority,omitempty"`
	Labels      []string    `json:"labels,omitempty"`
	Assignee    string      `json:"assignee,omitempty"`
	Reporter    string      `json:"reporter,omitempty"`
	Created     string      `json:"created,omitempty"`
	Updated     string      `json:"updated,omitempty"`
	Project     string      `json:"project"`
	Comments    []string    `json:"comments,omitempty"`
	Links       []IssueLink `json:"links,omitempty"`
}

// SearchResult is one-site JQL output. Items use the default search fields.
type SearchResult struct {
	JQL   string  `json:"jql"`
	Site  string  `json:"site"`
	Count int     `json:"count"`
	Items []Issue `json:"items"`
}

// CreateIssue is the REST field set we own for atlas jira create.
// Markdown is the default description format. Assignee uses catalog defaults when empty.
type CreateIssue struct {
	Project     string
	IssueType   string
	Parent      string
	Summary     string
	Description string
	Labels      []string
	Assignee    string
}

func BrowseURL(hostname, key string) string {
	return "https://" + hostname + "/browse/" + key
}
