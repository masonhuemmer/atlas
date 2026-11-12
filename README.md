# atlas

Atlassian CLI for Jira, Confluence, Bitbucket, and JSM customer REST. JSON on stdout by default; `--human` for text.

## Install

From source (Go 1.25+):

```sh
make install
```

Homebrew:

```sh
brew tap masonhuemmer/atlas https://github.com/masonhuemmer/atlas
brew install masonhuemmer/atlas/atlas
```

Upgrade with `brew upgrade masonhuemmer/atlas/atlas`. For unreleased `main`, `brew install --HEAD masonhuemmer/atlas/atlas` and later `brew upgrade --fetch-HEAD masonhuemmer/atlas/atlas`.

Chocolatey (64-bit Windows, once the package is approved in the community repository):

```powershell
choco install atlas-atlassian
```

The Chocolatey package contains `atlas.exe` and adds it to `PATH` through a Chocolatey shim. Until community approval, download the `chocolatey-package` artifact from a GitHub Actions run and install its `.nupkg` from the containing directory with `choco install atlas-atlassian --source .`.

## Config

Write `$XDG_CONFIG_HOME/atlas/config.toml` (or `~/.config/atlas/config.toml`), or set `ATLAS_CONFIG` to a toml/json file. Sites, project keys, space keys, Bitbucket workspace, and the default JSM site live there. The binary does not ship a tenant table.

```toml
[defaults]
workspace = "WORKSPACE"
jsm_site = "ALIAS"

[[sites]]
alias = "ALIAS"
hostname = "example.atlassian.net"
uuid = "00000000-0000-0000-0000-000000000000"
role = "licensed"          # or jsm_customer

[projects]
KEY = "ALIAS"

[spaces]
SPACE = "ALIAS"
```

Missing config without `ATLAS_FAKE=1` is usage (exit 3).

## Auth

Per-site Basic `email:token` in the OS keyring (file fallback for oversized credentials). Login stays in a terminal. Jira, Confluence, and JSM use `--site`. Bitbucket PRs and Pipelines use a **separate** workspace token; they do not reuse a licensed-site or JSM slot.

```sh
atlas auth login --site ALIAS --email EMAIL --token TOKEN
atlas auth login --workspace WORKSPACE --email EMAIL --token TOKEN
atlas auth status
```

`--site` and `--workspace` are mutually exclusive. `--from-op` is a human `login` flag only. Do not call `auth login` through MCP. Tokens never go in `config.toml`.

On headless Linux without a D-Bus Secret Service, set `ATLAS_CREDENTIAL_STORE=file` for both login and later commands. This explicitly stores credentials as JSON in `$XDG_STATE_HOME/atlas/session.json`, or `~/.local/state/atlas/session.json` when `XDG_STATE_HOME` is unset. Atlas creates the directory with mode `0700` and the file with mode `0600`. The default is the OS keyring; switching from the keyring to file mode requires signing in again. Set `ATLAS_CREDENTIAL_STORE=keyring` to select the default explicitly.

### Bitbucket API token

`atlas pr` talks to `https://api.bitbucket.org/2.0` with the cred stored for `--workspace` (else `[defaults].workspace`). A classic unscoped Atlassian API token is not enough. Create an API token **with scopes**, app **Bitbucket**:

Read:

- `read:user:bitbucket`
- `read:workspace:bitbucket`
- `read:project:bitbucket`
- `read:repository:bitbucket`
- `read:pullrequest:bitbucket`
- `read:issue:bitbucket`

Write:

- `write:repository:bitbucket`
- `write:pullrequest:bitbucket`
- `write:issue:bitbucket`

`pr get`, `list`, `comments`, and `diff` need the read pullrequest and repository scopes. `pr create`, `edit`, `comment`, `approve`, and `merge` also need `write:pullrequest:bitbucket`. Missing workspace cred is auth (exit 4); hint is `atlas auth login --workspace WORKSPACE`. There is no fallback to a Jira site token.

