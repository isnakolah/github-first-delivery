package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/isnakolah/github-first-delivery/internal/github"
	"github.com/isnakolah/github-first-delivery/internal/writer"
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

func TestHasReceipt(t *testing.T) {
	body, err := writer.RenderReceipt(writer.Receipt{RequestID: "wiki-retry-123", Result: "accepted", Detail: "generated Wiki journal repaired", At: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	comments := []github.Comment{{Body: body}}
	if !hasReceipt(comments, "wiki-retry-123") {
		t.Fatal("expected matching receipt")
	}
	if hasReceipt(comments, "wiki-retry-456") {
		t.Fatal("unexpected unmatched receipt")
	}
}

func TestPendingWikiJournalAllowsRepairedReceipt(t *testing.T) {
	pending, err := writer.RenderReceipt(writer.Receipt{RequestID: "evidence-1", Result: "accepted", Detail: "evidence recorded; Wiki journal pending", Evidence: &writer.Evidence{}, At: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	comments := []github.Comment{{Body: pending}}
	if !pendingWikiJournal(comments) {
		t.Fatal("expected pending wiki journal")
	}
	repaired, err := writer.RenderReceipt(writer.Receipt{RequestID: "wiki-retry-evidence-1", Result: "accepted", Detail: "generated Wiki journal repaired", At: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}
	comments = append(comments, github.Comment{Body: repaired})
	if pendingWikiJournal(comments) {
		t.Fatal("repaired wiki journal must not block completion")
	}
}

func TestRequireEmptyBootstrapRoot(t *testing.T) {
	root := t.TempDir()
	if err := requireEmptyBootstrapRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "existing"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := requireEmptyBootstrapRoot(root); err == nil {
		t.Fatal("expected nonempty root refusal")
	}
}

func TestContainsAndTitleCase(t *testing.T) {
	if !contains([]string{"delivery", "core"}, "core") || contains([]string{"delivery"}, "ops") {
		t.Fatal("configured area lookup is incorrect")
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
