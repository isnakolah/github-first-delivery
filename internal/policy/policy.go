package policy

import (
	"fmt"
	"regexp"
	"strconv"
)

var Branch = regexp.MustCompile(`^[0-9]{3}/[a-z0-9][a-z0-9-]*$`)
var IssueRef = regexp.MustCompile(`(?mi)\b(?:Fixes|Closes|Resolves|Refs)\s+#([0-9]+)\b`)

// ReferencedIssues accepts exactly one canonical Issue reference from a PR
// body. Policy needs one unambiguous implementation leaf per branch and PR.
func ReferencedIssues(body string) ([]int, error) {
	matches := IssueRef.FindAllStringSubmatch(body, -1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("PR body must reference exactly one Issue")
	}
	number, err := strconv.Atoi(matches[0][1])
	if err != nil || number < 1 {
		return nil, fmt.Errorf("PR body has invalid Issue reference")
	}
	return []int{number}, nil
}

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
	if _, err := ReferencedIssues(body); err != nil {
		return err
	}
	return nil
}
