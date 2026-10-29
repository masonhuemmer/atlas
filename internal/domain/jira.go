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

// JiraAttachment is metadata for a file attached to an issue.
type JiraAttachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	MIMEType string `json:"mime_type,omitempty"`
	Created  string `json:"created,omitempty"`
}

// JiraComment identifies one comment on a licensed Jira issue.
type JiraComment struct {
	ID                 string `json:"id"`
	Body               string `json:"body"`
	UnsupportedContent string `json:"unsupported_content,omitempty"`
	Author             string `json:"author,omitempty"`
	AuthorAccountID    string `json:"author_account_id,omitempty"`
	Created            string `json:"created,omitempty"`
	Updated            string `json:"updated,omitempty"`
}

type JiraCommentPage struct {
	Key         string        `json:"key"`
	Site        string        `json:"site"`
	Total       int           `json:"total"`
	StartAt     int           `json:"start_at"`
	Limit       int           `json:"limit"`
	Comments    []JiraComment `json:"comments"`
	NextStartAt int           `json:"next_start_at,omitempty"`
	IsLast      bool          `json:"is_last"`
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

// CustomField is one non-empty custom field of an issue. Type is the last
// segment of the field's schema.custom (textarea, userpicker, ...) or the
// schema type. Text is the field value flattened to plain text.
type CustomField struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

// StatusChange is one status transition from the issue changelog.
type StatusChange struct {
	Created string `json:"created,omitempty"`
	Author  string `json:"author,omitempty"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
}

// IssueCustom is what jira get --fields adds to an issue: custom fields and
// status history. Both stay empty when the issue has none.
type IssueCustom struct {
	CustomFields  []CustomField  `json:"custom_fields,omitempty"`
	StatusHistory []StatusChange `json:"status_history,omitempty"`
}

// SearchResult is one-site JQL output. Items use the default search fields.
type SearchResult struct {
	JQL           string  `json:"jql"`
	Site          string  `json:"site"`
	Count         int     `json:"count"`
	Items         []Issue `json:"items"`
	NextPageToken string  `json:"next_page_token,omitempty"`
	IsLast        bool    `json:"is_last"`
}

// IssueSuggestions are Jira's issue-picker matches for interactive search.
type IssueSuggestions struct {
	Site    string            `json:"site"`
	Query   string            `json:"query"`
	Project string            `json:"project,omitempty"`
	Items   []IssueSuggestion `json:"items"`
}

type IssueSuggestion struct {
	Key     string `json:"key"`
	Summary string `json:"summary"`
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
	// Fields are extra REST fields from --fields, merged into the payload.
	Fields map[string]any
}

func BrowseURL(hostname, key string) string {
	return "https://" + hostname + "/browse/" + key
}
