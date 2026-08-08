package model

import "testing"

func TestWorkBodyValid(t *testing.T) {
	body := WorkBody("ship", "code", "binary release", "tests pass", "go test ./...", "None: no user-facing change", "Issue source", "CI")
	if err := ValidateWorkContract(body); err != nil {
		t.Fatal(err)
	}
}
func TestValidateGraphRejectsParentCycle(t *testing.T) {
	issues := []Issue{{ID: "a", Number: 1, Kind: "Epic", ParentID: "b"}, {ID: "b", Number: 2, Kind: "Epic", ParentID: "a"}}
	if err := ValidateGraph(issues); err == nil {
		t.Fatal("expected cycle error")
	}
}
