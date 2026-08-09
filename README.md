# GitHub-first Delivery

`gfd` is source-first Go control plane for GitHub-only delivery state: Issue graph, Project workflow, leases, durable mutation requests, receipts, evidence, and generated noncanonical Wiki journal.

Status: bootstrap implementation. `v0` not released. Do not use for production coordination until disposable-repository integration gate passes.

## Source install

```sh
gh repo clone isnakolah/github-first-delivery
cd github-first-delivery
GOBIN="$HOME/.local/bin" go install ./cmd/gfd
gfd doctor
```

## Operating contract

GitHub is authority. No local `plan/`, `tasks/`, roadmap, or TODO ledger. Every live non-Epic Issue has native parent. Every implementation branch and PR maps to one leaf Issue. Project `Status` is only mutable workflow state; status labels forbidden. Writer serializes mutations. Evidence receipt, not merged PR, closes work.

Read [operations guide](docs/operations.md), [source install and recovery](docs/source-install.md), [architecture](docs/architecture.md), and GitHub Project before work.

## Development

```sh
go test ./...
go vet ./...
go build ./cmd/gfd
scripts/check-plugins.sh
```

Plugins live under `plugins/codex` and `plugins/claude`. Review and explicitly trust bundled hooks. Hooks guard common paths only; Writer, CI, and protected branches enforce policy.

License: [MIT](LICENSE).
