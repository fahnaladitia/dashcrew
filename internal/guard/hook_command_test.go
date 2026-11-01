package guard_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// dartHookCommandTests is the number of tests in test/unit/hook_command_test.dart, each a row of hookCommandCases.
const dartHookCommandTests = 3

// hookProject is a git repo whose `.ai/bin/guard` is a stub that records its arguments and blocks.
func hookProject(t *testing.T) (root, calls string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	calls = filepath.Join(root, "calls.txt")
	stub := filepath.Join(root, ".ai", "bin", "guard")
	if err := os.MkdirAll(filepath.Dir(stub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stub, []byte("#!/bin/sh\ncat > /dev/null\necho \"called $*\" >> \""+calls+"\"\nexit 2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "lib", "feature"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root, calls
}

// runHook runs command through `sh -c`, as an agent does; env replaces the environment when not nil.
func runHook(t *testing.T, command, cwd string, env []string) (int, string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = cwd
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode(), string(out)
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, string(out)
}

func antigravityGuardCommand(t *testing.T) string {
	t.Helper()
	entry := adapters.AntigravityGuardHook().Value("PreToolUse").([]any)[0].(*omap.Map[any])
	return entry.Value("hooks").([]any)[0].(*omap.Map[any]).Value("command").(string)
}

var hookCommandCases = []struct {
	dart string
	run  func(t *testing.T)
}{
	{"claude code reaches the guard from a subfolder through CLAUDE_PROJECT_DIR", func(t *testing.T) {
		root, calls := hookProject(t)
		// Dart's Process.runSync merges the given environment into the parent's.
		code, out := runHook(t, adapters.ClaudeGuardCommand, filepath.Join(root, "lib", "feature"), append(os.Environ(), "CLAUDE_PROJECT_DIR="+root))
		assertCalled(t, code, out, calls, "called "+filepath.Join(root, ".ai", "guard.json")+"\n")
	}},
	{"codex reaches the guard from a subfolder through git rev-parse", func(t *testing.T) {
		root, calls := hookProject(t)
		code, out := runHook(t, adapters.CodexGuardCommand, filepath.Join(root, "lib", "feature"), nil)
		assertCalled(t, code, out, calls, "called "+filepath.Join(root, ".ai", "guard.json")+"\n")
	}},
	{"antigravity reaches the guard from the workspace root", func(t *testing.T) {
		root, calls := hookProject(t)
		code, out := runHook(t, antigravityGuardCommand(t), root, nil)
		assertCalled(t, code, out, calls, "called \n")
	}},
}

func assertCalled(t *testing.T, code int, out, calls, want string) {
	t.Helper()
	if code != 2 {
		t.Errorf("exit %d: %s", code, out)
	}
	got, err := os.ReadFile(calls)
	if err != nil || string(got) != want {
		t.Errorf("calls %q, %v; want %q", got, err, want)
	}
}

func TestEveryDartHookCommandTestIsPorted(t *testing.T) {
	if len(hookCommandCases) != dartHookCommandTests {
		t.Errorf("%d ported, want %d", len(hookCommandCases), dartHookCommandTests)
	}
	t.Logf("hook_command_test.dart: %d tests ported", len(hookCommandCases))
}

func TestHookCommands(t *testing.T) {
	for _, c := range hookCommandCases {
		t.Run(c.dart, c.run)
	}
}
