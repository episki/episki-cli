<div align="center">

# episki

</div>

# episki CLI

The official command-line interface for episki.

## Install

### curl

```sh
curl -sSf https://cli.episki.com/install.sh | sh
```

### Nix

Run without installing:

```sh
nix run github:episki/episki-cli -- auth status
```

Install into your profile:

```sh
nix profile install github:episki/episki-cli
```

Requires [Nix](https://nixos.org/download) (or [Lix](https://lix.systems/install/)) with flakes enabled.

### Install with Go

Requires [Go](https://go.dev/doc/install) 1.22+.

```sh
go install 'github.com/episki/episki-cli/cmd/episki@latest'
```

The binary lands in `$(go env GOPATH)/bin`. Add that to your `PATH` if commands aren't found.

## Quick start

```sh
episki auth login
episki auth whoami
```

## Upgrading

```sh
episki upgrade                  # latest
episki upgrade --version 0.3.1  # pin a version
episki upgrade --force          # reinstall current
```

Set `EPISKI_INSTALL_DIR` to override the install location.

When a newer release is available, `episki` prints a one-line notice on stderr after the command runs (at most once per day). To disable:

```sh
export EPISKI_NO_UPDATE_CHECK=1
```

## Usage

```sh
episki [resource] <command> [flags...]
```

For help on any command, append `--help`.

### Authentication

The CLI talks to the [Supabase Data API](https://supabase.com/docs/guides/api) (PostgREST) using the signed-in user's JWT. **All authorization is enforced by [Row Level Security](https://supabase.com/docs/guides/database/postgres/row-level-security) policies in Postgres** — the CLI grants no permissions of its own; whatever your user can see and do in the database is what you can see and do here.

`episki auth login` runs an OAuth 2.0 PKCE flow against Supabase Auth in your browser. Tokens are stored in your OS keychain. The access token is automatically refreshed before each request when it's about to expire.

Credentials are resolved in this order:

1. `--api-key <jwt>` flag
2. `EPISKI_API_KEY` environment variable (a Supabase user JWT)
3. OAuth session from `episki auth login` (stored in your OS keychain)

Run `episki auth status` to see which credential is active.

> The `--api-key` / `EPISKI_API_KEY` paths are intended for non-interactive use (CI scripts) and expect a **user-scoped Supabase JWT**, not a service-role key. Service-role keys bypass RLS and must never be passed to a client tool.

### Global flags

- `--api-key` — Supabase user JWT for non-interactive use.
- `--debug` — Enable debug logging (HTTP requests/responses).
- `--version`, `-v` — Show the CLI version.
- `--help` — Show command-line usage.

### Environment variables

| Variable                  | Description                                                  |
| ------------------------- | ------------------------------------------------------------ |
| `EPISKI_API_KEY`          | Supabase user JWT for non-interactive use.                   |
| `SUPABASE_URL`            | Override the Supabase project URL (e.g. for staging).        |
| `SUPABASE_ANON_KEY`       | Override the public anon key.                                |
| `EPISKI_INSTALL_DIR`      | Override the install dir for `episki upgrade`.               |
| `EPISKI_NO_UPDATE_CHECK`  | Set to `1` to silence the daily update notice.               |

### Configuration

User config lives at `~/.config/episki/config.toml` (or `$XDG_CONFIG_HOME/episki/config.toml`). The defaults baked into the binary point at the production episki Supabase project; override per-host as needed:

```toml
[supabase]
url       = "https://your-project.supabase.co"
anon_key  = "eyJhbGciOi..."
provider  = "google"   # default OAuth provider for `episki auth login`
```

## Development

```sh
./scripts/run auth status
```
