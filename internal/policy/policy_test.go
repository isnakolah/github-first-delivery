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
