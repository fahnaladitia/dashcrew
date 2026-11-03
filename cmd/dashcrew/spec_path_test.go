package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSpecPathsAreReadFromTheKnowledgeClone: in the P7 live run a member typed `dashcrew spec revise
// specs/unreleased/order-summary` from the project, as the plan's Spec line names it, and got "no such spec".
func TestSpecPathsAreReadFromTheKnowledgeClone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	project := t.TempDir()
	os.MkdirAll(filepath.Join(project, ".ai"), 0o755)
	os.WriteFile(filepath.Join(project, ".ai", "project.yaml"),
		[]byte("toolkit: 1\nagents: [claude-code]\nknowledge: { repo: git@example.com:team/shop-knowledge.git, name: shop-knowledge }\n"), 0o644)
	folder := filepath.Join(home, ".dashcrew", "knowledge", "shop-knowledge", "specs", "unreleased", "order-summary")
	os.MkdirAll(folder, 0o755)
	front := "---\ntitle: Order summary\nfeature: order-summary\nrelease: unreleased\nstatus: approved\nrev: 1\nrepos: [shop-api]\nowners: [backend]\n---\n\n"
	os.WriteFile(filepath.Join(folder, "prd.md"), []byte(front+"# Order summary: PRD\n\n- AC-1 Given a, when b, then c.\n"), 0o644)
	os.WriteFile(filepath.Join(folder, "trd.md"), []byte(front+"# Order summary: TRD\n\n## Decisions\n\n- D-1 One query.\n"), 0o644)

	call := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr, map[string]string{})
		return code, stdout.String(), stderr.String()
	}
	if code, stdout, stderr := call("spec", "lint", "-p", project, "specs/unreleased/order-summary"); code != 0 || !strings.Contains(stdout, "2 spec files checked") {
		t.Errorf("lint: exit %d\n%s%s", code, stdout, stderr)
	}
	code, stdout, stderr := call("spec", "revise", "-p", project, "specs/unreleased/order-summary/")
	if code != 0 || !strings.Contains(stdout, "approved rev 1 -> review rev 2") {
		t.Fatalf("revise: exit %d\n%s%s", code, stdout, stderr)
	}
	if data, _ := os.ReadFile(filepath.Join(folder, "trd.md")); !strings.Contains(string(data), "rev: 2") {
		t.Errorf("trd.md not revised:\n%s", data)
	}
	if code, _, stderr := call("spec", "revise", "-p", project, "specs/unreleased/missing"); code != 1 ||
		!strings.Contains(stderr, "no such spec: specs/unreleased/missing (looked in . and in the knowledge clone") {
		t.Errorf("missing spec: exit %d, %q", code, stderr)
	}
	if code, _, stderr := call("spec", "revise", "-p", project, "elsewhere/x"); code != 1 || !strings.Contains(stderr, "no such spec: elsewhere/x") {
		t.Errorf("non-specs path: exit %d, %q", code, stderr)
	}
}
