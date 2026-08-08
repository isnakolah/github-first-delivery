package writer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Fingerprint is canonical digest input. Struct fields make serialization stable.
type Fingerprint struct {
	IssueID, IssueState, UpdatedAt, ParentID string
	DependencyIDs                            []string
	Project                                  map[string]string
	PRHeadSHA, PRState                       string
	RequestIDs                               []string
}

func StateFingerprint(value Fingerprint) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
