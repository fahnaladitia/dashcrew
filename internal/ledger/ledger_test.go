package ledger

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/lines"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

func gitT(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitFile(t *testing.T, dir, file, message string) string {
	t.Helper()
	write(t, filepath.Join(dir, file), file+"\n")
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", message)
	return gitT(t, dir, "rev-parse", "--short=7", "HEAD")
}

func TestStatusListsTasksPortsAndPendingPorts(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p, err := profile.Parse("toolkit: 1\nagents: [claude-code]\nproject:\n  release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)', hotfix: 'hotfix/(?P<version>[0-9.]+)' }\n",
		profile.KnownOptions{"flutter": omap.New[[]string]()})
	if err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "init", "-q", "-b", "develop")
	commitFile(t, dir, "a.txt", "chore: init")
	gitT(t, dir, "checkout", "-q", "-b", "release/2.4.0")
	totals := commitFile(t, dir, "t.txt", "fix: totals")
	gitT(t, dir, "checkout", "-q", "-b", "hotfix/2.4.5")
	crash := commitFile(t, dir, "c.txt", "fix: crash")
	gitT(t, dir, "checkout", "-q", "-b", "fix/crash-on-empty-cart")
	gitT(t, dir, "config", "branch.fix/crash-on-empty-cart.dashcrew-release", "2.4.5")
	gitT(t, dir, "checkout", "-q", "develop")
	gitT(t, dir, "checkout", "-q", "-b", "release/2.5.0")
	ported := commitFile(t, dir, "t.txt", "fix: totals\n\nPorted-from: "+totals)
	gitT(t, dir, "checkout", "-q", "develop")
	write(t, filepath.Join(dir, ".ai/plans/2.4.5/crash-on-empty-cart/plan.md"), "plan\n")
	write(t, filepath.Join(dir, ".ai/plans/2.4.5/no-branch-yet/plan.md"), "plan\n")
	write(t, filepath.Join(dir, ".ai/pr/fix/crash-on-empty-cart.md"), "pr\n")

	all, err := lines.List(dir, p.ReleaseLines)
	if err != nil {
		t.Fatal(err)
	}
	text, err := Status(dir, p, all, "2.4.5")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Release 2.4.5\n",
		"Line: `hotfix/2.4.5` (hotfix), since `release/2.4.0`.",
		"| `crash-on-empty-cart` | `fix/crash-on-empty-cart` | `.ai/pr/fix/crash-on-empty-cart.md` |",
		"| `no-branch-yet` | none found | none |",
		"## Ported here\n\nNone.",
		"- `release/2.4.0 -> hotfix/2.4.5`: none pending",
		"- `hotfix/2.4.5 -> release/2.5.0`: missing `" + crash + "` fix: crash",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	text, err = Status(dir, p, all, "2.5.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"No task folders in `.ai/plans/2.5.0/`.",
		"- `" + ported + "` fix: totals (Ported-from: " + totals + ")",
		"- `release/2.5.0 -> develop`: missing `" + ported + "` fix: totals",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	if text, err = Status(dir, p, all, "9.9.9"); err != nil || !strings.Contains(text, "No line branch holds 9.9.9") {
		t.Errorf("%v\n%s", err, text)
	}
}

func TestWriteTouchesOnlyTheReadme(t *testing.T) {
	dir := t.TempDir()
	path, err := Write(dir, ".ai/plans", "2.4.5", "text\n")
	if err != nil {
		t.Fatal(err)
	}
	if path != ".ai/plans/2.4.5/README.md" {
		t.Errorf("%s", path)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".ai/plans/2.4.5/README.md"))
	if err != nil || string(data) != "text\n" {
		t.Errorf("%q %v", data, err)
	}
	if _, err := Write(dir, ".ai/plans", "../x", "text\n"); err == nil {
		t.Error("a version with a path wrote")
	}
}
