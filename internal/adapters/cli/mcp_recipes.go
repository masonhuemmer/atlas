package cli

const (
	recipeJiraSearch = `jira-search

Resolve one site, then search or get. Never dual-query clouds.

  atlas jira get KEY-1
  atlas jira search --jql 'project = KEY'
  atlas_read namespace=jira verb=get args=["KEY-1"]

Project keys map to a site in config. Combined JQL that names two sites is usage.
A jsm_customer site is not Jira search: use atlas jsm.

Do not print tokens.
`

	recipeConfluenceWrite = `confluence-write

Space keys map to a site in config. No delete verb.
Create/update convert Markdown to storage markup. For prepared Confluence XHTML, pass body-format=storage.
Use a page ID as parent. Move places an existing page under a parent in the same space.

  atlas confluence search --cql 'space = KEY AND type = page'
  atlas confluence create --space KEY --title '…' --body '…' --dry-run
  atlas confluence create --space KEY --parent PAGE_ID --title '…' --body '…' --dry-run
  atlas confluence move PAGE_ID --parent NEW_PARENT_ID --site ALIAS --dry-run
  atlas_write namespace=confluence verb=create flags space=KEY title=… body=…

MCP writes dry-run unless write_opt_in is true.
`

	recipePRReview = `pr-review

Bitbucket Cloud REST. Workspace from config defaults.workspace. Authentication uses the separate credential stored for that workspace; it never falls back to a site credential. No PR delete. SSH stays out.

  atlas pr get --repo SLUG --id 1
  atlas pr edit --repo SLUG --id 1 --description '…' --dry-run
  atlas pr comment --repo SLUG --id 1 --body '…' --dry-run
  atlas pr merge --repo SLUG --id 1 --dry-run
  atlas_read namespace=pr verb=get flags repo=SLUG id=1

Edit and merge are writes. Reviewer updates are REST fields.
Missing workspace auth requires atlas auth login --workspace WORKSPACE in a terminal.
`

	recipeJSMCustomer = `jsm-customer

JSM customer portal REST. Never Jira search on a jsm_customer site.

  atlas jsm desks --site ALIAS
  atlas jsm list --status open
  atlas jsm comment KEY-1 --body '…' --dry-run
  atlas_read namespace=jsm verb=desks flags site=ALIAS

Comments are public: true only. Credentials are the customer keyring slot.
`
)

var recipeBodies = map[string]string{
	"jira-search":      recipeJiraSearch,
	"confluence-write": recipeConfluenceWrite,
	"pr-review":        recipePRReview,
	"jsm-customer":     recipeJSMCustomer,
}

var recipeNames = []string{"jira-search", "confluence-write", "pr-review", "jsm-customer"}

func recipe(topic string) (string, bool) {
	if topic == "atlas" {
		return skillMarkdown, true
	}
	s, ok := recipeBodies[topic]
	return s, ok
}
