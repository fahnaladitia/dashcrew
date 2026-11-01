package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The CLI transcript test runs the README's commands through the built binary in temp folders and compares, per
// step, the exit code, stdout, stderr and the files the step added, changed or removed with a recorded transcript
// in testdata/transcripts/. Only the temp root is normalized, to <root>. The transcripts were recorded while the
// output was byte-identical to the Dart engine (G8), so they pin the help text, messages and exit codes.
//
// Rewrite them after an intended change with `DASHCREW_UPDATE_GOLDENS=1 go test ./cmd/dashcrew -run Transcript`,
// then review the diff.

// cliEnvironment is the binary under test and the PATH folder it runs with.
type cliEnvironment struct {
	binary, bin string
}

// step is one CLI call; dir is relative to the side root ("w" when empty, "." for the root itself).
type step struct {
	args   []string
	dir    string
	stdin  string
	env    map[string]string
	before func(t *testing.T, side side)
}

type cliCase struct {
	name  string
	setup func(t *testing.T, side side)
	steps []step
}

// side is the temp folder a case runs in.
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

func setUpCLI(t *testing.T) cliEnvironment {
	t.Helper()
	repo := repoRoot(t)
	work, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := cliEnvironment{binary: filepath.Join(work, "dashcrew"), bin: filepath.Join(work, "bin")}
	// -buildvcs=false keeps the module version unstamped, so the binary reports and locks version.Unstamped.
	build := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "build", "-buildvcs=false", "-o", e.binary, "./cmd/dashcrew")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	// PATH holds git and the system tools only: no dashcrew, gh, flutter or fvm.
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

func (e cliEnvironment) newSide(t *testing.T, root string) side {
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
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_COMMITTER_DATE=" + date,
	}}
}

type outcome struct {
	code           int
	stdout, stderr string
}

func (e cliEnvironment) run(t *testing.T, s side, st step) outcome {
	t.Helper()
	cmd := exec.Command(e.binary, st.args...)
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
		if relative == "origin.git" {
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

// repo is internal/worktree's fixture: origin.git, a clone in w with main and develop pushed, and the
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
		{"--version"}, {"--version", "sync"}, {"sync", "--version"}, {"help", "--version"}, {"--version=1"}, {"--no-version"},
		{"help", "nope"}, {"help", "sync", "extra"}, {"help", "worktree", "nope"}, {"help", "-h"},
		{"init", "--help"}, {"sync", "-h"}, {"doctor", "--help"}, {"check", "--help"}, {"worktree", "--help"},
		{"worktree", "create", "--help"}, {"worktree", "list", "--help"}, {"worktree", "link", "--help"},
		{"worktree", "remove", "--help"}, {"worktree", "watch", "--help"}, {"guard", "--help"}, {"guard", "-h"},
		{"--help", "sync"}, {"-h", "worktree", "create"}, {"worktree", "-h", "create"}, {"sync", "--help", "extra"},
		{"nope"}, {"snyc"}, {"worktre"}, {"sink"}, {"chek"}, {"worktree"}, {"worktree", "nope"}, {"worktree", "craete"},
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
		cli("sync"),
		cli("sync", "-p", "."),
		{args: []string{"sync", "--project", "w"}, dir: "."},
		cli("sync", "--compile-guard"),
		cli("sync", "--no-compile-guard"),
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

func cliCases() []cliCase {
	cases := []cliCase{
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
			cli("sync"),
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
			cli("sync"), cli("doctor"), cli("check"), cli("worktree", "list"),
			cli("worktree", "create", "feat/a", "main"),
		}},
		{name: "adopt and force", setup: func(t *testing.T, s side) {
			s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code, codex]\n")
			s.write(t, "w/AGENTS.md", "# Team rules\n\nKeep it short.\n")
		}, steps: []step{
			cli("sync"),
			cli("sync", "--adopt"),
			{args: []string{"sync"}, before: appendTo("w/CLAUDE.md", "edited\n")},
			cli("doctor"),
			cli("sync", "--force"),
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [claude-code]\npolicy: { plan_gate: true }\n")
			}},
			{args: []string{"sync"}, before: func(t *testing.T, s side) {
				s.write(t, "w/.ai/project.yaml", "toolkit: 1\nagents: [nope]\n")
			}},
			cli("doctor"),
			cli("check"),
		}},
		{name: "migrate p3", setup: func(t *testing.T, s side) { copyDir(t, golden(t, "sample-migrate-p3", "p3"), s.path("w")) },
			steps: []step{cli("doctor"), cli("sync"), cli("sync"), cli("doctor")}},
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
		cases = append(cases, cliCase{name: name, setup: sample(name), steps: toolkitSteps()})
	}
	return cases
}

// transcriptPath is the recorded transcript of a case.
func transcriptPath(name string) string {
	slug := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, strings.ToLower(name))
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	return filepath.Join("testdata", "transcripts", strings.Trim(slug, "-")+".txt")
}

// describeStep is the transcript header of a step: the arguments, then the folder, stdin and environment it ran with.
func describeStep(i int, st step) string {
	var b strings.Builder
	fmt.Fprintf(&b, "=== step %d: dashcrew", i+1)
	for _, arg := range st.args {
		b.WriteString(" " + strconv.Quote(arg))
	}
	b.WriteString("\n")
	if st.dir != "" {
		fmt.Fprintf(&b, "dir: %s\n", st.dir)
	}
	if st.stdin != "" {
		fmt.Fprintf(&b, "stdin: %s\n", strconv.Quote(st.stdin))
	}
	keys := make([]string, 0, len(st.env))
	for key := range st.env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "env: %s=%s\n", key, st.env[key])
	}
	if st.before != nil {
		b.WriteString("(after the case's setup for this step)\n")
	}
	return b.String()
}

