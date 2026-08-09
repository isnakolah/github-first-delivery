// Package bootstrap installs source-controlled repository operating files.
package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
)

var files = map[string]string{
	".agent/rules/github-first-delivery.md": `# GitHub-first delivery

Run ` + "`gfd context`" + ` before work. Select only Ready, unblocked, unleased leaf work. Claim before creating branch or editing. Native parent and blocker links are authority. Submit evidence before completion. Never create local task ledger or autonomous Epic.
`,
	".github/ISSUE_TEMPLATE/decision.yml": `name: Decision
description: Decision requiring named approver.
title: "DECISION — "
labels: ["kind:decision"]
body:
  - type: textarea
    id: question
    attributes: {label: Decision question}
    validations: {required: true}
  - type: textarea
    id: options
    attributes: {label: Options}
    validations: {required: true}
  - type: textarea
    id: consequences
    attributes: {label: Consequences}
    validations: {required: true}
  - type: input
    id: approver
    attributes: {label: Required approver}
    validations: {required: true}
`,
	".github/ISSUE_TEMPLATE/epic.yml": `name: Epic
description: Owner-approved long-lived outcome. Never implementation leaf.
title: "EPIC — "
labels: ["kind:epic"]
body:
  - type: markdown
    attributes:
      value: "Owner approval required before Ready. Epic never carries implementation branch."
  - type: textarea
    id: outcome
    attributes: {label: Outcome}
    validations: {required: true}
  - type: textarea
    id: non_goals
    attributes: {label: Non-goals}
    validations: {required: true}
  - type: textarea
    id: constraints
    attributes: {label: Standing constraints}
    validations: {required: true}
  - type: textarea
    id: exit
    attributes: {label: Exit criteria}
    validations: {required: true}
  - type: textarea
    id: children
    attributes: {label: Expected children}
    validations: {required: true}
  - type: input
    id: approval
    attributes: {label: Owner approval, placeholder: "@owner approval comment URL"}
    validations: {required: true}
`,
	".github/ISSUE_TEMPLATE/work.yml": `name: Work
description: Story, Contract, Task, Gate, or Defect with GitHub-first contract.
title: "US- — "
body:
  - type: markdown
    attributes:
      value: |
        <!-- work:v1 -->
        ## Outcome
        ## Scope
        Includes:
        -
        Excludes:
        -
        ## Acceptance criteria
        - [ ]
        ## Required evidence
        -
        - Verification boundary: local | CI | target host | provider | staging | production | release
        ## Documentation impact
        None: explain specific reason
        ## Source and relationships
        - Native parent and blocker links are authoritative.
  - type: textarea
    id: contract
    attributes: {label: Work contract, description: "Copy exact contract above. Add native parent after creation."}
    validations: {required: true}
`,
}

func Install(root, owner string, projectNumber int) error {
	all := make(map[string]string, len(files)+1)
	for path, body := range files {
		all[path] = body
	}
	all[".github/ISSUE_TEMPLATE/config.yml"] = fmt.Sprintf(`blank_issues_enabled: false
contact_links:
  - name: Delivery Project
    url: https://github.com/users/%s/projects/%d
    about: Select existing Ready work. Do not use free-form task intake.
`, owner, projectNumber)
	for path, body := range all {
		target := filepath.Join(root, path)
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("refuse to overwrite bootstrap file %s", path)
		} else if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, []byte(body), 0644); err != nil {
			return err
		}
	}
	return nil
}
