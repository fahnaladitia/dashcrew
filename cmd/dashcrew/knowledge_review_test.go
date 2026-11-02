package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func reviewGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// TestKnowledgeReviewStopsOnAStaleClone: a teammate's merged change on the knowledge repo makes the local clone
// behind; review stops before touching a page, and runs again after the clone catches up.
func TestKnowledgeReviewStopsOnAStaleClone(t *testing.T) {
	root := t.TempDir()
	origin, clone, other := filepath.Join(root, "origin.git"), filepath.Join(root, "clone"), filepath.Join(root, "other")
	reviewGit(t, root, "init", "-q", "--bare", "-b", "main", origin)
	reviewGit(t, root, "clone", "-q", origin, other)
	for _, dir := range []string{"inbox", "features", "repos"} {
		os.MkdirAll(filepath.Join(other, dir), 0o755)
		os.WriteFile(filepath.Join(other, dir, ".gitkeep"), nil, 0o644)
	}
	reviewGit(t, other, "add", ".")
	reviewGit(t, other, "commit", "-q", "-m", "scaffold")
	reviewGit(t, other, "push", "-q", "origin", "main")
	reviewGit(t, root, "clone", "-q", origin, clone)

	os.WriteFile(filepath.Join(other, "features", "orders.md"), []byte("---\ntitle: Orders\n---\n\n# Orders\n"), 0o644)
	reviewGit(t, other, "add", ".")
	reviewGit(t, other, "commit", "-q", "-m", "orders")
	reviewGit(t, other, "push", "-q", "origin", "main")

	review := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := run(append([]string{"knowledge", "review", "--clone", clone}, args...), &stdout, &stderr, map[string]string{})
		return code, stdout.String(), stderr.String()
	}
	if code, _, stderr := review(); code != 1 || !strings.Contains(stderr, "1 commit(s) behind its upstream; run dashcrew knowledge pull") {
		t.Errorf("stale clone: exit %d, stderr %q", code, stderr)
	}
	if code, stdout, _ := review("--no-fetch"); code != 0 || !strings.Contains(stdout, "inbox/ is empty") {
		t.Errorf("--no-fetch: exit %d, stdout %q", code, stdout)
	}
	reviewGit(t, clone, "merge", "-q", "--ff-only", "origin/main")
	if code, stdout, stderr := review(); code != 0 || stderr != "" || !strings.Contains(stdout, "inbox/ is empty") {
		t.Errorf("fresh clone: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
