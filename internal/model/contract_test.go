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

func TestValidateGraphRejectsBlockerCycle(t *testing.T) {
	issues := []Issue{
		{ID: "a", Number: 1, Kind: "Epic"},
		{ID: "b", Number: 2, Kind: "Story", ParentID: "a", Body: WorkBody("x", "x", "x", "x", "x", "None: x", "x", "CI"), BlockerIDs: []string{"c"}},
		{ID: "c", Number: 3, Kind: "Story", ParentID: "a", Body: WorkBody("x", "x", "x", "x", "x", "None: x", "x", "CI"), BlockerIDs: []string{"b"}},
	}
	if err := ValidateGraph(issues); err == nil {
		t.Fatal("expected blocker cycle error")
	}
}

func TestValidateGraphRejectsReadyWorkWithBlocker(t *testing.T) {
	issues := []Issue{
		{ID: "a", Number: 1, Kind: "Epic"},
		{ID: "b", Number: 2, Kind: "Story", ParentID: "a", Body: WorkBody("x", "x", "x", "x", "x", "None: x", "x", "CI"), Status: "Ready", BlockerIDs: []string{"c"}},
		{ID: "c", Number: 3, Kind: "Story", ParentID: "a", Body: WorkBody("x", "x", "x", "x", "x", "None: x", "x", "CI"), Status: "Backlog"},
	}
	if err := ValidateGraph(issues); err == nil {
		t.Fatal("expected readiness blocker error")
	}
}

func TestValidateGraphRejectsDoneParentWithIncompleteChild(t *testing.T) {
	issues := []Issue{
		{ID: "a", Number: 1, Kind: "Epic", Status: "Done"},
		{ID: "b", Number: 2, Kind: "Story", ParentID: "a", Body: WorkBody("x", "x", "x", "x", "x", "None: x", "x", "CI"), Status: "In progress"},
	}
	if err := ValidateGraph(issues); err == nil {
		t.Fatal("expected parent completion error")
	}
}

func TestValidateGraphRejectsClassificationMismatchAndEpicBranch(t *testing.T) {
	issues := []Issue{
		{ID: "a", Number: 1, Kind: "Epic", Branch: "001/nope"},
		{ID: "b", Number: 2, Kind: "Story", ParentID: "a", Body: WorkBody("x", "x", "x", "x", "x", "None: x", "x", "CI"), Area: "core", ProjectKind: "Task", ProjectArea: "delivery"},
	}
	if err := ValidateGraph(issues); err == nil {
		t.Fatal("expected classification and Epic branch errors")
	}
}

func TestValidateGraphAcceptsCompleteGraph(t *testing.T) {
	issues := []Issue{
		{ID: "a", Number: 1, Kind: "Epic", Status: "Backlog", ProjectKind: "Epic", Labels: []string{"kind:epic"}},
		{ID: "b", Number: 2, Kind: "Story", ParentID: "a", Body: WorkBody("x", "x", "x", "x", "x", "None: x", "x", "CI"), Status: "Backlog", Area: "stable", ProjectKind: "Story", ProjectArea: "stable", Labels: []string{"kind:story", "area:stable"}},
	}
	if err := ValidateGraph(issues); err != nil {
		t.Fatal(err)
	}
}
