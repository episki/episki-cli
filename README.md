<div align="center">

# episki

</div>

# episki CLI

The official command-line interface for episki.

## Install

### Homebrew

```sh
brew install episki/tap/episki
```

### curl

```sh
curl -sSf https://cli.episki.com/install.sh | sh
```

### Install with Go

Requires [Go](https://go.dev/doc/install) 1.22+.

```sh
go install 'github.com/episki/episki-cli/cmd/episki@latest'
```

The binary lands in `$(go env GOPATH)/bin`. Add that to your `PATH` if commands aren't found.

## Quick start

```sh
episki auth login                 # browser sign-in (Google by default)
episki workspaces list            # workspaces you belong to
episki workspaces use <id|slug>   # pick the active workspace
episki work-items list            # you're in business
```

## Commands

```sh
episki [resource] <command> [flags...]
```

| Resource | Commands |
| --- | --- |
| `auth` | `login`, `logout`, `status`, `whoami`, `refresh` |
| `workspaces` (`ws`) | `list`, `current`, `use <id\|slug>` |
| `frameworks` | `list`, `get <id>` |
| `controls` | `list`, `get <id\|ref>` |
| `work-items` (`wi`, `tasks`) | `list`, `get <id\|ref>`, `update <id\|ref> --status/--due/--name`, `archive`, `restore` |
| `evidence` | `list`, `get <id>` |
| `policies` | `list`, `get <id>` |
| `risks` | `list`, `get <id\|ref>` |

List commands take `--limit N` (default 50, server caps at 1000) and `--json`
for scripting; `get` always prints JSON. For help on any command, append `--help`.

## Authentication

The CLI talks to the episki Supabase Data API (PostgREST) using the signed-in
user's JWT. **All authorization is enforced by Row Level Security policies in
Postgres** — the CLI grants no permissions of its own; whatever your user can
see and do in [app.episki.com](https://app.episki.com) is what you can see
and do here.

`episki auth login` runs an OAuth PKCE flow against episki's auth in your
browser. Tokens are stored in your OS keychain and refreshed automatically.

Credentials are resolved in this order:

1. `--api-key <jwt>` flag
2. `EPISKI_API_KEY` environment variable (a user-scoped access token)
3. OAuth session from `episki auth login` (stored in your OS keychain)

Run `episki auth status` to see which credential is active.

> The `--api-key` / `EPISKI_API_KEY` paths are intended for non-interactive
> use (CI scripts) and expect a **user-scoped access token**, never a
> service-role or secret key.

## Workspaces

Everything in episki is scoped to a **workspace**, and the scoping lives in
your token: the active workspace id is a claim on your JWT
(`app_metadata.workspace_id`) that every RLS policy reads. Without it, all
entity commands would return nothing — so the CLI errors with a hint instead.

- `episki workspaces use <id|slug>` asks the episki app to stamp the claim,
  then refreshes your session so the new token carries it.
- If you switch workspaces in the web app instead, run `episki auth refresh`
  to pick the change up.
- `episki auth status` and `episki workspaces current` show the active
  workspace.

## Global flags

- `--api-key` — user access token for non-interactive use.
- `--debug` — Enable debug logging.
- `--version`, `-v` — Show the CLI version.
- `--help` — Show command-line usage.

## Environment variables

| Variable                 | Description                                                    |
| ------------------------ | -------------------------------------------------------------- |
| `EPISKI_API_KEY`         | User access token for non-interactive use.                     |
| `SUPABASE_URL`           | Override the Supabase project URL (e.g. local dev).            |
| `SUPABASE_KEY`           | Override the publishable key (`sb_publishable_*`).             |
| `SUPABASE_ANON_KEY`      | Legacy alias for `SUPABASE_KEY`; wins if both are set.         |
| `EPISKI_APP_URL`         | Override the web app origin used by `workspaces use`.          |
| `EPISKI_INSTALL_DIR`     | Override the install dir for `episki upgrade`.                 |
| `EPISKI_NO_UPDATE_CHECK` | Set to `1` to silence the daily update notice.                 |

## Configuration

User config lives at `~/.config/episki/config.toml` (or
`$XDG_CONFIG_HOME/episki/config.toml`). The defaults baked into the binary
point at the production episki project (`https://api.episki.com`); override
per-host as needed:

```toml
app_url = "https://app.episki.com"

[supabase]
url      = "https://api.episki.com"
anon_key = "sb_publishable_..."   # publishable key; field name is historical
provider = "google"               # default OAuth provider for `episki auth login`
```

For local development against `supabase start`, set `SUPABASE_URL=http://127.0.0.1:54321`
and `SUPABASE_KEY` to your local publishable key.

## Upgrading

```sh
episki upgrade                  # latest
episki upgrade --version 0.3.1  # pin a version
episki upgrade --force          # reinstall current
```

Set `EPISKI_INSTALL_DIR` to override the install location.

When a newer release is available, `episki` prints a one-line notice on
stderr after the command runs (at most once per day). To disable:

```sh
export EPISKI_NO_UPDATE_CHECK=1
```

## Development

```sh
./scripts/run auth status
```

## Releasing

Releases are automated: release-please opens a version PR off `main`;
merging it tags a release and goreleaser builds darwin/linux (amd64/arm64)
archives, publishes them to GitHub Releases, and pushes a Homebrew formula
to `episki/homebrew-tap`.

One-time setup before the first release:

1. Create the `episki/episki-cli` GitHub repo and push `main`.
2. Create an empty `episki/homebrew-tap` repo.
3. Add a `TAP_GITHUB_TOKEN` repo secret (a token with push access to the
   tap) — the default `GITHUB_TOKEN` can't write to other repos.
4. Serve `bin/install.sh` at `https://cli.episki.com/install.sh` (the
   `episki upgrade` command and the curl installer both point there).
