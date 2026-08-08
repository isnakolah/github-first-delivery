package writer

import (
	"fmt"
	"strings"
	"time"

	gh "github.com/isnakolah/github-first-delivery/internal/github"
)

// PendingRequest is a validated request comment eligible for exactly one receipt.
// Mutating lifecycle state belongs to later Writer stories; this layer owns durable
// parsing, tamper rejection, edit rejection, and idempotency detection.
type PendingRequest struct {
	Request   Request
	CommentID int64
}

func Pending(comments []gh.Comment) ([]PendingRequest, []Receipt) {
	receipted := make(map[string]bool)
	for _, comment := range comments {
		if strings.Contains(comment.Body, ReceiptPrefix) {
			for _, id := range receiptRequestIDs(comment.Body) {
				receipted[id] = true
			}
		}
	}
	var pending []PendingRequest
	var rejected []Receipt
	for _, comment := range comments {
		if !strings.HasPrefix(strings.TrimSpace(comment.Body), RequestPrefix) {
			continue
		}
		r, err := ParseRequest(comment.Body)
		if err != nil {
			rejected = append(rejected, Receipt{RequestID: requestID(comment.Body), Result: "rejected", Detail: "invalid request: " + err.Error(), At: time.Now().UTC()})
			continue
		}
		if comment.CreatedAt != "" && comment.UpdatedAt != "" && comment.CreatedAt != comment.UpdatedAt {
			rejected = append(rejected, Receipt{RequestID: r.ID, Result: "rejected", Detail: "request comment was edited; submit a new request", At: time.Now().UTC()})
			continue
		}
		if !receipted[r.ID] {
			pending = append(pending, PendingRequest{Request: r, CommentID: comment.ID})
		}
	}
	return pending, rejected
}

func receiptRequestIDs(body string) []string {
	const key = "request="
	var ids []string
	for _, line := range strings.Split(body, "\n") {
		if i := strings.Index(line, key); i >= 0 {
			value := strings.Fields(line[i+len(key):])
			if len(value) > 0 {
				ids = append(ids, strings.Trim(value[0], "->"))
			}
		}
	}
	return ids
}

func requestID(body string) string {
	for _, field := range strings.Fields(body) {
		if strings.HasPrefix(field, "id=") {
			return strings.Trim(strings.TrimPrefix(field, "id="), "->")
		}
	}
	return "unknown"
}

func RejectionComment(r Receipt) (string, error) {
	if r.RequestID == "" {
		return "", fmt.Errorf("receipt request id required")
	}
	return RenderReceipt(r)
}
