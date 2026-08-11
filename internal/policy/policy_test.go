package policy

import "testing"

func TestValidatePR(t *testing.T) {
	if err := ValidatePR("001/bootstrap", "Fixes #1", true, true); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePR("main", "Fixes #1", true, true); err == nil {
		t.Fatal("wanted error")
	}
}

func TestReferencedIssuesRequiresExactlyOne(t *testing.T) {
	issues, err := ReferencedIssues("Refs #7")
	if err != nil || len(issues) != 1 || issues[0] != 7 {
		t.Fatalf("issues=%v err=%v", issues, err)
	}
	if _, err := ReferencedIssues("Refs #7\nRefs #8"); err == nil {
		t.Fatal("expected multiple Issue rejection")
	}
}
