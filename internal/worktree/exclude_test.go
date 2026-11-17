package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExcludeLeavesAnExcludeFileItCannotReadAlone(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions that bind the user")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	excludePath := filepath.Join(dir, ".git", "info", "exclude")
	const mine = "# mine\n/secret.env\n"
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(excludePath, 0o200); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(excludePath, 0o644) })

	if err := Exclude(dir, []string{".ai/knowledge"}); err == nil {
		t.Error("an unreadable exclude file was written")
	}

	if err := os.Chmod(excludePath, 0o644); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(excludePath); string(data) != mine {
		t.Errorf("exclude file now %q", data)
	}
}

func TestExcludeOutsideARepositoryNamesTheGitFailure(t *testing.T) {
	dir := t.TempDir()

	err := Exclude(dir, []string{".ai/knowledge"})

	if err == nil || !strings.Contains(err.Error(), "git rev-parse") {
		t.Errorf("got %v", err)
	}
}
