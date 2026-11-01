package worktree

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
)

// fixture is test/unit/worktree_test.dart's setUp: an origin, a clone with main and develop pushed, and the
// git-ignored toolkit written in the main checkout.
type fixture struct {
	root, app string
	worktrees *Worktrees
	output    []string
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", message)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", "origin.git")
	gitIn(t, root, "clone", "-q", filepath.Join(root, "origin.git"), "app")
	app := filepath.Join(root, "app")
	writeFile(t, filepath.Join(app, ".gitignore"), "AGENTS.md\nCLAUDE.md\n.claude/\n.codex/\n.agents/\n.ai/bin/\ndocs/plans/\n")
	writeFile(t, filepath.Join(app, ".ai", "project.yaml"), "toolkit: 1\nagents: [claude-code]\npaths: { plans: docs/plans }\n")
	writeFile(t, filepath.Join(app, "docs", "readme.md"), "docs\n")
	writeFile(t, filepath.Join(app, "lib", "a.dart"), "void a() {}\n")
	commitAll(t, app, "init")
	gitIn(t, app, "push", "-q", "origin", "main")
	gitIn(t, app, "push", "-q", "origin", "main:develop")
	gitIn(t, app, "fetch", "-q", "origin")
	writeFile(t, filepath.Join(app, "AGENTS.md"), "rules\n")
	writeFile(t, filepath.Join(app, "CLAUDE.md"), "@AGENTS.md\n")
	writeFile(t, filepath.Join(app, ".claude", "settings.json"), "{}\n")
	writeFile(t, filepath.Join(app, ".codex", "hooks.json"), "{}\n")
	writeFile(t, filepath.Join(app, ".agents", "skills", "plan", "SKILL.md"), "plan\n")
	writeFile(t, filepath.Join(app, ".ai", "bin", "guard"), "binary\n")
	writeFile(t, filepath.Join(app, "docs", "plans", "x", "plan.md"), "plan\n")
	f := &fixture{root: root, app: app}
	f.worktrees = f.open(t, app, nil)
	return f
}

func (f *fixture) open(t *testing.T, dir string, c *catalog.Catalog) *Worktrees {
	t.Helper()
	w, err := New(dir, c)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func (f *fixture) out(line string) { f.output = append(f.output, line) }

func (f *fixture) path(name string) string { return filepath.Join(f.app+".worktrees", name) }

func (f *fixture) create(t *testing.T, w *Worktrees, branch, base string, name *string, deps bool) {
	t.Helper()
	if _, err := w.Create(branch, base, name, deps, f.out); err != nil {
		t.Fatal(err)
	}
}

func shipped(t *testing.T) *catalog.Catalog {
	t.Helper()
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func isLink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink != 0
}

func hasUpstream(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "@{u}")
	cmd.Dir = dir
	return cmd.Run() == nil
}

func errorContaining(t *testing.T, err error, part string) {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) || !strings.Contains(e.Message, part) {
		t.Errorf("expected a worktree error containing %q, got %v", part, err)
	}
}

func has(list []string, value string) bool { return contains(list, value) }

func TestCreateAddsAWorktreeWithNoUpstreamTheBaseRecordedAndTheToolkitLinked(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.worktrees, "feat/orders", "origin/develop", nil, false)
	path := f.path("feat-orders")

	if got := gitIn(t, path, "branch", "--show-current"); got != "feat/orders" {
		t.Errorf("branch %s", got)
	}
	if hasUpstream(path) {
		t.Error("the new branch has an upstream")
	}
	if got := gitIn(t, f.app, "config", "--get", "branch.feat/orders.dashcrew-base"); got != "develop" {
		t.Errorf("recorded base %s", got)
	}
	for _, linked := range []string{"AGENTS.md", "CLAUDE.md", ".claude", ".codex", ".agents", ".ai/bin", "docs/plans"} {
		target := filepath.Join(path, filepath.FromSlash(linked))
		if !isLink(target) {
			t.Errorf("%s is not a link", linked)
			continue
		}
		if dest, _ := os.Readlink(target); dest != filepath.Join(f.app, filepath.FromSlash(linked)) {
			t.Errorf("%s -> %s", linked, dest)
		}
	}
	if isLink(filepath.Join(path, ".ai", "project.yaml")) {
		t.Error(".ai/project.yaml is linked")
	}
	if status := gitIn(t, path, "status", "--porcelain"); status != "" {
		t.Errorf("status %q", status)
	}
	if !has(f.output, "upstream: none") || !has(f.output, "deps: skipped") {
		t.Errorf("output %v", f.output)
	}
}

