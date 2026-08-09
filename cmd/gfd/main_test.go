package main

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

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

func TestJournalRenderIsDeterministic(t *testing.T) {
	output, err := captureStdout(t, func() error {
		return journalCommand([]string{"render", "--request-id", "r1", "--date", "2026-08-09", "--issue", "#13", "--pr", "https://example.test/pr/1", "--outcome", "recorded", "--proof", "CI", "--boundary", "CI", "--next-blocker", "None"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "<!-- gfd-journal:v1 request=r1 -->") || !strings.Contains(output, "- Issue: #13") {
		t.Fatalf("unexpected journal output: %s", output)
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	previous := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	err = fn()
	_ = writer.Close()
	os.Stdout = previous
	var output bytes.Buffer
	if _, copyErr := io.Copy(&output, reader); copyErr != nil && err == nil {
		err = copyErr
	}
	_ = reader.Close()
	return output.String(), err
}
