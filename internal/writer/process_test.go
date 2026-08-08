package writer

import (
	"testing"

	gh "github.com/isnakolah/github-first-delivery/internal/github"
)

func TestPendingRejectsEditedAndSkipsReceipted(t *testing.T) {
	r := Request{ID: "request-1", Action: "claim", IssueID: "I1", Actor: "agent", ExpectedFingerprint: "f"}
	body, err := RenderRequest(r)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := RenderReceipt(Receipt{RequestID: r.ID, Fingerprint: "f", Result: "accepted"})
	if err != nil {
		t.Fatal(err)
	}
	comments := []gh.Comment{{ID: 1, Body: body, CreatedAt: "x", UpdatedAt: "x"}, {ID: 2, Body: receipt}}
	pending, rejected := Pending(comments)
	if len(pending) != 0 || len(rejected) != 0 {
		t.Fatalf("pending=%d rejected=%d", len(pending), len(rejected))
	}
	comments = []gh.Comment{{ID: 1, Body: body, CreatedAt: "x", UpdatedAt: "y"}}
	pending, rejected = Pending(comments)
	if len(pending) != 0 || len(rejected) != 1 {
		t.Fatalf("pending=%d rejected=%d", len(pending), len(rejected))
	}
}
