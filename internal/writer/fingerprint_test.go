package writer

import "testing"

func TestFingerprintCanonicalizesRelationshipOrder(t *testing.T) {
	a := Fingerprint{IssueID: "I", DependencyIDs: []string{"b", "a"}, RequestIDs: []string{"2", "1"}, Project: map[string]string{"status": "Ready"}}
	b := Fingerprint{IssueID: "I", DependencyIDs: []string{"a", "b"}, RequestIDs: []string{"1", "2"}, Project: map[string]string{"status": "Ready"}}
	ha, err := StateFingerprint(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := StateFingerprint(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("fingerprints differ: %s %s", ha, hb)
	}
}

func TestRequireFreshFingerprintRejectsStale(t *testing.T) {
	state := Fingerprint{IssueID: "I", IssueState: "OPEN"}
	actual, err := StateFingerprint(state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RequireFreshFingerprint(actual, state); err != nil {
		t.Fatal(err)
	}
	if _, err := RequireFreshFingerprint("stale", state); err == nil {
		t.Fatal("expected stale error")
	}
}