func TestCreateRunsTheDepsCommandOfTheProfilePlatformWhenOneOfItsDetectFilesExists(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.app, ".ai", "project.yaml"), `
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [Makefile], commands: { analyze: make lint, test: make test, test_scoped: make one, deps: touch deps-ran, format: null } }
`)
	writeFile(t, filepath.Join(f.app, "Makefile"), "deps:\n")
	commitAll(t, f.app, "platform other")

	f.create(t, f.open(t, f.app, shipped(t)), "feat/a", "main", nil, true)

	if !has(f.output, "deps: ok (touch deps-ran)") {
		t.Errorf("output %v", f.output)
	}
	if _, err := os.Stat(filepath.Join(f.path("feat-a"), "deps-ran")); err != nil {
		t.Error("the deps command did not run in the worktree")
	}
}

func TestDepsIsSkippedWithoutADetectFileAndReportsAFailingCommandWithTheRerunLine(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.open(t, f.app, shipped(t)), "feat/a", "main", nil, true)
	if !has(f.output, "deps: skipped (no pubspec.yaml)") {
		t.Errorf("output %v", f.output)
	}

	writeFile(t, filepath.Join(f.app, ".ai", "project.yaml"), `
toolkit: 1
agents: [claude-code]
platform: other
platform_config: { detect: [lib/a.dart], commands: { analyze: a, test: b, test_scoped: c, deps: exit 3 } }
`)
	f.create(t, f.open(t, f.app, shipped(t)), "feat/b", "main", nil, true)
	if want := "deps: failed (exit 3); rerun: cd " + f.path("feat-b") + " && exit 3"; !has(f.output, want) {
		t.Errorf("output %v lacks %q", f.output, want)
	}
}

func TestCreateLinksOnlyWhatTheMainCheckoutIgnores(t *testing.T) {
	f := newFixture(t)
	writeFile(t, filepath.Join(f.app, ".mcp.json"), "{}\n")
	gitIn(t, f.app, "rm", "-q", "--cached", ".ai/project.yaml")
	gitIn(t, f.app, "commit", "-q", "-m", "untrack profile")
	gitIn(t, f.app, "push", "-q", "origin", "main")

	f.create(t, f.worktrees, "feat/a", "origin/main", nil, false)
	path := f.path("feat-a")

	for _, absent := range []string{".mcp.json", ".ai/project.yaml"} {
		if _, err := os.Lstat(filepath.Join(path, filepath.FromSlash(absent))); err == nil {
			t.Errorf("%s exists in the worktree", absent)
		}
	}
	if !isLink(filepath.Join(path, ".ai", "bin")) {
		t.Error(".ai/bin is not linked")
	}
	if status := gitIn(t, path, "status", "--porcelain"); status != "" {
		t.Errorf("worktree status %q", status)
	}
	mainStatus := gitIn(t, f.app, "status", "--porcelain", "--untracked-files=all")
	for _, part := range []string{"?? .mcp.json", "?? .ai/project.yaml"} {
		if !strings.Contains(mainStatus, part) {
			t.Errorf("main status lacks %q", part)
		}
	}
}

func TestCreateRefusesAMissingBaseAnExistingBranchAndAnExistingFolder(t *testing.T) {
	f := newFixture(t)
	_, err := f.worktrees.Create("feat/a", "origin/nope", nil, false, f.out)
	errorContaining(t, err, "origin/nope")

	f.create(t, f.worktrees, "feat/a", "main", nil, false)
	_, err = f.worktrees.Create("feat/a", "main", nil, false, f.out)
	errorContaining(t, err, "already exists")
	name := "feat-a"
	_, err = f.worktrees.Create("feat/b", "main", &name, false, f.out)
	errorContaining(t, err, "already exists")
}

func TestCreateWorksFromInsideAnotherWorktree(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.worktrees, "feat/a", "main", nil, false)

	f.create(t, f.open(t, f.path("feat-a"), nil), "feat/b", "main", nil, false)

	if info, err := os.Stat(f.path("feat-b")); err != nil || !info.IsDir() {
		t.Error("feat-b was not created next to feat-a")
	}
}

