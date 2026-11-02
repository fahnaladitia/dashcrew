package deps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/profile"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// upstream is a dependency repository on main with a tag v1.0.0 and a branch feat/x one commit ahead; it returns
// its path, a commit on main and the commit only on feat/x.
func upstream(t *testing.T) (string, string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "core")
	git(t, filepath.Dir(dir), "init", "-q", "-b", "main", dir)
	write(t, filepath.Join(dir, "a.txt"), "a\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "a")
	git(t, dir, "tag", "v1.0.0")
	onMain := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", "-b", "feat/x")
	write(t, filepath.Join(dir, "b.txt"), "b\n")
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "b")
	onBranch := git(t, dir, "rev-parse", "HEAD")
	git(t, dir, "checkout", "-q", "main")
	return dir, onMain, onBranch
}

func app(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	for name, text := range files {
		write(t, filepath.Join(dir, name), text)
	}
	return dir
}

func links(pkg, repo string) *profile.Profile {
	return &profile.Profile{Links: []profile.Link{{Package: pkg, Repo: repo}}}
}

func only(t *testing.T, results []Result) Result {
	t.Helper()
	if len(results) != 1 {
		t.Fatalf("results %#v", results)
	}
	return results[0]
}

func expect(t *testing.T, result Result, status Status, detail string) {
	t.Helper()
	if result.Status != status || !strings.Contains(result.Detail, detail) {
		t.Errorf("got %d %q, want %d containing %q", result.Status, result.Detail, status, detail)
	}
}

func TestPubGitRefs(t *testing.T) {
	core, onMain, onBranch := upstream(t)
	pubspec := func(git string) map[string]string {
		return map[string]string{"pubspec.yaml": "name: app\ndependencies:\n  shared_core:\n    git:\n" + git}
	}
	lock := func(sha string) string {
		return "packages:\n  shared_core:\n    source: git\n    description:\n      resolved-ref: \"" + sha + "\"\n"
	}
	cases := []struct {
		name   string
		files  map[string]string
		status Status
		detail string
	}{
		{"tag", pubspec("      url: " + core + "\n      ref: v1.0.0\n"), OK, "tag v1.0.0"},
		{"branch", pubspec("      url: " + core + "\n      ref: feat/x\n"), Refused, "ref feat/x in dependencies is a branch"},
		{"default branch by name", pubspec("      url: " + core + "\n      ref: main\n"), Refused, "is a branch"},
		{"commit on main", pubspec("      url: " + core + "\n      ref: " + onMain + "\n"), OK, "is on main"},
		{"commit only on a branch", pubspec("      url: " + core + "\n      ref: " + onBranch + "\n"), Refused, "is not on main"},
		{"unknown ref", pubspec("      url: " + core + "\n      ref: nope\n"), Refused, "is no tag, branch or commit"},
		{"no ref, locked on main", map[string]string{"pubspec.yaml": pubspec("      url: " + core + "\n")["pubspec.yaml"], "pubspec.lock": lock(onMain)}, OK, "is on main"},
		{"no ref, locked on a branch", map[string]string{"pubspec.yaml": pubspec("      url: " + core + "\n")["pubspec.yaml"], "pubspec.lock": lock(onBranch)}, Refused, "is not on main"},
		{"no ref, no lock", pubspec("      url: " + core + "\n"), Refused, "no ref and no locked commit"},
		{"git as a plain url", map[string]string{"pubspec.yaml": "name: app\ndependencies:\n  shared_core:\n    git: " + core + "\n", "pubspec.lock": lock(onMain)}, OK, "is on main"},
		{"unreachable", pubspec("      url: " + filepath.Join(core, "missing") + "\n      ref: v1.0.0\n"), Unreachable, "CI needs read access"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expect(t, only(t, Check(app(t, c.files), links("shared_core", ""), pubspecOverrides)), c.status, c.detail)
		})
	}
}

func TestPubOtherKinds(t *testing.T) {
	cases := []struct {
		name, deps string
		status     Status
		detail     string
	}{
		{"hosted string", "dependencies:\n  shared_core: ^1.0.0\n", OK, "hosted version"},
		{"hosted map", "dependencies:\n  shared_core:\n    hosted: https://pub.example.com\n    version: ^1.0.0\n", OK, "hosted version"},
		{"path inside", "dependencies:\n  shared_core:\n    path: packages/core\n", OK, "inside the repository"},
		{"path outside", "dependencies:\n  shared_core:\n    path: ../core\n", Refused, "leaves the repository"},
		{"override in pubspec", "dependencies:\n  shared_core: ^1.0.0\ndependency_overrides:\n  shared_core:\n    path: ../core\n", Refused, "dependency_overrides leaves"},
		{"dev dependency", "dev_dependencies:\n  shared_core:\n    path: /abs/core\n", Refused, "leaves the repository"},
		{"absent", "dependencies:\n  other: ^1.0.0\n", Absent, "not a dependency"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := app(t, map[string]string{"pubspec.yaml": "name: app\n" + c.deps})
			expect(t, only(t, Check(dir, links("shared_core", ""), pubspecOverrides)), c.status, c.detail)
		})
	}
}

func TestACommittedLinkFileIsRefused(t *testing.T) {
	dir := app(t, map[string]string{"pubspec.yaml": "name: app\ndependencies:\n  shared_core: ^1.0.0\n", "pubspec_overrides.yaml": "x: 1\n"})
	git(t, dir, "add", "pubspec_overrides.yaml")
	results := Check(dir, links("shared_core", ""), pubspecOverrides)
	if len(results) != 2 || results[0].Package != "" || results[0].Status != Refused || !strings.Contains(results[0].Detail, "pubspec_overrides.yaml is committed") {
		t.Errorf("results %#v", results)
	}
}

func TestGoModules(t *testing.T) {
	core, onMain, onBranch := upstream(t)
	const module = "example.com/acme/api"
	pseudo := func(sha string) string { return "v0.0.0-20261102000000-" + sha[:12] }
	cases := []struct {
		name, gomod string
		status      Status
		detail      string
	}{
		{"release", "require " + module + " v1.2.0\n", OK, "is a release"},
		{"pseudo on main", "require (\n\t" + module + " " + pseudo(onMain) + "\n)\n", OK, "is on main"},
		{"pseudo on a branch", "require " + module + " " + pseudo(onBranch) + " // indirect\n", Refused, "is not on main"},
		{"local replace", "require " + module + " v1.2.0\nreplace " + module + " => ../api\n", Refused, "points at a local folder"},
		{"replace block to a module", "require " + module + " v1.2.0\nreplace (\n\t" + module + " => example.com/fork/api v1.3.0\n)\n", OK, "replace => example.com/fork/api v1.3.0 is a release"},
		{"absent", "require example.com/other v1.0.0\n", Absent, "not required"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := app(t, map[string]string{"go.mod": "module example.com/acme/app\n\ngo 1.26\n\n" + c.gomod})
			expect(t, only(t, Check(dir, links(module, core), goWork)), c.status, c.detail)
		})
	}
}

func TestLeaves(t *testing.T) {
	for target, want := range map[string]bool{"packages/core": false, "./core": false, "a/../b": false, "../core": true, "a/../../core": true, "/abs": true, "": true, "..": true} {
		if got := leaves(target); got != want {
			t.Errorf("leaves(%q) = %v", target, got)
		}
	}
}