// block is a labelled stream, each line prefixed with "| " so empty lines and missing final newlines stay visible.
func block(label, text string) string {
	if text == "" {
		return label + ": (empty)\n"
	}
	var b strings.Builder
	b.WriteString(label + ":\n")
	body := strings.TrimSuffix(text, "\n")
	for _, line := range strings.Split(body, "\n") {
		b.WriteString("| " + line + "\n")
	}
	if !strings.HasSuffix(text, "\n") {
		b.WriteString("(no final newline)\n")
	}
	return b.String()
}

// entry describes one snapshot value briefly: a folder, a link with its target, or a file's mode and content hash.
func entry(value string) string {
	if value == "dir" || strings.HasPrefix(value, "link -> ") {
		return value
	}
	mode, content, _ := strings.Cut(value, " ")
	return fmt.Sprintf("%s sha256:%x", mode, sha256.Sum256([]byte(content)))[:len(mode)+24]
}

// lineChanges lists the lines a step removed from (-) and added to (+) a file, as a multiset difference in file
// order. Files under .git use it instead of a hash, because git's own initial content (the default config, the
// exclude template) differs between git versions and platforms; the lines the step changed do not.
func lineChanges(old, value string) string {
	_, oldContent, _ := strings.Cut(old, " ")
	_, content, _ := strings.Cut(value, " ")
	lines := func(text string) []string {
		if text == "" {
			return nil
		}
		return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	}
	count := func(text string) map[string]int {
		out := map[string]int{}
		for _, line := range lines(text) {
			out[line]++
		}
		return out
	}
	var b strings.Builder
	added, removed := count(content), count(oldContent)
	for _, line := range lines(oldContent) {
		if added[line] > 0 {
			added[line]--
			continue
		}
		fmt.Fprintf(&b, "    - %q\n", line)
	}
	for _, line := range lines(content) {
		if removed[line] > 0 {
			removed[line]--
			continue
		}
		fmt.Fprintf(&b, "    + %q\n", line)
	}
	return b.String()
}

// treeChanges lists what a step added (+), changed (~) and removed (-), in path order.
func treeChanges(before, after map[string]string) string {
	paths := map[string]bool{}
	for path := range before {
		paths[path] = true
	}
	for path := range after {
		paths[path] = true
	}
	sorted := make([]string, 0, len(paths))
	for path := range paths {
		sorted = append(sorted, path)
	}
	sort.Strings(sorted)
	var b strings.Builder
	for _, path := range sorted {
		old, inBefore := before[path]
		value, inAfter := after[path]
		switch {
		case !inBefore && inGitDir(path) && value != "dir":
			fmt.Fprintf(&b, "+ %s\n%s", path, lineChanges(" ", value))
		case !inBefore:
			fmt.Fprintf(&b, "+ %s  %s\n", path, entry(value))
		case !inAfter:
			fmt.Fprintf(&b, "- %s\n", path)
		case old != value && inGitDir(path) && value != "dir":
			fmt.Fprintf(&b, "~ %s\n%s", path, lineChanges(old, value))
		case old != value:
			fmt.Fprintf(&b, "~ %s  %s\n", path, entry(value))
		}
	}
	if b.Len() == 0 {
		return "files: (unchanged)\n"
	}
	return "files:\n" + b.String()
}

// inGitDir is true for a path inside a .git folder.
func inGitDir(path string) bool { return strings.Contains(path, ".git/") }

// transcript runs every step of c in a fresh side and returns what the steps printed and changed.
func (e cliEnvironment) transcript(t *testing.T, c cliCase) string {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := e.newSide(t, root)
	if c.setup != nil {
		c.setup(t, s)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n", c.name)
	for i, st := range c.steps {
		if st.before != nil {
			st.before(t, s)
		}
		before := snapshot(t, s)
		got := e.run(t, s, st)
		b.WriteString("\n" + describeStep(i, st))
		fmt.Fprintf(&b, "exit: %d\n", got.code)
		b.WriteString(block("stdout", got.stdout))
		b.WriteString(block("stderr", got.stderr))
		b.WriteString(treeChanges(before, snapshot(t, s)))
	}
	return b.String()
}

func TestCLITranscripts(t *testing.T) {
	e := setUpCLI(t)
	update := os.Getenv("DASHCREW_UPDATE_GOLDENS") == "1"
	calls := 0
	seen := map[string]string{}
	for _, c := range cliCases() {
		calls += len(c.steps)
		path := transcriptPath(c.name)
		if other, ok := seen[path]; ok {
			t.Fatalf("cases %q and %q share %s", other, c.name, path)
		}
		seen[path] = c.name
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := e.transcript(t, c)
			if update {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v; record it with DASHCREW_UPDATE_GOLDENS=1 go test ./cmd/dashcrew -run Transcript", err)
			}
			if got != string(want) {
				t.Errorf("transcript differs from %s; if the change is intended, rewrite it with "+
					"DASHCREW_UPDATE_GOLDENS=1 go test ./cmd/dashcrew -run Transcript and review the diff\n%s",
					path, firstDifference(string(want), got))
			}
		})
	}
	t.Logf("%d cases, %d calls", len(cliCases()), calls)
}

// firstDifference shows the first differing line of two transcripts with a few lines of context.
func firstDifference(want, got string) string {
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wantLines) || i < len(gotLines); i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			start := max(0, i-6)
			context := strings.Join(wantLines[start:min(i, len(wantLines))], "\n")
			return fmt.Sprintf("line %d, after:\n%s\n--- recorded\n%s\n--- now\n%s", i+1, context, w, g)
		}
	}
	return "(no line differs)"
}