func TestLinkRestoresAMissingLinkAndDropsAnUpstream(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.worktrees, "feat/a", "main", nil, false)
	path := f.path("feat-a")
	os.Remove(filepath.Join(path, "AGENTS.md"))
	gitIn(t, path, "branch", "--set-upstream-to=origin/main")

	added, err := f.worktrees.Link("feat-a", f.out)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(added, []string{"AGENTS.md"}) {
		t.Errorf("added %v", added)
	}
	if hasUpstream(path) {
		t.Error("the upstream was kept")
	}
	if again, err := f.worktrees.Link("feat-a", f.out); err != nil || len(again) != 0 {
		t.Errorf("second link %v, %v", again, err)
	}
}

func TestListShowsBranchBaseUpstreamDirtyLinesAndMissingLinks(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.worktrees, "feat/a", "origin/develop", nil, false)
	writeFile(t, filepath.Join(f.path("feat-a"), "lib", "a.dart"), "void a() { }\n")
	os.Remove(filepath.Join(f.path("feat-a"), "CLAUDE.md"))

	infos, err := f.worktrees.List()
	if err != nil || len(infos) != 1 {
		t.Fatalf("list %v, %v", infos, err)
	}
	info := infos[0]

	if info.Name != "feat-a" || info.Branch != "feat/a" || info.Base == nil || *info.Base != "develop" ||
		info.Upstream != nil || info.Dirty != 1 || !reflect.DeepEqual(info.MissingLinks, []string{"CLAUDE.md"}) {
		t.Errorf("info %+v", info)
	}
}

func TestRemoveRefusesADirtyWorktree(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.worktrees, "feat/a", "main", nil, false)
	writeFile(t, filepath.Join(f.path("feat-a"), "lib", "new.dart"), "\n")

	errorContaining(t, f.worktrees.Remove("feat-a", f.out), "uncommitted")
	if _, err := os.Stat(f.path("feat-a")); err != nil {
		t.Error("the dirty worktree was removed")
	}
}

func TestRemoveRemovesACleanWorktreeKeepsTheBranchAndTheLinkedToolkit(t *testing.T) {
	f := newFixture(t)
	f.create(t, f.worktrees, "feat/a", "main", nil, false)

	if err := f.worktrees.Remove("feat-a", f.out); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(f.path("feat-a")); err == nil {
		t.Error("the worktree folder is still there")
	}
	if !strings.Contains(gitIn(t, f.app, "branch", "--list", "feat/a"), "feat/a") {
		t.Error("the branch was deleted")
	}
	if data, _ := os.ReadFile(filepath.Join(f.app, "AGENTS.md")); string(data) != "rules\n" {
		t.Errorf("AGENTS.md %q", data)
	}
	if _, err := os.Stat(filepath.Join(f.app, "docs", "plans", "x", "plan.md")); err != nil {
		t.Error("the plan in the main checkout is gone")
	}
}

type watchFixture struct {
	*fixture
	stateDir string
	prs      map[string]string
}

func newWatchFixture(t *testing.T) *watchFixture {
	f := newFixture(t)
	return &watchFixture{fixture: f, stateDir: filepath.Join(f.root, "state"), prs: map[string]string{}}
}

func (w *watchFixture) watch() *Watch {
	return &Watch{Worktrees: w.worktrees, StateDir: w.stateDir, PrLookup: func(branch string) *string {
		if pr, ok := w.prs[branch]; ok {
			return &pr
		}
		return nil
	}}
}

