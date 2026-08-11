---
name: github-first-delivery
description: GitHub-first work selection, leases, evidence, and delivery-policy guardrails.
---

Run `gfd doctor` then `gfd context --issue-number N --json` before work. Select only Ready, unblocked, unleased leaf work. Claim through `gfd work claim --issue-number N --issue-id ID --fingerprint SHA --branch NNN/short-description --apply` before branch or edit. Export accepted lease context as `GFD_LEASE_ISSUE`, `GFD_LEASE_HOLDER`, and `GFD_LEASE_BRANCH`; guard checks all three against live GitHub state. `GFD_BOOTSTRAP_ADMIN=1` is documented bootstrap exception only. Use native GitHub parent and blocker links. Never create local task ledger or autonomous Epic. Submit evidence before completion. Review and explicitly trust hooks.
