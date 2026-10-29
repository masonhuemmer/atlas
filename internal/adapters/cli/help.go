package cli

const rootHelp = `atlas — Atlassian CLI (Jira, Confluence, Bitbucket, JSM customer REST)

Usage: atlas <namespace> <verb> [flags]

Namespaces:
  auth         sign in, status, logout (core, not a workload)
  site         list and resolve clouds from config
  jira         get, comments, search, suggest, users, boards, board, transitions, create, edit, comment, transition, link
  confluence   get, search, create, update (no delete)
  pr           get, list, comments, create, edit, comment, approve, merge, diff (no delete)
  jsm          customer REST: desks, types, list, get, create, comment, transition
  mcp          stdio MCP for agents (serve)

Output: JSON on stdout by default. --human for text. Diagnostics on stderr.
Exit classes: 0 success; 3 usage/config; 4 auth; 5 service; 6 not-found.

Every call resolves one site from config. Sites, project keys, space keys,
Bitbucket workspace, and JSM default site live in config.toml (or ATLAS_CONFIG).
A site with role jsm_customer refuses jira and confluence (use atlas jsm).
Secrets never appear in stdout, MCP text, or logs.
`

const authHelp = `atlas auth — site and Bitbucket workspace Basic email:token (keyring service atlas-cli)

Set ATLAS_CREDENTIAL_STORE=file for headless Linux without Secret Service.
The file is $XDG_STATE_HOME/atlas/session.json or ~/.local/state/atlas/session.json.

Verbs: status, login, logout
status: signed_in, session_usable, sites[], and workspaces[]. No emails or tokens.
login:  --site ALIAS --email EMAIL --token TOKEN
        --workspace WORKSPACE --email EMAIL --token TOKEN
        --site and --workspace are mutually exclusive.
        --from-op is a human terminal flag only (not via MCP).
logout: optional --site ALIAS or --workspace WORKSPACE (all credentials if omitted)
Login and logout stay terminal-only. Do not call them through MCP.
Output: JSON (default) or --human.
`

const siteHelp = `atlas site — catalog from config (no HTTP)

Verbs: list, resolve
list:    aliases loaded from ATLAS_CONFIG or ~/.config/atlas/config.toml
resolve: alias, hostname, or UUID → one Site
REST later uses hostname, not UUID.
Output: JSON (default) or --human.
`

const jiraHelp = `atlas jira — licensed Jira on one cloud

Verbs: get, comments, attachments, download, search, suggest, users, boards, board, transitions, create, edit, comment, transition, link
get:        atlas jira get KEY-1 [--fields comment,summary]
            get includes comments; use it after jira comment to verify the text persisted.
            --fields selects output fields; comment selects comments.
            --fields custom adds custom_fields: every non-empty custom field as {id,name,type,text}.
            --fields customfield_10538 adds just that field; empty or absent fields are omitted.
            --fields history adds status_history: [{created,author,from,to}] from the changelog.
            The tokens combine: --fields summary,status,custom,history. Without --fields the output is unchanged.
comments:   atlas jira comments KEY-1 [--start-at 0] [--limit 50]
            lists comment IDs, bodies, and authors; use next_start_at for another page.
attachments: atlas jira attachments KEY-1 [--site ALIAS]
download:   atlas jira download KEY-1 --attachment ID --output PATH [--site ALIAS] [--dry-run]
            output must be a new file; an existing file is never overwritten.
search:     atlas jira search --jql 'project = KEY' [--limit 50] [--page-token TOKEN]
suggest:    atlas jira suggest --query 'text' [--project KEY] [--site ALIAS]
users:      atlas jira users --query 'Alex' --project KEY
            atlas jira users --query 'alex@example.com' --issue KEY-1
            atlas jira users --query 'Alex' --site ALIAS
boards:     atlas jira boards --project KEY [--site ALIAS]
placement:  atlas jira board placement KEY-1 --board 123 [--site ALIAS]
transitions: atlas jira transitions KEY-1 [--board 123]
create:     atlas jira create --project KEY --type Task --summary '...'
create sub-task: atlas jira create --project KEY --type Sub-task --summary '...' --parent KEY-1
create fields: atlas jira create --project KEY --type Task --summary '...' --fields '{"customfield_10538":"line one\n\nline two"}' [--dry-run]
            --fields is a JSON object of REST fields merged into the new issue. project, issuetype and summary
            are usage errors there, and description, parent, labels and assignee clash with their flags.
            A string for a textarea custom field is converted from Markdown to rich text, as is a description.
            --dry-run echoes the merged payload as fields; it may read GET /field once and never writes.
            A 400 from Jira reports its field errors; the exit class stays service.
edit:       atlas jira edit SUBTASK-2 --parent KEY-1  (or --fields '{...}', converted the same way)
assign:     atlas jira edit KEY-1 --assignee ACCOUNT_ID
comment:    atlas jira comment KEY-1 --body '...' [--internal]
edit comment: atlas jira comment edit KEY-1 --id ID --body '...' [--dry-run]
            editing refuses existing rich ADF nodes that Markdown cannot preserve.
delete comment: atlas jira comment delete KEY-1 --id ID [--dry-run]
transition: atlas jira transition KEY-1 --name Done (or --id from jira transitions)
link:       atlas jira link KEY-1 OTHER-2 [--type Relates]

Project keys map to a site in config. Combined JQL that names two sites is usage.
users with --project or --issue returns accounts Jira considers assignable there.
Use the returned account_id with create or edit --assignee.
Email appears only when Jira permits it under the user's privacy settings.
Jira descriptions and public comments convert Markdown to rich text on write.
--site is required only when inference cannot run.
A jsm_customer site is not Jira search: use atlas jsm, not atlas jira search.
--internal requires an accessible JSM customer request and never falls back to a public comment.
link default type is Relates. Both keys must be the same cloud.
--parent creates or reparents a sub-task under a story in the same project.
link creates a regular issue link; it does not set the sub-task parent.
When type is Blocks, inward is the blocker and outward is the blocked issue.
MCP writes dry-run unless write_opt_in is true.

Default search fields: summary, description, status, issuetype, parent, priority,
labels, assignee, reporter, created, updated, project.
Browse URL: https://<hostname>/browse/<KEY>
Output: JSON (default) or --human.
`

