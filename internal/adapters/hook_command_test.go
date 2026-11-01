package adapters

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// hookProject is a git repository whose .ai/bin/guard records its arguments and exits 2.
func hookProject(t *testing.T) (root, calls string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("hook commands run through sh")
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	calls = filepath.Join(root, "calls.txt")
	writeFile(t, root, ".ai/bin/guard", "#!/bin/sh\ncat > /dev/null\necho \"called $*\" >> \""+calls+"\"\nexit 2\n")
	if err := os.Chmod(filepath.Join(root, ".ai", "bin", "guard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "lib", "feature"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, calls
}

func runHook(t *testing.T, command, cwd string, env ...string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("exit %v: %s", err, out)
	}
}

func TestClaudeCodeHookReachesTheGuardFromASubfolder(t *testing.T) {
	root, calls := hookProject(t)

	runHook(t, ClaudeGuardCommand, filepath.Join(root, "lib", "feature"), "CLAUDE_PROJECT_DIR="+root)

	if got := readFile(t, root, "calls.txt"); got != "called "+filepath.Join(root, ".ai", "guard.json")+"\n" {
		t.Errorf("calls %q (%s)", got, calls)
	}
}

func TestCodexHookReachesTheGuardFromASubfolder(t *testing.T) {
	root, _ := hookProject(t)

	runHook(t, CodexGuardCommand, filepath.Join(root, "lib", "feature"))

	if got := readFile(t, root, "calls.txt"); got != "called "+filepath.Join(root, ".ai", "guard.json")+"\n" {
		t.Errorf("calls %q", got)
	}
}

func TestAntigravityHookReachesTheGuardFromTheWorkspaceRoot(t *testing.T) {
	root, _ := hookProject(t)
	entry := AntigravityGuardHook().Value("PreToolUse").([]any)[0].(*omap.Map[any])
	command := entry.Value("hooks").([]any)[0].(*omap.Map[any]).Value("command").(string)

	runHook(t, command, root)

	if got := readFile(t, root, "calls.txt"); got != "called \n" {
		t.Errorf("calls %q", got)
	}
}
