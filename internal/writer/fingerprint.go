package writer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
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
	value.DependencyIDs = append([]string(nil), value.DependencyIDs...)
	value.RequestIDs = append([]string(nil), value.RequestIDs...)
	sort.Strings(value.DependencyIDs)
	sort.Strings(value.RequestIDs)
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func RequireFreshFingerprint(expected string, state Fingerprint) (string, error) {
	actual, err := StateFingerprint(state)
	if err != nil {
		return "", err
	}
	if expected != actual {
		return actual, fmt.Errorf("stale request fingerprint: expected %s, observed %s", expected, actual)
	}
	return actual, nil
}