const confluenceHelp = `atlas confluence — licensed Confluence on one cloud

Verbs: get, search, create, update, move
get:    atlas confluence get <pageId> --site ALIAS
search: atlas confluence search --cql 'space = KEY AND type = page'
create: atlas confluence create --space KEY --title '...' --body '...' [--parent PAGE_ID]
update: atlas confluence update <pageId> --body '...' [--site ALIAS]
move:   atlas confluence move <pageId> --parent PAGE_ID [--site ALIAS]

Space keys map to a site in config. Create/update convert Markdown to storage markup.
Use --body-format storage for prepared Confluence XHTML. Get returns storage markup.
Parent IDs are page IDs. Move places a page under another page in the same space.
CQL that names a configured space infers that site; CQL with no space and no --site is usage.
There is no delete verb.
MCP writes dry-run unless write_opt_in is true.
Live APIs: /wiki/api/v2/pages and /wiki/rest/api/content/{id}/move/append/{parentId}.
Output: JSON (default) or --human.
`

const prHelp = `atlas pr — Bitbucket Cloud REST 2.0

Verbs: get, list, comments, create, edit, comment, approve, merge, diff
get:     atlas pr get --repo SLUG --id 1
list:    atlas pr list --repo SLUG
comments: atlas pr comments --repo SLUG --id 1 [--page 1] [--limit 50]
create:  atlas pr create --repo SLUG --title '...' --source <branch> [--target main]
edit:    atlas pr edit --repo SLUG --id 1 --description '...' [--title '...']
comment: atlas pr comment --repo SLUG --id 1 --body '...'
approve: atlas pr approve --repo SLUG --id 1
merge:   atlas pr merge --repo SLUG --id 1
diff:    atlas pr diff --repo SLUG --id 1

--workspace is optional and defaults to config defaults.workspace. Edit, approve, and merge are writes.
PR operations require a separate credential stored by workspace:
atlas auth login --workspace WORKSPACE --email EMAIL --token TOKEN.
There is no fallback to a Jira or Confluence site credential.
There is no delete verb. SSH and git stay out of pr.
Reviewer updates are REST fields on create.
get includes the author and approval participants. Read comments with pr comments.
MCP writes dry-run unless write_opt_in is true.
Live base: https://api.bitbucket.org/2.0/repositories/{workspace}/{repo}/pullrequests
Output: JSON (default) or --human.
`

const jsmHelp = `atlas jsm — JSM customer REST

Verbs: desks, types, list, get, create, comment, transition
desks:      atlas jsm desks [--site ALIAS]
types:      atlas jsm types --desk ID
list:       atlas jsm list [--status open|closed|all]
get:        atlas jsm get KEY-1
create:     atlas jsm create --desk ID --type <id> --summary '...'
comment:    atlas jsm comment KEY-1 --body '...'
transition: atlas jsm transition KEY-1 --id <transition>

Missing --site uses defaults.jsm_site when set; otherwise --site is required.
jsm against a licensed site is usage. Comments are public: true only. No raiseOnBehalfOf.
Portal URL: https://<hostname>/servicedesk/customer/portal/{desk}/{KEY}
MCP writes dry-run unless write_opt_in is true.
Live base: https://<hostname>/rest/servicedeskapi
Output: JSON (default) or --human.
`