Pipeline reads need `read:pipeline:bitbucket`. Starting runs needs both `read:pipeline:bitbucket` and `write:pipeline:bitbucket`; stopping needs `write:pipeline:bitbucket`. Add these scopes to the workspace token as needed. See the [Bitbucket Pipelines REST API](https://developer.atlassian.com/cloud/bitbucket/rest/api-group-pipelines/) for endpoint permissions.

Reading variables and schedules also needs `read:pipeline:bitbucket`. Variable creation, replacement, and deletion need `admin:pipeline:bitbucket`. Schedule creation and enabling/disabling need both read and write pipeline scopes; schedule deletion needs the write scope. The token's user must also have access to the selected repository, workspace, or deployment environment.

## Usage

```
atlas <namespace> <verb> [flags]
```

| Namespace | Verbs |
| --- | --- |
| `auth` | `login`, `status`, `logout` |
| `site` | `list`, `resolve` |
| `jira` | `get`, `comments`, `attachments`, `download`, `search`, `suggest`, `users`, `boards`, `board placement`, `transitions`, `create`, `edit`, `comment`, `transition`, `link` |
| `confluence` | `get`, `search`, `create`, `update`, `move` |
| `pr` | `get`, `list`, `comments`, `create`, `edit`, `comment`, `approve`, `merge`, `diff` |
| `pipeline` | `list`, `get`, `steps`, `logs`, `run`, `stop`, `variable-list/get/create/update/delete`, `schedule-list/get/create/update/delete` |
| `jsm` | `desks`, `types`, `list`, `get`, `create`, `comment`, `transition` |
| `mcp` | `serve` |

Every call resolves one site from config. A `jsm_customer` site refuses `jira` and `confluence` (use `atlas jsm`). `atlas pr` uses the workspace Bitbucket cred, not a site token. Exit classes: `0` success, `3` usage/config, `4` auth, `5` service, `6` not-found.

Confluence create and update convert Markdown bodies to Confluence storage markup. Use `--body-format storage` for prepared Confluence XHTML; get returns storage markup.
Use `--parent PAGE_ID` to create a child page, or `atlas confluence move PAGE_ID --parent NEW_PARENT_ID --site ALIAS` to move a page under another page in the same space.

For an internal note on a JSM customer request at a licensed site, use `atlas jira comment KEY-1 --body '...' --internal`. Atlas checks that the request is accessible, posts with internal visibility, and verifies the response. It never falls back to a public comment. `--dry-run` previews the command without checking site access.

To change an existing Jira comment, run `atlas jira comments CAB-119` to find its ID, then `atlas jira comment edit CAB-119 --id ID --body 'replacement text'`. To remove it, use `atlas jira comment delete CAB-119 --id ID`. Both commands support `--dry-run`; deletion previews the comment before removing it. Jira requires Edit Own/All Comments or Delete Own/All Comments permission, respectively. The edit changes the body only.
Atlas refuses to edit a comment when it detects unsupported rich Jira nodes or marks, such as mentions and emoji; edit such a comment in Jira instead.

List an issue's files with `atlas jira attachments KEY-1`. Download one with `atlas jira download KEY-1 --attachment ID --output ./filename`. The attachment ID must belong to that issue. The output path must be a new file; Atlas will not overwrite an existing file. `--dry-run` previews the download without creating a file.

Find an account ID before assigning a ticket with `atlas jira users --query 'Alex' --project SDO` or `atlas jira users --query 'Alex' --issue SDO-588`. The scoped forms return users Jira considers assignable to that project or issue. Use `--site ALIAS` without a scope for a general lookup. Results include `account_id`, display name, and email when Jira permits it. Pass the ID to `jira create --assignee ID` or `jira edit KEY-1 --assignee ID`.

For an interactive issue picker, use `atlas jira suggest --query 'provision' --project SDO` to get Jira's key and summary suggestions. `atlas jira search --jql 'project = SDO ORDER BY updated DESC' --limit 50` returns `next_page_token` when another page is available; pass it with `--page-token TOKEN` to continue. The search limit is 1–100 per page.

Find a board with `atlas jira boards --project SDO`, then run `atlas jira board placement SDO-588 --board 423` for its exact column. Placement reports `in_column`, `not_on_board`, or `unmapped_status` and keeps the underlying Jira status separate. `atlas jira transitions SDO-588 --board 423` lists available transition IDs with destination column previews; use `atlas jira transition SDO-588 --id ID` to select one unambiguously.

Create a sub-task under a story with `atlas jira create --project KEY --type Sub-task --summary '...' --parent KEY-1`. Reparent an existing sub-task with `atlas jira edit KEY-2 --parent KEY-1`. Both issues must be in the same project. Jira must allow the chosen issue type and parent. `atlas jira get KEY-2` shows the `parent` key. For a regular issue link without changing the parent, use `atlas jira link KEY-1 KEY-2 [--type Relates]`.

Read custom fields and status history with `--fields`: `atlas jira get KEY-1 --fields custom,history`. `custom` returns every non-empty custom field, `customfield_NNNNN` returns that one field, and `history` returns the status transitions from the changelog. They combine with the issue fields (`--fields summary,status,custom,history`) and the default output is unchanged without `--fields`. The output gains `custom_fields` and `status_history`, and both keys are left out when empty:

```json
{
  "key": "SDO-1",
  "custom_fields": [
    {"id": "customfield_10481", "name": "Peer Reviewer", "type": "userpicker", "text": "Avery Example"}
  ],
  "status_history": [
    {"created": "2026-09-01T09:30:00.000+0000", "author": "Alex Example", "from": "To Do", "to": "In Progress"}
  ]
}
```

`type` is the last segment of the field's `schema.custom` (`textarea`, `userpicker`, `select`, ...), or the schema type. `text` is the value flattened to plain text: rich-text fields get a newline between blocks, users show their display name, options their value (a cascading select as `parent / child`), and lists are joined with `; `. Custom fields come from `GET /rest/api/3/field` plus one issue read, and history pages through the whole changelog. A response cut off by the 1 MiB read limit is a `service` error, never an empty result.

Set custom fields when creating or editing with `--fields`, a JSON object of REST fields: `atlas jira create --project SDO --type Task --summary '...' --fields '{"customfield_10538":"line one\n\nline two","customfield_10481":{"accountId":"ACCOUNT_ID"}}'`. On create the object is merged into the issue; `project`, `issuetype` and `summary` in it are usage errors, and `description`, `parent`, `labels` and `assignee` cannot also be given as flags. Jira REST v3 rejects a plain string for a multi-line (`textarea`) custom field, so Atlas looks up the field's schema (one `GET /rest/api/3/field`, only when `--fields` has a string under a `customfield_` key) and converts that string from Markdown to Jira's rich-text format, the same way as a description. Strings for other field types, and objects and lists, are sent as given. `--dry-run` on create echoes the merged payload as `fields`, so you can check the conversion; it may do that one read and never writes. When Jira rejects a create or edit with a 400, the error message includes Jira's per-field text (at most about 500 characters) and the exit class stays `service` (5). Through MCP, pass `fields` as a JSON string.

Jira issue descriptions and public comments accept Markdown. Atlas converts headings, lists, emphasis, links, code, and tables to Jira's rich-text document format when writing. Existing descriptions stored as literal Markdown need to be edited once to render with formatting.

### Bitbucket Pipelines

Manage runs of definitions already in the repository's `bitbucket-pipelines.yml`:

```sh
atlas pipeline list --repo SLUG --branch main --status FAILED --page 1 --limit 20
atlas pipeline get --repo SLUG --id '{PIPELINE_UUID}'
atlas pipeline steps --repo SLUG --id '{PIPELINE_UUID}'
atlas pipeline logs --repo SLUG --id '{PIPELINE_UUID}' --step '{STEP_UUID}' --human
atlas pipeline run --repo SLUG --branch main --dry-run
atlas pipeline run --repo SLUG --branch main --custom 'Deploy preview' --dry-run
atlas pipeline stop --repo SLUG --id '{PIPELINE_UUID}' --dry-run
```

All commands accept `--workspace WORKSPACE`, defaulting to `[defaults].workspace`. Use the `uuid` from `list` for `--id` and the step `uuid` from `steps` for `--step`; braces are optional. Runs require an explicit `--branch`. Omit `--custom` to use the branch's normal pipeline definition.

`list` and `steps` return one page, with `count`, `page`, `pagelen`, `items`, and a `next` link when another page exists. Increase `--page` to fetch the next page; `--limit` accepts 1–100. List status filters are `PARSING`, `PENDING`, `PAUSED`, `HALTED`, `BUILDING`, `ERROR`, `PASSED`, `FAILED`, `STOPPED`, or `UNKNOWN` (case insensitive). The run output separates its progress `state` from its final `result`; for example, `COMPLETED` / `SUCCESSFUL` corresponds to the `PASSED` filter.

Logs are returned in the JSON `log` field; `--human` prints plain text. A failed or interrupted download returns an error rather than a partial log. CLI `run` and `stop` execute when `--dry-run` is omitted. A successful stop returns `stop_requested: true`; use `get` to check the final state.

MCP uses `atlas_read` with `namespace=pipeline` for `list`, `get`, `steps`, and `logs`. Use `atlas_write` for `run` and `stop`; these remain dry-run previews until `write_opt_in=true`. `atlas_help` with `namespace=pipeline` lists the flags. Per-run variable overrides and editing pipeline definitions are outside this command set.

#### Stored variables

`variable-list`, `variable-get`, `variable-create`, `variable-update`, and `variable-delete` support three scopes:

| Scope | Required flags |
| --- | --- |
| `repository` (default) | `--repo SLUG` |
| `workspace` | `--scope workspace`; omit `--repo` and `--environment` |
| `deployment` | `--scope deployment --repo SLUG --environment UUID` |

```sh
atlas pipeline variable-list --repo SLUG
atlas pipeline variable-get --repo SLUG --id '{VARIABLE_UUID}'
atlas pipeline variable-create --repo SLUG --key API_KEY --value-file /path/to/value --dry-run
atlas pipeline variable-update --repo SLUG --id '{VARIABLE_UUID}' --key API_KEY --value-file /path/to/replacement --dry-run
atlas pipeline variable-delete --repo SLUG --id '{VARIABLE_UUID}' --dry-run
```

Values are always omitted from output, including unsecured values and dry-run previews. Writes read the exact file contents (including trailing newlines), default to `--secured=true`, and replace the key, value, and secured flag together. Pass `--secured=false` explicitly for an unsecured variable. To read a value from terminal stdin, use `--value-file -`; through MCP, use a file path so the command does not consume the MCP transport. Dry-run does not read the value file.

`variable-list` supports `--page` and `--limit`. Use its UUIDs with the other variable commands. Deployment `variable-get` searches the paginated environment-variable list because Bitbucket has no individual GET endpoint for that scope.

#### Schedules

```sh
atlas pipeline schedule-list --repo SLUG
atlas pipeline schedule-get --repo SLUG --id '{SCHEDULE_UUID}'
atlas pipeline schedule-create --repo SLUG --branch main --custom nightly --cron '0 0 0 * * ? *' --dry-run
atlas pipeline schedule-update --repo SLUG --id '{SCHEDULE_UUID}' --enabled=false --dry-run
atlas pipeline schedule-delete --repo SLUG --id '{SCHEDULE_UUID}' --dry-run
```

The example cron runs at midnight UTC. Bitbucket's API uses seven fields (seconds, minutes, hours, day of month, month, day of week, year); Atlas checks the field count and Bitbucket validates the expression. New schedules default to enabled; pass `--enabled=false` to create one disabled. `--custom NAME` selects a custom pipeline; omitting it selects the default pipeline definition. `schedule-list` supports `--page` and `--limit`.

The API can update only a schedule's enabled status. To change its cron, branch, or pipeline definition, create a replacement schedule and delete the old one explicitly. Atlas does not silently recreate schedules.

All settings writes support `--dry-run`. Through MCP, use `atlas_read` for `variable-list/get` and `schedule-list/get`, and `atlas_write` for the corresponding create/update/delete verbs with `write_opt_in=true` to execute. JSON boolean `false` is preserved for flags such as `enabled` and `secured`.

## MCP

```sh
atlas mcp serve
```

Stdio JSON-RPC for agents. Tools: `atlas_status`, `atlas_help`, `atlas_read`, `atlas_write`. The embedded agent skill is available from `atlas_help` with `topic=atlas` and as the `atlas://skill` resource; it is not a fifth prompt. Do not pass `--human`. `atlas_read` runs read verbs only and is marked read-only, so clients such as Codex can run it without a prompt; it refuses any verb it does not list. `atlas_write` runs write verbs only, is marked destructive, and stays dry-run unless `write_opt_in` is true. Login stays `atlas auth login` in a terminal.

## Develop

```sh
make verify
```

CI runs `make verify` and builds `./cmd/atlas` on `macos-latest` for pushes and PRs to `main`.
The Chocolatey workflow builds a Windows x64 executable, packs it with its license and verification record, and checks local install, `atlas --help`, and uninstall. Publishing a GitHub release pushes the package to the Chocolatey Community Repository when `CHOCOLATEY_API_KEY` is configured as a repository secret.

## License

MIT
