package model

import (
	"fmt"
	"strings"
)

const WorkMarker = "<!-- work:v1 -->"

var RequiredWorkSections = []string{"## Outcome", "## Scope", "## Acceptance criteria", "## Required evidence", "## Documentation impact", "## Source and relationships"}

func ValidateWorkContract(body string) error {
	if !strings.Contains(body, WorkMarker) {
		return fmt.Errorf("missing %s", WorkMarker)
	}
	for _, section := range RequiredWorkSections {
		if !strings.Contains(body, section) {
			return fmt.Errorf("missing required section %q", section)
		}
	}
	if !strings.Contains(body, "Verification boundary:") {
		return fmt.Errorf("missing verification boundary")
	}
	if !strings.Contains(body, "Includes:") || !strings.Contains(body, "Excludes:") {
		return fmt.Errorf("scope requires Includes and Excludes")
	}
	return nil
}

func WorkBody(outcome, includes, excludes, criteria, evidence, docs, source, boundary string) string {
	return fmt.Sprintf(`%s
## Outcome

%s

## Scope
Includes:
- %s

Excludes:
- %s

## Acceptance criteria
- [ ] %s

## Required evidence
- %s
- Verification boundary: %s

## Documentation impact
%s

## Source and relationships
- %s
- Native parent and blocker links are authoritative.
`, WorkMarker, outcome, includes, excludes, criteria, evidence, boundary, docs, source)
}
