---
name: atlas
description: >
  Use dedicated atlas MCP for Jira, Confluence, Bitbucket, and JSM customer
  REST on one configured cloud per call. Use when calling atlas_status,
  atlas_help, atlas_read, or atlas_write, or when the operator names atlas, Jira,
  Confluence, Bitbucket PRs, or a JSM customer portal.
---

# Atlas MCP

Four tools on stdio. Prefer them over a shell `atlas` one-off when the
session has the atlas server. Sites, project keys, space keys, Bitbucket
workspace, and the default JSM site live in the operator catalog
(`ATLAS_CONFIG` or `$XDG_CONFIG_HOME/atlas/config.toml`). Login stays
`atlas auth login` in a terminal.

| Tool | When |
|------|------|
| `atlas_status` | Signed-in, per-site role, and per-workspace usability. No tokens. Does not open a browser. |
| `atlas_help` | Namespace/verb help, or recipe `topic`. No session. |
| `atlas_read` | One read namespace+verb. Returns that command's JSON. Read-only. |
| `atlas_write` | One write namespace+verb. Dry-run preview unless `write_opt_in`. |

Do not add per-verb tools. Do not invent Atlassian REST.

## Call shape

`atlas_read` input: `namespace`, `verb`, optional `args`, `flags`.
`atlas_write` takes the same plus `write_opt_in`. Flag names are long flags
without dashes (`jql`, `site`, `dry-run`). Writes stay dry-run unless
`write_opt_in` is true. Each tool refuses the other's verbs, and
`atlas_read` refuses any verb it does not list.

Forbidden via either tool: `auth login`, `auth logout`, namespace `mcp`.
Those are terminal commands.

Failure content is `{class,message,hint}` with `usage` | `auth` | `service` | `not_found`. Site auth → tell the operator to run `atlas auth login --site ALIAS`. PR auth → tell the operator to run `atlas auth login --workspace WORKSPACE`. Do not print tokens.

Recipes (`atlas_help` `topic`, also MCP prompts): `jira-search`,
`confluence-write`, `pr-review`, `jsm-customer`. This skill is also
`atlas_help` `topic=atlas` and MCP resource `atlas://skill`.

## Site isolation

Every call resolves **one** site from the catalog. Project keys and space
keys map to an alias. JQL/CQL that names two sites is usage. A
`jsm_customer` site refuses `jira` and `confluence` (use `jsm`). A
`licensed` site refuses `jsm`.

## Workloads

| Need | Call |
|------|-------------|
| Status | `atlas_status` |
| Help / recipe | `atlas_help` `topic=jira-search` (or namespace/verb) |
| Get issue | `atlas_read` `namespace=jira` `verb=get` `args=["KEY-1"]` |
| Search Jira | `atlas_read` `namespace=jira` `verb=search` `flags={jql:"project = KEY"}` |
| Find and assign a Jira user | Follow **Assign a Jira issue** below |
| Create issue | `atlas_write` `namespace=jira` `verb=create` `flags={project,type,summary}` + `write_opt_in` |
| Create sub-task under story | `atlas_write` `namespace=jira` `verb=create` `flags={project,type:"Sub-task",summary,parent:"KEY-1"}` + `write_opt_in` |
| Reparent sub-task | `atlas_write` `namespace=jira` `verb=edit` `args=["KEY-2"]` `flags={parent:"KEY-1"}` + `write_opt_in` |
| Link issues without changing parent | `atlas_write` `namespace=jira` `verb=link` `args=["KEY-1","KEY-2"]` + `write_opt_in` |
| Comment / transition / link | `atlas_write` `jira` `comment` / `transition` / `link` |
| Internal JSM note on a licensed site | `atlas_write` `namespace=jira` `verb=comment` `args=["KEY-1"]` `flags={body:"…",internal:true}` + `write_opt_in`; the key must resolve as a customer request |
| Confluence get/search | `atlas_read` `confluence` `get` / `search` (`cql`); body is storage markup |
| Confluence create/update | `atlas_write`; Markdown is converted to storage markup. Set `body-format:"storage"` for prepared Confluence XHTML; no delete |
| PR get/list/diff | `atlas_read` `pr` `get` / `list` / `diff` (`repo`, `id`) |
| PR create/edit/comment/merge | `atlas_write`; edit an open PR's title or description with `namespace=pr` `verb=edit` `flags={repo:"SLUG",id:1,description:"…"}` + `write_opt_in`; no delete |
| JSM desks/types/list/get | `atlas_read` `jsm` `desks` / `types` / `list` / `get` |
| JSM create/comment/transition | `atlas_write`; comments are public |

## Assign a Jira issue

1. Identify the issue key for an existing issue or the project key for a new one. For an existing issue, read it with `atlas_read` `namespace=jira` `verb=get` `args=["KEY-1"]` so the target and site are clear.
2. Search for the person with `atlas_read` `namespace=jira` `verb=users` `flags={query:"Name or email",issue:"KEY-1"}` for an existing issue, or `flags={query:"Name or email",project:"KEY"}` for a new one. These scoped searches return users Jira considers assignable to that issue or project.
3. Select an active user whose `display_name` and, when present, `email` match the intended person. Use that result's `account_id`. Refine the query if there is no match; ask the operator which person they mean if multiple results remain plausible. Jira may hide email addresses.
4. For an existing issue, call `atlas_write` `namespace=jira` `verb=edit` `args=["KEY-1"]` `flags={assignee:"ACCOUNT_ID"}`. For a new issue, include `assignee:"ACCOUNT_ID"` with the required `project`, `type`, and `summary` flags in `jira create`. Set `write_opt_in=true` when the operator has authorized the change.
5. Read the issue again with `jira get` (using the key returned by `jira create` for a new issue). Confirm its assignee is the selected person; report a mismatch rather than claiming assignment succeeded.

## Jira descriptions

When creating an issue, pass Markdown in the `description` flag. When editing one, pass a Markdown string as `description` inside the `fields` JSON object. Atlas converts those strings to Jira rich text on write. Read the issue with `jira get` afterward to check the stored description.

## Preflight

If `atlas_status` shows the needed site or workspace `usable` false, stop and tell the operator to use the matching terminal login command. Sites use `atlas auth login --site ALIAS --email EMAIL --token TOKEN`; PR workspaces use `atlas auth login --workspace WORKSPACE --email EMAIL --token TOKEN`. Do not loop on login. A site credential never authorizes a PR operation.
