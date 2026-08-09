# Operations

Use dedicated fine-grained `GFD_WRITER_TOKEN` only in Actions secret. Scope it to target repository contents, Issues, Pull requests, Actions, and owning user Projects. Never place token in config, cache, log, request, or shell history. Rotate, revoke, and verify it through `gfd doctor` before Writer enablement.

All changing commands require `--apply`; bootstrap requires `--yes`. `gfd context` is first command for agents. Claim leaf work before branch/edit. Submit evidence with final SHA, PR/CI URLs, commands, environments, acceptance result, artifacts, documentation, risks, and proof boundary. A merged PR alone is not Done.

`gfd validate --json` reads live GitHub Issues and Project fields. It reports every detected live-work violation: missing parent/contract/Project Status/Kind/Area, parent or blocker cycle, unresolved blocker in Ready/Claimed/In progress/Done, Epic branch ownership, field-label mismatch, and a Done parent with unfinished child. Closed historical Issues remain relationship context but do not require current Project classification.

Wiki journal is generated from receipts and is noncanonical. If Wiki write fails, Writer leaves work at Evidence pending and reconciliation retries.