func (w *watchFixture) pass(t *testing.T, names ...string) []string {
	t.Helper()
	events, err := w.watch().Pass(names)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestWatchReportsFirstCommitAndPushOnceEach(t *testing.T) {
	w := newWatchFixture(t)
	w.create(t, w.worktrees, "feat/a", "origin/main", nil, false)
	path := w.path("feat-a")
	if events := w.pass(t); len(events) != 0 {
		t.Errorf("first pass %v", events)
	}

	writeFile(t, filepath.Join(path, "lib", "b.dart"), "\n")
	commitAll(t, path, "b")
	if events := w.pass(t); !reflect.DeepEqual(events, []string{"commit feat/a: first commit, 1 ahead of origin/main"}) {
		t.Errorf("after commit %v", events)
	}

	gitIn(t, path, "push", "-q", "origin", "feat/a")
	if events := w.pass(t); len(events) != 1 || !strings.HasPrefix(events[0], "pushed feat/a: ") {
		t.Errorf("after push %v", events)
	}
	if events := w.pass(t); len(events) != 0 {
		t.Errorf("repeat %v", events)
	}
}

func TestWatchReportsAMovedBaseWithTheMergeTreeResult(t *testing.T) {
	w := newWatchFixture(t)
	w.create(t, w.worktrees, "feat/a", "origin/main", nil, false)
	path := w.path("feat-a")
	writeFile(t, filepath.Join(path, "lib", "a.dart"), "void a() { /* branch */ }\n")
	commitAll(t, path, "branch change")
	w.pass(t)

	writeFile(t, filepath.Join(w.app, "lib", "other.dart"), "\n")
	commitAll(t, w.app, "base change")
	gitIn(t, w.app, "push", "-q", "origin", "main")
	clean := w.pass(t)
	if len(clean) == 0 || !strings.HasPrefix(clean[0], "base-moved origin/main: ") || !has(clean, "merge-tree feat/a vs origin/main: CLEAN") {
		t.Errorf("clean %v", clean)
	}

	writeFile(t, filepath.Join(w.app, "lib", "a.dart"), "void a() { /* base */ }\n")
	commitAll(t, w.app, "conflicting base change")
	gitIn(t, w.app, "push", "-q", "origin", "main")
	if events := w.pass(t); !has(events, "merge-tree feat/a vs origin/main: CONFLICT lib/a.dart") {
		t.Errorf("conflict %v", events)
	}
}

func TestWatchReportsGrowingFileOverlapBetweenWorktrees(t *testing.T) {
	w := newWatchFixture(t)
	w.create(t, w.worktrees, "feat/a", "main", nil, false)
	w.create(t, w.worktrees, "feat/b", "main", nil, false)
	writeFile(t, filepath.Join(w.path("feat-a"), "lib", "a.dart"), "void a() { 1; }\n")
	if events := w.pass(t); len(events) != 0 {
		t.Errorf("one side %v", events)
	}

	writeFile(t, filepath.Join(w.path("feat-b"), "lib", "a.dart"), "void a() { 2; }\n")
	if events := w.pass(t); !reflect.DeepEqual(events, []string{"overlap feat/a + feat/b: lib/a.dart"}) {
		t.Errorf("overlap %v", events)
	}
	if events := w.pass(t); len(events) != 0 {
		t.Errorf("repeat %v", events)
	}
}

func TestWatchReportsPrChangesAndEndsWithDoneWhenEveryWatchedWorktreeIsMergedOrRemoved(t *testing.T) {
	w := newWatchFixture(t)
	w.create(t, w.worktrees, "feat/a", "main", nil, false)
	w.create(t, w.worktrees, "feat/b", "main", nil, false)
	if events := w.pass(t); len(events) != 0 {
		t.Errorf("first %v", events)
	}

	w.prs["feat/a"] = "#3 OPEN"
	if events := w.pass(t); !reflect.DeepEqual(events, []string{"pr feat/a: #3 OPEN"}) {
		t.Errorf("open %v", events)
	}

	w.prs["feat/a"] = "#3 MERGED"
	if err := w.worktrees.Remove("feat-b", w.out); err != nil {
		t.Fatal(err)
	}
	if events := w.pass(t); !reflect.DeepEqual(events, []string{"pr feat/a: #3 MERGED", "done: 2 worktrees merged or removed"}) {
		t.Errorf("done %v", events)
	}
}

func TestWatchRunWithOnceStopsAfterOnePass(t *testing.T) {
	w := newWatchFixture(t)
	w.create(t, w.worktrees, "feat/a", "main", nil, false)
	w.prs["feat/a"] = "#5 MERGED"
	lines := []string{}

	if err := w.watch().Run(nil, true, time.Hour, func(line string) { lines = append(lines, line) }); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(lines, []string{"pr feat/a: #5 MERGED", "done: 1 worktrees merged or removed"}) {
		t.Errorf("lines %v", lines)
	}
}

func TestWatchNamesLimitTheWatchToThoseWorktrees(t *testing.T) {
	w := newWatchFixture(t)
	w.create(t, w.worktrees, "feat/a", "main", nil, false)
	w.create(t, w.worktrees, "feat/b", "main", nil, false)
	w.prs["feat/b"] = "#4 OPEN"

	if events := w.pass(t, "feat-a"); len(events) != 0 {
		t.Errorf("feat-a only %v", events)
	}
	_, err := w.watch().Pass([]string{"feat-x"})
	errorContaining(t, err, "No worktree named feat-x")
}
