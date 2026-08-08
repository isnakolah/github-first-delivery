# Source install and recovery

`github-first-delivery` is source-first. No release binary or marketplace package is a supported install path before GFD v0 Gate passes.

## Requirements

- macOS or Linux host;
- Go 1.26.4;
- GitHub CLI authenticated to target owner/repository;
- a dedicated fine-grained Writer token only when activating Writer in Actions.

## Install

```sh
gh repo clone isnakolah/github-first-delivery
cd github-first-delivery
GOBIN="$HOME/.local/bin" go install ./cmd/gfd
export PATH="$HOME/.local/bin:$PATH"
gfd doctor --json
gfd context --json
```

`gfd doctor` reports local config/token visibility only. It does not prove Writer authority, hook trust, target-host behavior, or provider state.

## Plugin source install

Install local Codex and Claude plugin entries from checked-out `plugins/` source. Review every hook script before trust. Trust by reviewed hash in each client; installation must never bypass hook trust.

Before editing work, claim an accepted leaf and export its context:

```sh
export GFD_LEASE_ISSUE=123
export GFD_LEASE_HOLDER='provider/profile/session'
```

Hooks block write paths without both variables. `GFD_BOOTSTRAP_ADMIN=1` permits only documented repository bootstrap administration; remove it immediately after that operation.

## Upgrade and recovery

Pull source, rerun `go install`, then `gfd doctor`. Recreate local cache freely: cache is fetched metadata only and GitHub remains authority. If a request needs correction, create a new request comment; do not edit prior request or receipt comments.

## Proof status

macOS source install has local development evidence. Fresh macOS and Linux source-install, trusted-plugin, and hook-behavior proof remain release-Gate requirements.
