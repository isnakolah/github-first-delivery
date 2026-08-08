package policy

import (
	"fmt"
	"regexp"
)

var Branch = regexp.MustCompile(`^[0-9]{3}/[a-z0-9][a-z0-9-]*$`)
var IssueRef = regexp.MustCompile(`(?m)(?:Fixes|Closes|Resolves)\s+#([0-9]+)`)

func ValidatePR(branch, body string, hasLease, blockersResolved bool) error {
	if !Branch.MatchString(branch) {
		return fmt.Errorf("branch must match NNN/short-description")
	}
	if !hasLease {
		return fmt.Errorf("active lease required")
	}
	if !blockersResolved {
		return fmt.Errorf("unresolved blocker")
	}
	if !IssueRef.MatchString(body) {
		return fmt.Errorf("PR body must close exactly one Issue")
	}
	return nil
}
