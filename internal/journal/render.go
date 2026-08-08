package journal

import (
	"fmt"
	"strings"
)

type Entry struct{ RequestID, Date, Issue, PR, Outcome, Proof, Boundary, NextBlocker string }

func Render(entry Entry) string {
	return fmt.Sprintf("<!-- gfd-journal:v1 request=%s -->\n## %s — %s\n\n- Issue: %s\n- PR: %s\n- Outcome: %s\n- Proof: %s\n- Boundary: %s\n- Next blocker: %s\n", entry.RequestID, entry.Date, entry.Issue, entry.Issue, entry.PR, entry.Outcome, entry.Proof, entry.Boundary, entry.NextBlocker)
}
func HasRequest(log, requestID string) bool {
	return strings.Contains(log, "<!-- gfd-journal:v1 request="+requestID+" -->")
}
