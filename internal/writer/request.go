package writer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const RequestPrefix = "<!-- gfd-request:v1"
const ReceiptPrefix = "<!-- gfd-receipt:v1"

type Request struct {
	ID                  string    `json:"id"`
	Action              string    `json:"action"`
	IssueID             string    `json:"issue_id"`
	Actor               string    `json:"actor"`
	ExpectedFingerprint string    `json:"expected_fingerprint"`
	LeaseExpiresAt      string    `json:"lease_expires_at,omitempty"`
	Branch              string    `json:"branch,omitempty"`
	Status              string    `json:"status,omitempty"`
	PR                  string    `json:"pr,omitempty"`
	Evidence            *Evidence `json:"evidence,omitempty"`
}
type Evidence struct{ FinalSHA, CIURL, Commands, Environments, Criteria, Artifacts, Documentation, Risks, Boundary string }

func (e Evidence) Validate() error {
	if e.FinalSHA == "" || e.CIURL == "" || e.Commands == "" || e.Environments == "" || e.Criteria == "" || e.Artifacts == "" || e.Documentation == "" || e.Risks == "" || e.Boundary == "" {
		return fmt.Errorf("evidence requires final SHA, CI URL, commands, environments, criteria, artifacts, documentation, risks, and proof boundary")
	}
	return nil
}

type Receipt struct {
	RequestID   string    `json:"request_id"`
	Fingerprint string    `json:"fingerprint"`
	Result      string    `json:"result"`
	Detail      string    `json:"detail"`
	Evidence    *Evidence `json:"evidence,omitempty"`
	At          time.Time `json:"at"`
}

func HashRequest(r Request) (string, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func RenderRequest(r Request) (string, error) {
	hash, err := HashRequest(r)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("<!-- gfd-request:v1 id=%s sha256=%s -->\n```json\n%s\n```", r.ID, hash, payload), nil
}
func RenderReceipt(r Receipt) (string, error) {
	raw, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("<!-- gfd-receipt:v1 request=%s fingerprint=%s -->\n```json\n%s\n```", r.RequestID, r.Fingerprint, raw), nil
}

func ParseRequest(comment string) (Request, error) {
	var r Request
	if !strings.HasPrefix(strings.TrimSpace(comment), RequestPrefix) {
		return r, fmt.Errorf("not a gfd request")
	}
	start, end := strings.Index(comment, "```json"), strings.LastIndex(comment, "```")
	if start < 0 || end <= start {
		return r, fmt.Errorf("request JSON missing")
	}
	payload := strings.TrimSpace(comment[start+len("```json") : end])
	if err := json.Unmarshal([]byte(payload), &r); err != nil {
		return r, err
	}
	if r.ID == "" || r.Action == "" || r.IssueID == "" || r.Actor == "" || r.ExpectedFingerprint == "" {
		return r, fmt.Errorf("request has required fields missing")
	}
	hash, err := HashRequest(r)
	if err != nil {
		return r, err
	}
	marker := fmt.Sprintf("id=%s sha256=%s", r.ID, hash)
	if !strings.Contains(comment, marker) {
		return r, fmt.Errorf("request hash mismatch")
	}
	return r, nil
}
