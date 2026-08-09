# Operations

Use dedicated fine-grained `GFD_WRITER_TOKEN` only in Actions secret. Scope it to target repository contents, Issues, Pull requests, Actions, and owning user Projects. Never place token in config, cache, log, request, or shell history. Rotate, revoke, and verify it through `gfd doctor` before Writer enablement.

All changing commands require `--apply`; bootstrap requires `--yes`. `gfd context` is first command for agents. Claim leaf work before branch/edit. Submit evidence with final SHA, PR/CI URLs, commands, environments, acceptance result, artifacts, documentation, risks, and proof boundary. A merged PR alone is not Done.

`gfd validate --json` reads live GitHub Issues and Project fields. It reports every detected live-work violation: missing parent/contract/Project Status/Kind/Area, parent or blocker cycle, unresolved blocker in Ready/Claimed/In progress/Done, Epic branch ownership, field-label mismatch, and a Done parent with unfinished child. Closed historical Issues remain relationship context but do not require current Project classification.

Use `gfd context --issue-number N --json` to obtain Issue node ID and current state fingerprint. The Writer rejects a changed fingerprint. Claims also require `--branch NNN/short-description`; leases use a two-hour maximum. On an Issue-comment event, active Writer runs fingerprint verification, lifecycle validation, Project field update, then receipt emission. It never activates without `GFD_WRITER_TOKEN`.

`gfd writer reconcile --apply` scans every open Issue, replays unreceipted requests, and returns expired Claimed/In progress leases to Ready while retaining branch context. Scheduled Writer runs invoke it every five minutes. Reconciliation writes an expiry receipt only once.

Evidence is a Writer request, never a free-form completion comment. `gfd evidence submit` requires `--pr`, final SHA, CI URL, exact commands, environments, criteria result, artifacts (`None: reason` allowed), documentation impact, residual risks (`None` allowed), and proof boundary. Valid evidence moves only In review work to Evidence pending and is preserved in Writer receipt JSON.

Wiki journal is generated from receipts and is noncanonical. If Wiki write fails, Writer leaves work at Evidence pending and reconciliation retries.
