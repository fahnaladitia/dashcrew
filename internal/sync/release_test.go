package sync

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

const lineProfile = `toolkit: 1
agents: [claude-code]
project:
  base_branches: [develop]
  release_lines: { main: develop, release: 'release/(?P<version>[0-9.]+)', hotfix: 'hotfix/(?P<version>[0-9.]+)' }
  branch_prefix:
    feat: feat/
    hotfix: hotfix/
    fix: fix/
`

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// lineProject is a synced project committed on develop, with release/2.4.0 cut from it and checked out.
func lineProject(t *testing.T) (Sync, string) {
	t.Helper()
	s, project := newProject(t, lineProfile)
	run(t, s, false, false)
	gitIn(t, project, "init", "-q", "-b", "develop")
	gitIn(t, project, "add", "-A")
	gitIn(t, project, "commit", "-q", "-m", "toolkit")
	gitIn(t, project, "checkout", "-q", "-b", "release/2.4.0")
	return s, project
}

// olderToolkit makes the lock look written by dashcrew 0.0.0, whose catalog rendered path differently.
func olderToolkit(t *testing.T, project string, paths ...string) {
	t.Helper()
	current := lock(t, project)
	current.Set("dashcrew_version", "0.0.0")
	files := current.Value("files").(*omap.Map[any])
	for _, path := range paths {
		old := strings.Replace(read(t, project, path), "\n", "\nOlder catalog text.\n", 1)
		write(t, project, path, old)
		files.Set(path, FileHash(path, old))
	}
	write(t, project, LockPath, encode(t, current))
}

