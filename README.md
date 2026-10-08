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

Per-site Basic `email:token` in the macOS keychain (file fallback if needed). Login stays in a terminal. Jira, Confluence, and JSM use `--site`. Bitbucket PRs use a **separate** workspace token; they do not reuse a licensed-site or JSM slot.

```sh
atlas auth login --site ALIAS --email EMAIL --token TOKEN
atlas auth login --workspace WORKSPACE --email EMAIL --token TOKEN
atlas auth status
```

`--site` and `--workspace` are mutually exclusive. `--from-op` is a human `login` flag only. Do not call `auth login` through MCP. Tokens never go in `config.toml`.

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

`pr get`, `list`, and `diff` need the read pullrequest and repository scopes. `pr create`, `edit`, `comment`, and `merge` also need `write:pullrequest:bitbucket`. Missing workspace cred is auth (exit 4); hint is `atlas auth login --workspace WORKSPACE`. There is no fallback to a Jira site token.

## Usage

```
atlas <namespace> <verb> [flags]
```

| Namespace | Verbs |
| --- | --- |
| `auth` | `login`, `status`, `logout` |
| `site` | `list`, `resolve` |
| `jira` | `get`, `search`, `create`, `edit`, `comment`, `transition`, `link` |
| `confluence` | `get`, `search`, `create`, `update` |
| `pr` | `get`, `list`, `create`, `comment`, `merge`, `diff` |
| `jsm` | `desks`, `types`, `list`, `get`, `create`, `comment`, `transition` |
| `mcp` | `serve` |

Every call resolves one site from config. A `jsm_customer` site refuses `jira` and `confluence` (use `atlas jsm`). `atlas pr` uses the workspace Bitbucket cred, not a site token. Exit classes: `0` success, `3` usage/config, `4` auth, `5` service, `6` not-found.

Confluence create and update convert Markdown bodies to Confluence storage markup. Use `--body-format storage` for prepared Confluence XHTML; get returns storage markup.

For an internal note on a JSM customer request at a licensed site, use `atlas jira comment KEY-1 --body '...' --internal`. Atlas checks that the request is accessible, posts with internal visibility, and verifies the response. It never falls back to a public comment. `--dry-run` previews the command without checking site access.

Find an account ID before assigning a ticket with `atlas jira users --query 'Alex' --project SDO` or `atlas jira users --query 'Alex' --issue SDO-588`. The scoped forms return users Jira considers assignable to that project or issue. Use `--site ALIAS` without a scope for a general lookup. Results include `account_id`, display name, and email when Jira permits it. Pass the ID to `jira create --assignee ID` or `jira edit KEY-1 --assignee ID`.

Create a sub-task under a story with `atlas jira create --project KEY --type Sub-task --summary '...' --parent KEY-1`. Reparent an existing sub-task with `atlas jira edit KEY-2 --parent KEY-1`. Both issues must be in the same project. Jira must allow the chosen issue type and parent. `atlas jira get KEY-2` shows the `parent` key. For a regular issue link without changing the parent, use `atlas jira link KEY-1 KEY-2 [--type Relates]`.

Jira issue descriptions and public comments accept Markdown. Atlas converts headings, lists, emphasis, links, code, and tables to Jira's rich-text document format when writing. Existing descriptions stored as literal Markdown need to be edited once to render with formatting.

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
