package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// The parity test runs the Dart CLI (bin/dashcrew.dart) and the Go CLI side by side on identical temp copies and
// compares stdout, stderr, the exit code and the resulting file tree of every step. Only the temp root is normalized.
//
// It needs a Dart SDK and `dart pub get` in the repository: set DASHCREW_DART to the dart executable, for example
// `DASHCREW_DART=$(command -v dart) go test ./cmd/dashcrew -run Parity`. Without it the test skips.
//
// Known parity exceptions, all until G7 switches the shim to `dashcrew guard`:
//   - `sync` with `--compile-guard` (the default): Dart compiles and installs its guard into $DASHCREW_HOME, Go
//     accepts the flag and installs nothing. Every sync step here passes `--no-compile-guard`.
//   - `guard install`: Dart compiles the guard and prints its path, Go exits 64.
//   - `guard` and `guard <policy>`: Dart reports a missing or unknown subcommand (exit 64), Go runs the guard hook.
// TestKnownParityExceptionsKeepTheirGoBehaviour pins the Go side of each.

// parityEnvironment is what both CLIs run with.
type parityEnvironment struct {
	dart, dill, packages, goBinary string
	bin                            string
}

// step is one CLI call; dir is relative to the side root ("w" when empty, "." for the root itself).
type step struct {
	args   []string
	dir    string
	stdin  string
	env    map[string]string
	before func(t *testing.T, side side)
}

type parityCase struct {
	name  string
	setup func(t *testing.T, side side)
	steps []step
}

// side is one of the two identical temp copies.
type side struct {
	root string
	env  []string
}

func (s side) path(relative string) string {
	return filepath.Join(s.root, filepath.FromSlash(relative))
}

func (s side) write(t *testing.T, relative, content string) {
	t.Helper()
	writeFile(t, s.path(relative), content)
}

