package main

import "testing"

func TestLiveWorkFingerprintIgnoresRequestCommentTimestamp(t *testing.T) {
	before := liveWork{IssueID: "I_1", IssueState: "OPEN", UpdatedAt: "2026-08-09T19:31:00Z", Status: "Ready"}
	after := before
	after.UpdatedAt = "2026-08-09T19:32:00Z"

	beforeHash, err := fingerprintLiveWork(before)
	if err != nil {
		t.Fatal(err)
	}
	afterHash, err := fingerprintLiveWork(after)
	if err != nil {
		t.Fatal(err)
	}
	if beforeHash != afterHash {
		t.Fatalf("request comment timestamp changed fingerprint: %s != %s", beforeHash, afterHash)
	}
}
