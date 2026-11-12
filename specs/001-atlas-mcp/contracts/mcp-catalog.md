# MCP catalog

Public interface: `atlas mcp serve` on stdio JSON-RPC. Human CLI remains `atlas <namespace> <verb> [flags]`. This catalog is the MCP door, not an Atlassian REST dump.

`tools/list` MUST return exactly these four names.

Transport: stdio only. No SSE, no Streamable HTTP.

## Tools

| Name | Description (MUST convey) | Annotations | Session |
| --- | --- | --- | --- |
| `atlas_status` | Signed-in, session usable, per-site role, and per-workspace usability. No tokens. Does not open a browser. | `readOnlyHint` | Optional |
| `atlas_help` | CLI help for a namespace or verb, or a recipe `topic` (`jira-search`, `confluence-write`, `pr-review`, `jsm-customer`, `atlas`). No session required. | `readOnlyHint`, `openWorldHint` false | None |
| `atlas_read` | Run one read namespace+verb with a flag map. Returns that command's JSON. Refuses write verbs. Lookup examples: help topics jira-search, confluence-write, pr-review, jsm-customer. Skill: `atlas://skill`. | `readOnlyHint`, `openWorldHint` | Required for workloads |
| `atlas_write` | Run one write namespace+verb with a flag map. Dry-run preview unless `write_opt_in` is true. Refuses read verbs. | `destructiveHint`, `openWorldHint` | Required for workloads |

Unknown tool name (including the removed `atlas_run`) → MCP protocol error. Do not add `atlas_login`, or per-verb tools.

Annotations let clients such as Codex run read-only tools without an approval prompt. `readOnlyHint` MUST be true only on tools that cannot change a workload.

## `atlas_status`

**Input**: empty object.

**Output**: same JSON as `atlas auth status`. Keys: `signed_in`, `session_usable`, `sites[]` with `alias`, `hostname`, `role` (`licensed` | `jsm_customer`), `usable`, and `workspaces[]` with `slug`, `usable`. MUST NOT include email or token fields. A usable credential in either collection makes the session signed in and usable.

Signed out → success, `signed_in` false, no interactive login.

## `atlas_help`

**Input**:

| Field | Type | Required |
| --- | --- | --- |
| `namespace` | string | no |
| `verb` | string | no |
| `topic` | string | no (`jira-search` \| `confluence-write` \| `pr-review` \| `jsm-customer` \| `atlas`) |

No args → overview: four tools, four recipe topics, skill resource, write opt-in. `topic` set → recipe or skill text. `namespace` / `verb` → CLI help. `topic` and `namespace` together → usage. Unknown topic → usage. No session. Text, not JSON.

## Named recipes (MCP prompts)

`prompts/list` MUST return exactly: `jira-search`, `confluence-write`, `pr-review`, `jsm-customer`. `prompts/get` returns the same body as `atlas_help` for that topic. MUST NOT add a fifth prompt. The baked skill is not a prompt.

## Skill resource

`resources/list` MUST include `atlas://skill` (`text/markdown`). `resources/read` returns the same body as `atlas_help` `topic=atlas`. Unknown resource URIs MUST fail without returning the skill body. This is how the binary ships the agent skill.

## `atlas_read` and `atlas_write`

**Input**: `namespace`, `verb`, optional `args`, `flags`. `atlas_write` also takes `write_opt_in`.

**Output (success)**: one text content item = CLI JSON stdout for the equivalent command. Help goes through `atlas_help`.

**Argument safety**: a positional arg that starts with `-` and a flag name outside `[a-z][a-z0-9-]*` → `usage`. Neither can end flag parsing early or override the injected `--dry-run`.

**Output (failure)**: `isError` true; one text content item `{class,message,hint}`. Classes MUST remain `usage` | `auth` | `service` | `not_found`.

### Read list

`auth status`, `site list|resolve`, `jira get|search`, `confluence get|search`, `pr get|list|diff`, `pipeline list|get|steps|logs|variable-list|variable-get|schedule-list|schedule-get`, `jsm desks|types|list|get`. `atlas_read` refuses every other verb with `usage`: a write verb gets hint `use atlas_write`. The list is an allowlist, so a new verb is refused until it is classified.

### Write gate

Write list: `jira create|edit|comment|transition|link`, `confluence create|update`, `pr create|comment|merge`, `pipeline run|stop|variable-create|variable-update|variable-delete|schedule-create|schedule-update|schedule-delete`, `jsm create|comment|transition`. `atlas_write` refuses every other verb with `usage`: a read verb gets hint `use atlas_read`. Writes dry-run unless `write_opt_in` is true.

Boolean false flag values are retained, including `secured:false` and `enabled:false`. Variable writes take a `value-file` path; `value-file:"-"` is refused through MCP because stdin carries the transport. Variable values are omitted from all outputs and previews.

### Forbidden via either tool

`auth login`, `auth logout`, namespace `mcp` → `usage` with hint to run `atlas auth login` in a terminal.

## CLI surface

| Command | Behavior |
| --- | --- |
| `atlas --help` | Lists `auth`, `site`, `jira`, `confluence`, `pr`, `pipeline`, `jsm`, `mcp`. JSON, exit classes 3/4/5/6. |
| `atlas mcp --help` / `atlas mcp serve --help` | Exit 0, no session. Names stdio, four tools, write opt-in default false, four recipe topics. |
| `atlas mcp serve` | JSON-RPC on stdio until stdin closes. `--human` → usage (3). |
| `atlas auth status` | Signed-out JSON with catalog sites, no tokens. |

## Error mapping

| CLI exit | MCP |
| --- | --- |
| 0 | `isError` false, stdout text |
| 3 | `isError` true, `class=usage` |
| 4 | `isError` true, `class=auth` |
| 5 | `isError` true, `class=service` |
| 6 | `isError` true, `class=not_found` |

Do not map these onto JSON-RPC error codes.

## Secrets

Redact access tokens, refresh tokens, authorization codes, API tokens, and client secrets in every MCP content item and log line.
