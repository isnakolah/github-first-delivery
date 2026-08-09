package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallWritesCompleteDynamicContract(t *testing.T) {
	root := t.TempDir()
	if err := Install(root, "octo", 7, "trunk"); err != nil {
		t.Fatal(err)
	}
	for path := range files {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	for _, workflow := range []string{".github/workflows/gfd-writer.yml", ".github/workflows/gfd-policy.yml", ".github/workflows/ci.yml"} {
		content, err := os.ReadFile(filepath.Join(root, workflow))
		if err != nil || !strings.Contains(string(content), "name:") {
			t.Fatalf("workflow %s content=%q err=%v", workflow, content, err)
		}
	}
	writer, err := os.ReadFile(filepath.Join(root, ".github/workflows/gfd-writer.yml"))
	if err != nil || !strings.Contains(string(writer), "ref: trunk") || !strings.Contains(string(writer), "github.com/isnakolah/github-first-delivery/cmd/gfd@main") {
		t.Fatalf("writer default branch content=%q err=%v", writer, err)
	}
	ci, err := os.ReadFile(filepath.Join(root, ".github/workflows/ci.yml"))
	if err != nil || strings.Contains(string(ci), "./cmd/gfd") || strings.Contains(string(ci), "plugins/codex") || !strings.Contains(string(ci), "if [ -f go.mod ]") {
		t.Fatalf("target CI must not require GFD source layout: content=%q err=%v", ci, err)
	}
	config, err := os.ReadFile(filepath.Join(root, ".github/ISSUE_TEMPLATE/config.yml"))
	if err != nil || !strings.Contains(string(config), "https://github.com/users/octo/projects/7") {
		t.Fatalf("config=%q err=%v", config, err)
	}
	if err := Install(root, "octo", 7, "trunk"); err == nil {
		t.Fatal("expected overwrite refusal")
	}
}