func runWith(t *testing.T, s Sync, opts Options) *Result {
	t.Helper()
	result, err := s.RunWith(opts)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestALockWithoutReleaseLinesHasNoInputs(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	run(t, s, false, false)
	if lock(t, project).Has("inputs") {
		t.Error("inputs written without release_lines")
	}
}

func TestReleaseLinesRecordTheCatalogAndProfileInTheLock(t *testing.T) {
	s, project := newProject(t, lineProfile)
	run(t, s, false, false)
	inputs, _ := lock(t, project).Value("inputs").(*omap.Map[any])
	if inputs == nil || inputs.Value("catalog") != s.Catalog.Digest || inputs.Value("profile") != FileHash("", lineProfile) {
		t.Errorf("inputs %v", encode(t, lock(t, project).Value("inputs")))
	}
}

func TestALineHoldsCatalogDrivenChanges(t *testing.T) {
	s, project := lineProject(t)
	olderToolkit(t, project, "AGENTS.md", ".claude/agents/reviewer.md")
	agents := read(t, project, "AGENTS.md")

	result := run(t, s, false, false)

	if !slices.Equal(result.Held, []string{".claude/agents/reviewer.md", "AGENTS.md"}) {
		t.Errorf("held %v", result.Held)
	}
	if len(result.Written) != 0 || read(t, project, "AGENTS.md") != agents {
		t.Errorf("wrote %v", result.Written)
	}
	for _, part := range []string{"release/2.4.0 is the release line 2.4.0", "dashcrew " + version.Current() + " is newer than the lock's 0.0.0",
		"  AGENTS.md\n", "dashcrew sync --upgrade", "upgrade on the main line develop"} {
		if !strings.Contains(result.HeldMessage, part) {
			t.Errorf("message lacks %q:\n%s", part, result.HeldMessage)
		}
	}
	if got := lock(t, project).Value("dashcrew_version"); got != "0.0.0" {
		t.Errorf("lock version moved to %v", got)
	}
	if again := run(t, s, false, false); len(again.Held) != 2 {
		t.Errorf("second run held %v", again.Held)
	}

	upgraded := runWith(t, s, Options{Upgrade: true})
	if len(upgraded.Held) != 0 || !slices.Contains(upgraded.Written, "AGENTS.md") || lock(t, project).Value("dashcrew_version") != version.Current() {
		t.Errorf("upgrade held %v wrote %v", upgraded.Held, upgraded.Written)
	}
	if strings.Contains(read(t, project, "AGENTS.md"), "Older catalog text.") {
		t.Error("upgrade left the old text")
	}
}

func TestALineWritesProfileDrivenChangesAndHoldsTheRest(t *testing.T) {
	s, project := lineProject(t)
	olderToolkit(t, project, ".claude/agents/reviewer.md")
	write(t, project, ".ai/project.yaml", strings.Replace(lineProfile, "feat: feat/", "feat: feature/", 1))

	result := run(t, s, false, false)

	if !slices.Equal(result.Held, []string{".claude/agents/reviewer.md"}) {
		t.Errorf("held %v", result.Held)
	}
	if !slices.Contains(result.Written, "AGENTS.md") || !strings.Contains(read(t, project, "AGENTS.md"), "`feature/` feature") {
		t.Errorf("profile change not written: %v", result.Written)
	}
	current := lock(t, project)
	if current.Value("dashcrew_version") != "0.0.0" {
		t.Errorf("lock version %v", current.Value("dashcrew_version"))
	}
	files := current.Value("files").(*omap.Map[any])
	if files.Value("AGENTS.md") != FileHash("AGENTS.md", read(t, project, "AGENTS.md")) ||
		files.Value(".claude/agents/reviewer.md") != FileHash(".claude/agents/reviewer.md", read(t, project, ".claude/agents/reviewer.md")) {
		t.Error("lock does not match the files")
	}
}

func TestAFileChangedByBothTheProfileAndTheCatalogIsHeld(t *testing.T) {
	s, project := lineProject(t)
	olderToolkit(t, project, "AGENTS.md")
	write(t, project, ".ai/project.yaml", strings.Replace(lineProfile, "feat: feat/", "feat: feature/", 1))

	if result := run(t, s, false, false); !slices.Equal(result.Held, []string{"AGENTS.md"}) {
		t.Errorf("held %v", result.Held)
	}
}

func TestWithoutTheLockedProfileEveryChangeIsHeld(t *testing.T) {
	s, project := lineProject(t)
	olderToolkit(t, project, ".claude/agents/reviewer.md")
	write(t, project, ".ai/project.yaml", strings.Replace(lineProfile, "feat: feat/", "feat: feature/", 1))
	gitIn(t, project, "commit", "-q", "-am", "profile and toolkit edited together")

	held := run(t, s, false, false).Held
	if !slices.Contains(held, ".claude/agents/reviewer.md") || !slices.Contains(held, "AGENTS.md") || !slices.Contains(held, ".claude/skills/branching/SKILL.md") {
		t.Errorf("held %v", held)
	}
}

func TestCatalogTextAloneIsHeldAtTheSameVersion(t *testing.T) {
	s, project := lineProject(t)
	changed := *s.Catalog
	changed.Digest = "another build"
	changed.Core = append([]catalog.Fragment{}, s.Catalog.Core...)
	for i, fragment := range changed.Core {
		if fragment.Source == "core/rules/branching.md" {
			changed.Core[i].Body = strings.Replace(fragment.Body, "## Branching", "## Branching\n\nNewer catalog text.", 1)
		}
	}
	s.Catalog = &changed

	result := run(t, s, false, false)

	if !slices.Equal(result.Held, []string{"AGENTS.md"}) || !strings.Contains(result.HeldMessage, "differs from the one that wrote the lock") {
		t.Errorf("held %v\n%s", result.Held, result.HeldMessage)
	}
	if lock(t, project).Value("inputs").(*omap.Map[any]).Value("catalog") == "another build" {
		t.Error("the lock took the new catalog digest")
	}
}

func TestABranchThatRecordsAReleaseIsHeldAndTheMainLineIsNot(t *testing.T) {
	s, project := lineProject(t)
	gitIn(t, project, "checkout", "-q", "-b", "fix/totals")
	gitIn(t, project, "config", "branch.fix/totals.dashcrew-release", "2.4.0")
	olderToolkit(t, project, "AGENTS.md")
	if result := run(t, s, false, false); !slices.Equal(result.Held, []string{"AGENTS.md"}) ||
		!strings.Contains(result.HeldMessage, "fix/totals records release 2.4.0") {
		t.Errorf("held %v\n%s", result.Held, result.HeldMessage)
	}

	gitIn(t, project, "checkout", "-q", "-b", "feat/guess")
	if result := run(t, s, false, false); len(result.Held) != 0 || lock(t, project).Value("dashcrew_version") != version.Current() {
		t.Errorf("a guessed line held %v", result.Held)
	}
	gitIn(t, project, "checkout", "-q", "-f", "develop")
	olderToolkit(t, project, "AGENTS.md")
	if result := run(t, s, false, false); len(result.Held) != 0 || !slices.Contains(result.Written, "AGENTS.md") {
		t.Errorf("the main line held %v", result.Held)
	}
}

func TestReleaseLinesOutsideGitChangeNothing(t *testing.T) {
	s, project := newProject(t, lineProfile)
	run(t, s, false, false)
	olderToolkit(t, project, "AGENTS.md")
	if result := run(t, s, false, false); len(result.Held) != 0 || lock(t, project).Value("dashcrew_version") != version.Current() {
		t.Errorf("held %v", result.Held)
	}
}

// mergedLines is a project on release/2.5.0 that merged release/2.4.0: each line changed one branch prefix, so
// AGENTS.md, the lock and README.md conflict while .ai/project.yaml merges cleanly.
func mergedLines(t *testing.T) (Sync, string) {
	t.Helper()
	s, project := lineProject(t)
	write(t, project, "README.md", "readme\n")
	gitIn(t, project, "add", "-A")
	gitIn(t, project, "commit", "-q", "-m", "readme")
	gitIn(t, project, "branch", "release/2.5.0")
	edit := func(from, to, readme string) {
		write(t, project, ".ai/project.yaml", strings.Replace(read(t, project, ".ai/project.yaml"), from, to, 1))
		write(t, project, "README.md", readme)
		run(t, s, false, false)
		gitIn(t, project, "add", "-A")
		gitIn(t, project, "commit", "-q", "-m", to)
	}
	edit("feat: feat/", "feat: feature/", "readme 2.4.0\n")
	gitIn(t, project, "checkout", "-q", "release/2.5.0")
	edit("fix: fix/", "fix: bugfix/", "readme 2.5.0\n")
	cmd := exec.Command("git", "-c", "user.name=t", "-c", "user.email=t@example.com", "merge", "--no-edit", "release/2.4.0")
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Fatalf("merge did not conflict:\n%s", out)
	}
	return s, project
}

