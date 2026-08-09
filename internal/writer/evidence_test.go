package writer

import "testing"

func TestEvidenceRequiresCompletionRecord(t *testing.T) {
	e := Evidence{FinalSHA: "abc", CIURL: "https://ci", Commands: "go test", Environments: "CI", Criteria: "pass", Artifacts: "None: command output retained in CI", Documentation: "docs", Risks: "None", Boundary: "CI"}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Evidence{}).Validate(); err == nil {
		t.Fatal("expected incomplete evidence error")
	}
}
