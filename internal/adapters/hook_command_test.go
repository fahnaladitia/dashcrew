package adapters

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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

	runHook(t, GitGuardCommand, filepath.Join(root, "lib", "feature"))

	if got := readFile(t, root, "calls.txt"); got != "called "+filepath.Join(root, ".ai", "guard.json")+"\n" {
		t.Errorf("calls %q", got)
	}
}

// TestAntigravityHookReachesTheGuardFromItsWorkingDirectory runs the hook where agy runs it, in the folder of
// .agents/hooks.json, and from the root and a subfolder in case a build runs it elsewhere.
func TestAntigravityHookReachesTheGuardFromItsWorkingDirectory(t *testing.T) {
	entry := AntigravityGuardHook().Value("PreToolUse").([]any)[0].(*omap.Map[any])
	command := entry.Value("hooks").([]any)[0].(*omap.Map[any]).Value("command").(string)
	for _, folder := range []string{".agents", ".", "lib/feature"} {
		t.Run(folder, func(t *testing.T) {
			root, _ := hookProject(t)
			if err := os.MkdirAll(filepath.Join(root, ".agents"), 0o755); err != nil {
				t.Fatal(err)
			}

			runHook(t, command, filepath.Join(root, filepath.FromSlash(folder)))

			if got := readFile(t, root, "calls.txt"); got != "called "+filepath.Join(root, ".ai", "guard.json")+"\n" {
				t.Errorf("calls %q", got)
			}
		})
	}
}

// TestAntigravityHookFindsTheShimInAMonorepoSubfolderAndWithoutGit covers a project below the git toplevel and a
// checkout with no .git: the command tries the folder above .agents/ before git.
func TestAntigravityHookFindsTheShimInAMonorepoSubfolderAndWithoutGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hook commands run through sh")
	}
	entry := AntigravityGuardHook().Value("PreToolUse").([]any)[0].(*omap.Map[any])
	command := entry.Value("hooks").([]any)[0].(*omap.Map[any]).Value("command").(string)
	for _, gitInit := range []bool{true, false} {
		repo, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		if gitInit {
			if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
				t.Skipf("git init: %v %s", err, out)
			}
		}
		project := filepath.Join(repo, "apps", "mobile")
		calls := filepath.Join(project, "calls.txt")
		writeFile(t, project, ".ai/bin/guard", "#!/bin/sh\ncat > /dev/null\necho \"called $*\" >> \""+calls+"\"\nexit 2\n")
		if err := os.Chmod(filepath.Join(project, ".ai", "bin", "guard"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(project, ".agents"), 0o755); err != nil {
			t.Fatal(err)
		}

		runHook(t, command, filepath.Join(project, ".agents"))

		if got := readFile(t, project, "calls.txt"); got != "called "+filepath.Join(project, ".ai", "guard.json")+"\n" {
			t.Errorf("git %v: calls %q", gitInit, got)
		}
	}
}

func TestAntigravityHookDeniesWithAReasonWhenNoShimIsFound(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hook commands run through sh")
	}
	entry := AntigravityGuardHook().Value("PreToolUse").([]any)[0].(*omap.Map[any])
	command := entry.Value("hooks").([]any)[0].(*omap.Map[any]).Value("command").(string)
	dir := filepath.Join(t.TempDir(), "w", ".agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil || !strings.HasPrefix(string(out), `{"decision":"deny","reason":"[dashcrew guard] .ai/bin/guard not found`) {
		t.Errorf("exit %v, stdout %q", err, out)
	}
}

func TestAntigravityMatcherCoversEveryWriteToolOfAgy(t *testing.T) {
	entry := AntigravityGuardHook().Value("PreToolUse").([]any)[0].(*omap.Map[any])
	matcher := regexp.MustCompile(entry.Value("matcher").(string))
	for _, tool := range []string{"run_command", "write_to_file", "replace_file_content", "multi_replace_file_content",
		"create_file", "edit_file", "write_file", "file_write", "str_replace_editor", "delete_file", "file_delete"} {
		if !matcher.MatchString(tool) {
			t.Errorf("%s is not matched", tool)
		}
	}
	for _, tool := range []string{"view_file", "list_dir", "grep_search", "write_to_file_extra"} {
		if matcher.MatchString(tool) {
			t.Errorf("%s is matched", tool)
		}
	}
}