func (s side) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = s.path(dir)
	cmd.Env = s.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func setUpParity(t *testing.T) parityEnvironment {
	t.Helper()
	dart := os.Getenv("DASHCREW_DART")
	if dart == "" {
		t.Skip("set DASHCREW_DART to the dart executable to compare the CLI with the Dart engine")
	}
	repo := repoRoot(t)
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := parityEnvironment{dart: dart, dill: filepath.Join(work, "dashcrew.dill"),
		packages: filepath.Join(repo, ".dart_tool", "package_config.json"), goBinary: filepath.Join(work, "dashcrew"),
		bin: filepath.Join(work, "bin")}
	if _, err := os.Stat(e.packages); err != nil {
		t.Fatalf("run dart pub get first: %v", err)
	}
	compile := exec.Command(dart, "compile", "kernel", "bin/dashcrew.dart", "-o", e.dill)
	compile.Dir = repo
	if out, err := compile.CombinedOutput(); err != nil {
		t.Fatalf("dart compile kernel: %v\n%s", err, out)
	}
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-o", e.goBinary, "./cmd/dashcrew")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	// PATH holds git and the system tools only: no dashcrew, gh, flutter or fvm, so both engines see the same machine.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(e.bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(gitPath, filepath.Join(e.bin, "git")); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e parityEnvironment) newSide(t *testing.T, root string) side {
	t.Helper()
	for _, dir := range []string{"home", "w"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	const date = "2026-01-01T00:00:00Z"
	return side{root: root, env: []string{
		"PATH=" + e.bin + ":/usr/bin:/bin",
		"HOME=" + filepath.Join(root, "home"),
		"DASHCREW_HOME=" + filepath.Join(root, "home", ".dashcrew"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_COMMITTER_DATE=" + date,
	}}
}

type outcome struct {
	code           int
	stdout, stderr string
}

func (e parityEnvironment) run(t *testing.T, s side, engine string, st step) outcome {
	t.Helper()
	var cmd *exec.Cmd
	if engine == "dart" {
		cmd = exec.Command(e.dart, append([]string{"--packages=" + e.packages, e.dill}, st.args...)...)
	} else {
		cmd = exec.Command(e.goBinary, st.args...)
	}
	dir := st.dir
	if dir == "" {
		dir = "w"
	}
	cmd.Dir = s.path(dir)
	cmd.Env = append([]string{}, s.env...)
	for key, value := range st.env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdin = strings.NewReader(st.stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return outcome{code, normalize(stdout.String(), s.root), normalize(stderr.String(), s.root)}
}

func normalize(text, root string) string { return strings.ReplaceAll(text, root, "<root>") }

// keptGitFile names the files under a .git directory the comparison reads: config, the exclude file and the
// watch state; objects, logs and the index differ by timestamps and are skipped.
func keptGitFile(relative string) bool {
	_, inGit, ok := strings.Cut(relative, ".git/")
	if !ok {
		return true
	}
	return inGit == "config" || inGit == "info/exclude" || inGit == "dashcrew-watch/state.json"
}

// snapshot maps every file, link and folder under the side root to a normalized description.
func snapshot(t *testing.T, s side) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(s.root, path)
		relative = filepath.ToSlash(relative)
		if relative == "." {
			return nil
		}
		if relative == "origin.git" || strings.HasPrefix(relative, "home/.dart") {
			return filepath.SkipDir
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			out[relative] = "link -> " + normalize(target, s.root)
		case info.IsDir():
			if strings.Contains(relative+"/", ".git/") {
				if _, inGit, _ := strings.Cut(relative+"/", ".git/"); inGit != "" && inGit != "info/" && inGit != "dashcrew-watch/" {
					return filepath.SkipDir
				}
				return nil
			}
			out[relative] = "dir"
		default:
			if !keptGitFile(relative) {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out[relative] = fmt.Sprintf("%v %s", info.Mode().Perm(), normalize(string(data), s.root))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func diffTrees(t *testing.T, label string, dart, goTree map[string]string) {
	t.Helper()
	paths := map[string]bool{}
	for path := range dart {
		paths[path] = true
	}
	for path := range goTree {
		paths[path] = true
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	for _, path := range sorted {
		d, inDart := dart[path]
		g, inGo := goTree[path]
		switch {
		case !inGo:
			t.Errorf("%s: %s only after Dart", label, path)
		case !inDart:
			t.Errorf("%s: %s only after Go", label, path)
		case d != g:
			t.Errorf("%s: %s differs\n--- dart\n%s\n--- go\n%s", label, path, d, g)
		}
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(from, path)
		target := filepath.Join(to, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

// ---- fixtures ----

func golden(t *testing.T, parts ...string) string {
	return filepath.Join(append([]string{repoRoot(t), "test", "golden"}, parts...)...)
}

// sample is a golden sample's profile with its project packs and platforms, as the conformance test sets it up.
func sample(name string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		data, err := os.ReadFile(golden(t, name, "project.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		s.write(t, "w/.ai/project.yaml", string(data))
		for _, folder := range []string{"packs", "platforms"} {
			if _, err := os.Stat(golden(t, name, folder)); err == nil {
				copyDir(t, golden(t, name, folder), s.path("w/.ai/"+folder))
			}
		}
	}
}

func initApp(t *testing.T, s side) {
	copyDir(t, golden(t, "init-app", "input"), s.path("w"))
	s.git(t, "w", "init", "-q", "-b", "main")
	s.git(t, "w", "add", "-A")
	s.git(t, "w", "commit", "-q", "-m", "init")
	for _, branch := range []string{"main", "develop", "feature/x"} {
		s.git(t, "w", "update-ref", "refs/remotes/origin/"+branch, "HEAD")
	}
}

func noPlatform(t *testing.T, s side) { s.write(t, "w/Makefile", "lint:\n") }

const goPlatformYaml = "detect: [go.mod]\ncommands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test, deps: go mod download }\n"

func projectPlatform(t *testing.T, s side) {
	s.write(t, "w/go.mod", "module example.com/app\n")
	s.write(t, "w/.ai/platforms/go/platform.yaml", goPlatformYaml)
	s.write(t, "w/.ai/platforms/go/packs/router/chi/pack.yaml", "description: chi\n")
}

func twoPlatforms(t *testing.T, s side) {
	projectPlatform(t, s)
	s.write(t, "w/pubspec.yaml", "name: app\ndependencies:\n  flutter_bloc: ^9.0.0\n  dio: ^5.0.0\n  http: ^1.0.0\n")
}

// repo is test/unit/worktree_test.dart's setUp: origin.git, a clone in w with main and develop pushed, and the
// git-ignored toolkit written in the main checkout.
func repo(profileYaml string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		if err := os.Remove(s.path("w")); err != nil {
			t.Fatal(err)
		}
		s.git(t, ".", "init", "-q", "--bare", "-b", "main", "origin.git")
		s.git(t, ".", "clone", "-q", s.path("origin.git"), "w")
		s.write(t, "w/.gitignore", "AGENTS.md\nCLAUDE.md\n.claude/\n.codex/\n.agents/\n.ai/bin/\ndocs/plans/\n")
		s.write(t, "w/.ai/project.yaml", profileYaml)
		s.write(t, "w/docs/readme.md", "docs\n")
		s.write(t, "w/lib/a.dart", "void a() {}\n")
		s.write(t, "w/Makefile", "deps:\n")
		s.git(t, "w", "add", "-A")
		s.git(t, "w", "commit", "-q", "-m", "init")
		s.git(t, "w", "push", "-q", "origin", "main")
		s.git(t, "w", "push", "-q", "origin", "main:develop")
		s.git(t, "w", "fetch", "-q", "origin")
		s.write(t, "w/AGENTS.md", "rules\n")
		s.write(t, "w/CLAUDE.md", "@AGENTS.md\n")
		s.write(t, "w/.claude/settings.json", "{}\n")
		s.write(t, "w/.codex/hooks.json", "{}\n")
		s.write(t, "w/.agents/skills/plan/SKILL.md", "plan\n")
		s.write(t, "w/.ai/bin/guard", "binary\n")
		s.write(t, "w/docs/plans/x/plan.md", "plan\n")
	}
}

const repoProfile = "toolkit: 1\nagents: [claude-code]\npaths: { plans: docs/plans }\n"

func cli(args ...string) step { return step{args: args} }

func usageSteps() []step {
	lines := [][]string{
		{}, {"--help"}, {"-h"}, {"help"}, {"help", "help"}, {"help", "init"}, {"help", "sync"}, {"help", "doctor"},
		{"help", "check"}, {"help", "worktree"}, {"help", "worktree", "create"}, {"help", "guard"}, {"help", "guard", "install"},
		{"help", "nope"}, {"help", "sync", "extra"}, {"help", "worktree", "nope"}, {"help", "-h"},
		{"init", "--help"}, {"sync", "-h"}, {"doctor", "--help"}, {"check", "--help"}, {"worktree", "--help"},
		{"worktree", "create", "--help"}, {"worktree", "list", "--help"}, {"worktree", "link", "--help"},
		{"worktree", "remove", "--help"}, {"worktree", "watch", "--help"}, {"guard", "--help"}, {"guard", "-h"},
		{"guard", "install", "--help"}, {"guard", "install", "-h"},
		{"--help", "sync"}, {"-h", "worktree", "create"}, {"worktree", "-h", "create"}, {"sync", "--help", "extra"},
		{"nope"}, {"snyc"}, {"worktre"}, {"sink"}, {"chek"}, {"worktree"}, {"worktree", "nope"}, {"worktree", "craete"},
		{"guard", "install", "extra", "--nope"},
		{"sync", "--bogus"}, {"sync", "-x"}, {"sync", "-xy"}, {"init", "-yp"}, {"init", "-yz"}, {"init", "-yfoo"},
		{"init", "--agents", "bogus"}, {"init", "--agents", "codex,bogus"}, {"init", "--agents"}, {"init", "--no-yes"},
		{"sync", "--no-project"}, {"sync", "--no-bogus"}, {"doctor", "-p"}, {"sync", "--force=1"}, {"sync", "--help=1"},
		{"--", "sync"}, {"foo", "sync"}, {"help", "--project", "x"}, {"--project", "x"}, {"---x"}, {"--"}, {"-"},
		{"worktree", "create"}, {"worktree", "create", "-p"}, {"worktree", "list"},
		{"worktree", "watch", "--interval", "0"}, {"check", "--pr-body"}, {"check", "--pr-body", "missing.md"},
		{"check", "--pr-body", ""}, {"check", "--pr-body", "."},
	}
	steps := make([]step, len(lines))
	for i, args := range lines {
		steps[i] = cli(args...)
	}
	return steps
}

// toolkitSteps are the README's sync, doctor and check calls on a synced project, a PR body missing a section and
// one with every section, and the GitHub Actions annotations.
func toolkitSteps() []step {
	return []step{
		cli("sync", "--no-compile-guard"),
		cli("sync", "--no-compile-guard", "-p", "."),
		{args: []string{"sync", "--no-compile-guard", "--project", "w"}, dir: "."},
		cli("doctor"),
		{args: []string{"doctor", "-p", "w"}, dir: "."},
		cli("check"),
		{args: []string{"check", "--pr-body", "../body.md"}, before: func(t *testing.T, s side) { s.write(t, "body.md", "## Summary\n") }},
		{args: []string{"check", "--pr-body", "../body.md"}, env: map[string]string{"GITHUB_ACTIONS": "true"}},
		{args: []string{"check", "-p", ".", "--pr-body", "../full.md"}, before: func(t *testing.T, s side) {
			data, err := os.ReadFile(s.path("w/.github/pull_request_template.md"))
			if err != nil {
				t.Fatal(err)
			}
			s.write(t, "full.md", string(data))
		}},
	}
}

func appendTo(path, text string) func(t *testing.T, s side) {
	return func(t *testing.T, s side) {
		data, err := os.ReadFile(s.path(path))
		if err != nil {
			t.Fatal(err)
		}
		s.write(t, path, string(data)+text)
	}
}

func parityCases() []parityCase {
	cases := []parityCase{
		{name: "usage", steps: usageSteps()},
		{name: "init-app yes, refused, forced with agents", setup: initApp, steps: []step{
			cli("init", "--yes"), cli("init", "--yes"),
			cli("init", "-y", "--force", "--agents", "codex", "--agents", "antigravity"),
			cli("init", "-y", "--force", "--agents=codex,antigravity"),
		}},
		{name: "init-app from the parent folder", setup: initApp, steps: []step{
			{args: []string{"init", "-p", "w", "-y"}, dir: "."},
			{args: []string{"init", "--project=w/", "--yes", "--force"}, dir: "."},
		}},
		{name: "init-app interactive", setup: initApp, steps: []step{
			{args: []string{"init"}, stdin: "\n\n\ndio\n-\nnope\nother chopper\n\n\n\n\n\n\n\n\n"},
			{args: []string{"init", "--force"}, stdin: "go\n\r\nflutter\r\n"},
			{args: []string{"init", "--force"}},
		}},
		{name: "init-app synced", setup: initApp, steps: append([]step{cli("init", "--yes")}, toolkitSteps()...)},
		{name: "init without a platform", setup: noPlatform, steps: []step{
			cli("init", "--yes"),
			{args: []string{"init"}, stdin: "\nMakefile\n"},
			{args: []string{"init"}, stdin: "flutter\n"},
			{args: []string{"init"}, stdin: "nope\n\nMakefile  go.mod\nmake lint\n\nmake test\nmake one\nmake deps\nmake fmt\n"},
			cli("sync", "--no-compile-guard"),
			cli("doctor"),
		}},
		{name: "init with a project platform", setup: projectPlatform, steps: []step{
			cli("init", "--yes", "--agents", "codex"),
			{args: []string{"init", "--force"}, stdin: "\nchi\n\n\n"},
		}},
		{name: "init with two platforms", setup: twoPlatforms, steps: []step{
			cli("init", "-y"),
			{args: []string{"init"}},
			{args: []string{"init"}, stdin: "\ngo\n"},
			{args: []string{"init", "--force"}, stdin: "flutter\n"},
		}},
		{name: "no profile", steps: []step{
			cli("sync", "--no-compile-guard"), cli("doctor"), cli("check"), cli("worktree", "list"),
			cli("worktree", "create", "feat/a", "main"),
		}},
		{name: "adopt and force", setup: func(t *testing.T, s side) {
			s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code, codex]\n")
			s.write(t, "w/AGENTS.md", "# Team rules\n\nKeep it short.\n")
		}, steps: []step{
			cli("sync", "--no-compile-guard"),
			cli("sync", "--no-compile-guard", "--adopt"),
			{args: []string{"sync", "--no-compile-guard"}, before: appendTo("w/CLAUDE.md", "edited\n")},
			cli("doctor"),
			cli("sync", "--no-compile-guard", "--force"),
			{args: []string{"sync", "--no-compile-guard"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n")
			}},
			{args: []string{"sync", "--no-compile-guard"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [nope]\n")
			}},
			cli("doctor"),
			cli("check"),
		}},
		{name: "migrate p3", setup: func(t *testing.T, s side) { copyDir(t, golden(t, "sample-migrate-p3", "p3"), s.path("w")) },
			steps: []step{cli("doctor"), cli("sync", "--no-compile-guard"), cli("sync", "--no-compile-guard"), cli("doctor")}},
		{name: "worktree create, list, link, remove", setup: repo(repoProfile), steps: []step{
			cli("worktree", "list"),
			cli("worktree", "create", "feat/a", "origin/develop", "--no-pub"),
			cli("worktree", "list"),
			cli("worktree", "create", "feat/b", "main", "--name", "b-dir"),
			cli("worktree", "create", "feat/a", "main", "--no-pub"),
			cli("worktree", "create", "feat/c", "main", "--no-pub", "--name", "b-dir"),
			cli("worktree", "create", "bad..name", "main"),
			cli("worktree", "create", "feat/c", "origin/nope"),
			cli("worktree", "create", "feat/c"),
			{args: []string{"worktree", "create", "feat/c", "main", "-p", "w.worktrees/feat-a", "--no-pub"}, dir: "."},
			{args: []string{"worktree", "list", "-p", "lib"}, before: func(t *testing.T, s side) {
				os.Remove(s.path("w.worktrees/feat-a/CLAUDE.md"))
				s.write(t, "w.worktrees/feat-a/lib/a.dart", "void a() { }\n")
				s.git(t, "w.worktrees/b-dir", "branch", "--set-upstream-to=origin/main")
			}},
			cli("worktree", "link", "feat-a"),
			cli("worktree", "link", "b-dir"),
			cli("worktree", "link", "feat-a"),
			cli("worktree", "link"),
			cli("worktree", "link", "nope"),
			cli("worktree", "remove", "feat-a"),
			cli("worktree", "remove", "b-dir"),
			cli("worktree", "remove", "nope"),
			cli("worktree", "list"),
		}},
		{name: "worktree deps through the platform command", setup: repo(
			"toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make one, deps: touch deps-ran, format: null } }\n"),
			steps: []step{
				cli("worktree", "create", "feat/a", "main"),
				{args: []string{"worktree", "create", "feat/b", "main", "--pub"}, before: func(t *testing.T, s side) {
					s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config: { detect: [lib/a.dart], commands: { analyze: a, test: b, test_scoped: c, deps: exit 3 } }\n")
				}},
				{args: []string{"worktree", "create", "feat/c", "main"}, before: func(t *testing.T, s side) {
					s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nstack: { state: nope }\n")
				}},
				{args: []string{"worktree", "create", "feat/d", "main"}, before: func(t *testing.T, s side) {
					s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\nplatform: go\n")
					s.write(t, "w/.ai/platforms/go/platform.yaml", goPlatformYaml)
				}},
				{args: []string{"worktree", "create", "feat/e", "main"}, before: func(t *testing.T, s side) {
					os.Remove(s.path("w/.ai/project.yaml"))
					s.write(t, "w/pubspec.yaml", "name: app\n")
					s.git(t, "w", "add", "pubspec.yaml")
					s.git(t, "w", "commit", "-q", "-m", "pubspec")
				}},
			}},
		{name: "worktree watch", setup: repo(repoProfile), steps: []step{
			cli("worktree", "watch", "--once"),
			cli("worktree", "create", "feat/a", "origin/main", "--no-pub"),
			cli("worktree", "create", "feat/b", "main", "--no-pub"),
			cli("worktree", "watch", "--once"),
			cli("worktree", "watch", "--once", "feat-x"),
			cli("worktree", "watch", "--interval", "abc", "--once"),
			{args: []string{"worktree", "watch", "--once", "--interval", " 0x10 "}, before: func(t *testing.T, s side) {
				s.write(t, "w.worktrees/feat-a/lib/b.dart", "\n")
				s.git(t, "w.worktrees/feat-a", "add", "-A")
				s.git(t, "w.worktrees/feat-a", "commit", "-q", "-m", "b")
			}},
			{args: []string{"worktree", "watch", "--once", "feat-a"}, before: func(t *testing.T, s side) {
				s.git(t, "w.worktrees/feat-a", "push", "-q", "origin", "feat/a")
			}},
			{args: []string{"worktree", "watch", "--once"}, before: func(t *testing.T, s side) {
				s.write(t, "w.worktrees/feat-a/lib/a.dart", "void a() { 1; }\n")
				s.write(t, "w.worktrees/feat-b/lib/a.dart", "void a() { 2; }\n")
			}},
			{args: []string{"worktree", "watch", "--once", "--state-dir", "../state"}, before: func(t *testing.T, s side) {
				s.write(t, "w/lib/a.dart", "void a() { /* base */ }\n")
				s.git(t, "w", "commit", "-q", "-am", "base")
				s.git(t, "w", "push", "-q", "origin", "main")
			}},
			{args: []string{"worktree", "watch", "--once"}, before: func(t *testing.T, s side) {
				s.git(t, "w.worktrees/feat-a", "commit", "-q", "-am", "branch")
				s.write(t, "w/lib/c.dart", "\n")
				s.git(t, "w", "add", "-A")
				s.git(t, "w", "commit", "-q", "-m", "base 2")
				s.git(t, "w", "push", "-q", "origin", "main")
			}},
			{args: []string{"worktree", "watch", "--once", "feat-a", "feat-b"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "push", "-q", "origin", "--delete", "develop")
				s.git(t, "w.worktrees/feat-b", "checkout", "-q", "--", ".")
			}},
			{args: []string{"worktree", "watch", "--once"}, before: func(t *testing.T, s side) {
				s.git(t, "w", "worktree", "remove", "--force", s.path("w.worktrees/feat-a"))
				s.git(t, "w", "worktree", "remove", "--force", s.path("w.worktrees/feat-b"))
			}},
		}},
	}
	for _, name := range []string{"sample-bloc-getx", "sample-project-pack", "sample-riverpod-gorouter", "sample-codex",
		"sample-antigravity", "sample-other", "sample-project-platform"} {
		cases = append(cases, parityCase{name: name, setup: sample(name), steps: toolkitSteps()})
	}
	return cases
}

func TestCLIParityWithTheDartEngine(t *testing.T) {
	e := setUpParity(t)
	calls := 0
	for _, c := range parityCases() {
		calls += len(c.steps)
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			sides := map[string]side{}
			for _, engine := range []string{"dart", "go"} {
				s := e.newSide(t, filepath.Join(root, engine))
				if c.setup != nil {
					c.setup(t, s)
				}
				sides[engine] = s
			}
			for i, st := range c.steps {
				label := fmt.Sprintf("step %d %q", i+1, st.args)
				results := map[string]outcome{}
				for _, engine := range []string{"dart", "go"} {
					if st.before != nil {
						st.before(t, sides[engine])
					}
					results[engine] = e.run(t, sides[engine], engine, st)
				}
				d, g := results["dart"], results["go"]
				if d.code != g.code {
					t.Errorf("%s: exit code dart %d, go %d", label, d.code, g.code)
				}
				if d.stdout != g.stdout {
					t.Errorf("%s: stdout differs\n--- dart\n%s\n--- go\n%s", label, d.stdout, g.stdout)
				}
				if d.stderr != g.stderr {
					t.Errorf("%s: stderr differs\n--- dart\n%s\n--- go\n%s", label, d.stderr, g.stderr)
				}
				diffTrees(t, label, snapshot(t, sides["dart"]), snapshot(t, sides["go"]))
			}
		})
	}
	t.Logf("%d cases, %d calls per engine", len(parityCases()), calls)
}

// TestKnownParityExceptionsKeepTheirGoBehaviour pins the Go side of the exceptions listed at the top of this file.
func TestKnownParityExceptionsKeepTheirGoBehaviour(t *testing.T) {
	project := t.TempDir()
	writeFile(t, filepath.Join(project, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\n")
	home := t.TempDir()
	t.Setenv("DASHCREW_HOME", home)

	for _, args := range [][]string{{"sync", "-p", project}, {"sync", "--compile-guard", "-p", project}} {
		if code, _, stderr := dashcrew(t, nil, args...); code != 0 {
			t.Errorf("%v: exit %d, %s", args, code, stderr)
		}
	}
	if entries, _ := os.ReadDir(home); len(entries) != 0 {
		t.Errorf("sync installed into DASHCREW_HOME: %v", entries)
	}
	for _, args := range [][]string{{"guard", "install"}, {"guard", "install", "--expect", "x", "--force"}} {
		if code, stdout, stderr := dashcrew(t, nil, args...); code != 64 || stdout != "" ||
			stderr != "dashcrew guard install is served by the Dart engine until the shim calls dashcrew guard.\n" {
			t.Errorf("%v: exit %d, stdout %q, stderr %q", args, code, stdout, stderr)
		}
	}
	stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`)
	t.Cleanup(func() { stdin = os.Stdin })
	if code, stdout, stderr := dashcrew(t, nil, "guard", filepath.Join(project, "missing.json")); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("guard hook: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
}
