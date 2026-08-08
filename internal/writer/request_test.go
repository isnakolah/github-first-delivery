package writer

import (
	"strings"
	"testing"
)

func TestRequestRoundTripAndTamper(t *testing.T) {
	r := Request{ID: "r1", Action: "claim", IssueID: "I1", Actor: "agent", ExpectedFingerprint: "f"}
	body, err := RenderRequest(r)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ParseRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != r.ID {
		t.Fatalf("got %q", got.ID)
	}
	if _, err := ParseRequest(strings.Replace(body, "sha256=", "sha256=bad", 1)); err == nil {
		t.Fatal("expected tamper error")
	}
}
func TestTransitions(t *testing.T) {
	if !CanTransition("Ready", "Claimed") || CanTransition("Done", "Ready") {
		t.Fatal("bad transitions")
	}
}
