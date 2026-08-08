# Operations

Use dedicated fine-grained `GFD_WRITER_TOKEN` only in Actions secret. Scope it to target repository contents, Issues, Pull requests, Actions, and owning user Projects. Never place token in config, cache, log, request, or shell history. Rotate, revoke, and verify it through `gfd doctor` before Writer enablement.

All changing commands require `--apply`; bootstrap requires `--yes`. `gfd context` is first command for agents. Claim leaf work before branch/edit. Submit evidence with final SHA, PR/CI URLs, commands, environments, acceptance result, artifacts, documentation, risks, and proof boundary. A merged PR alone is not Done.

Wiki journal is generated from receipts and is noncanonical. If Wiki write fails, Writer leaves work at Evidence pending and reconciliation retries.