func TestAfterMergeRendersConflictedGeneratedFilesFresh(t *testing.T) {
	s, project := mergedLines(t)
	unmerged := gitIn(t, project, "diff", "--name-only", "--diff-filter=U")
	for _, path := range []string{"AGENTS.md", LockPath, "README.md"} {
		if !strings.Contains(unmerged, path) {
			t.Fatalf("%s not conflicted: %s", path, unmerged)
		}
	}
	if !HasConflictMarkers(read(t, project, "AGENTS.md")) || !HasConflictMarkers(read(t, project, LockPath)) {
		t.Fatal("no conflict markers")
	}
	mustFail(t, s, LockPath+" has conflict markers from a merge; run dashcrew sync --after-merge.")

	result := runWith(t, s, Options{AfterMerge: true})

	if !slices.Contains(result.Resolved, "AGENTS.md") || !slices.Contains(result.Resolved, LockPath) {
		t.Errorf("resolved %v", result.Resolved)
	}
	if !slices.Contains(result.Stage, "AGENTS.md") || !slices.Contains(result.Stage, LockPath) || slices.Contains(result.Stage, "README.md") {
		t.Errorf("stage %v", result.Stage)
	}
	agents := read(t, project, "AGENTS.md")
	if HasConflictMarkers(agents) || !strings.Contains(agents, "`feature/` feature, `bugfix/` fix") {
		t.Errorf("AGENTS.md not rendered from the merged profile")
	}
	if !HasConflictMarkers(read(t, project, "README.md")) {
		t.Error("a file dashcrew does not own was touched")
	}
	if !strings.Contains(gitIn(t, project, "diff", "--name-only", "--diff-filter=U"), "AGENTS.md") {
		t.Error("staged by dashcrew")
	}
	if again := run(t, s, false, false); len(again.Written) != 0 || len(again.Held) != 0 {
		t.Errorf("a plain sync after --after-merge wrote %v held %v", again.Written, again.Held)
	}
}

func TestAfterMergeRefusesAConflictedProfile(t *testing.T) {
	s, project := mergedLines(t)
	write(t, project, ".ai/project.yaml", "<<<<<<< HEAD\ntoolkit: 1\n=======\ntoolkit: 1\n>>>>>>> release/2.4.0\n")
	_, err := s.RunWith(Options{AfterMerge: true})
	var syncErr *Error
	if !errors.As(err, &syncErr) || !strings.Contains(syncErr.Message, ".ai/project.yaml has conflict markers") {
		t.Errorf("got %v", err)
	}
}

func TestHasConflictMarkers(t *testing.T) {
	for text, want := range map[string]bool{
		"<<<<<<< HEAD\na\n=======\nb\n>>>>>>> other\n": true,
		"<<<<<<< HEAD\na\n||||||| base\n=======\nb\n>>>>>>> other\n": true,
		"a <<<<<<< b\n>>>>>>> c\n": false,
		"<<<<<<< HEAD\nonly the start\n": false,
		"plain\n": false,
	} {
		if got := HasConflictMarkers(text); got != want {
			t.Errorf("%q: %v", text, got)
		}
	}
}
