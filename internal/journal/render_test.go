package journal

import "testing"

func TestRenderIsIdempotencyKeyed(t *testing.T) {
	e := Render(Entry{RequestID: "x", Date: "2026-08-08", Issue: "#1"})
	if !HasRequest(e, "x") || HasRequest(e, "y") {
		t.Fatal("bad journal key")
	}
}

func TestAppendDoesNotDuplicateReceipt(t *testing.T) {
	entry := Entry{RequestID: "x", Date: "2026-08-08", Issue: "#1"}
	log := Append("", entry)
	if got := Append(log, entry); got != log {
		t.Fatal("duplicate journal entry")
	}
}
