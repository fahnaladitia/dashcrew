package knowledge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/pathstyle"
)

// gitEnv isolates git from the machine: no system or global config, a fixed identity.
func gitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, key := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+key+"_NAME", "t")
		t.Setenv("GIT_"+key+"_EMAIL", "t@example.com")
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitFile(t *testing.T, dir, file, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, file), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", file)
	return git(t, dir, "rev-parse", "HEAD")
}

// remote is a bare repo with one commit, and a working clone (seed) that pushes to it.
func remote(t *testing.T) (bare, seed string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bare, seed = filepath.Join(root, "notes.git"), filepath.Join(root, "seed")
	git(t, root, "init", "-q", "--bare", "-b", "main", bare)
	git(t, root, "clone", "-q", bare, seed)
	commitFile(t, seed, "features/share.md", "---\ntitle: Share\n---\n")
	git(t, seed, "push", "-q", "origin", "main")
	return bare, seed
}

func TestPullClonesFastForwardsAndNeverResetsADivergedClone(t *testing.T) {
	gitEnv(t)
	bare, seed := remote(t)
	clone := filepath.Join(t.TempDir(), ".dashcrew", "knowledge", "notes")
	now := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

	message, err := Pull(bare, clone, now)
	if err != nil || message != "cloned "+bare+" into "+clone+" at "+short(git(t, seed, "rev-parse", "HEAD")) {
		t.Fatalf("clone: %q, %v", message, err)
	}
	if head, err := HeadSHA(clone); err != nil || head != git(t, seed, "rev-parse", "HEAD") {
		t.Errorf("head %q, %v", head, err)
	}
	if pulled, ok := LastPull(clone); !ok || !pulled.Equal(now) {
		t.Errorf("last pull %v, %v", pulled, ok)
	}

	old := git(t, seed, "rev-parse", "HEAD")
	commitFile(t, seed, "features/tags.md", "tags\n")
	newer := commitFile(t, seed, "repos/notes-api.md", "api\n")
	git(t, seed, "push", "-q", "origin", "main")
	if err := Fetch(clone); err != nil {
		t.Fatal(err)
	}
	if behind, err := Behind(clone); err != nil || behind != 2 {
		t.Errorf("behind %d, %v", behind, err)
	}
	later := now.Add(time.Hour)
	message, err = Pull(bare, clone, later)
	if err != nil || message != "fast-forwarded "+clone+" "+short(old)+".."+short(newer)+" (2 commits)" {
		t.Fatalf("fast-forward: %q, %v", message, err)
	}
	if pulled, _ := LastPull(clone); !pulled.Equal(later) {
		t.Errorf("last pull not updated: %v", pulled)
	}
	if message, err = Pull(bare, clone, later); err != nil || message != clone+" is up to date at "+short(newer) {
		t.Errorf("up to date: %q, %v", message, err)
	}

	local := commitFile(t, clone, "inbox/2026-10-01-tags.md", "proposal\n")
	if message, err = Pull(bare, clone, later); err != nil || message != clone+" is 1 commit ahead of its remote; nothing to pull" {
		t.Errorf("ahead: %q, %v", message, err)
	}

	commitFile(t, seed, "features/export.md", "export\n")
	git(t, seed, "push", "-q", "origin", "main")
	_, err = Pull(bare, clone, later)
	var diverged *DivergedError
	if !errors.As(err, &diverged) || diverged.Local != 1 || diverged.Remote != 1 {
		t.Fatalf("diverged: %v", err)
	}
	if want := clone + " has diverged from its remote (1 local, 1 remote commit). dashcrew never resets it; " +
		"merge or rebase it by hand."; err.Error() != want {
		t.Errorf("message %q", err.Error())
	}
	if head, _ := HeadSHA(clone); head != local {
		t.Errorf("a diverged clone moved: %s, want %s", head, local)
	}
}

func TestPullRefusesAFolderThatIsNotAClone(t *testing.T) {
	gitEnv(t)
	bare, _ := remote(t)
	clone := t.TempDir()
	if _, err := Pull(bare, clone, time.Now()); err == nil || !strings.Contains(err.Error(), "is not a git clone") {
		t.Errorf("%v", err)
	}
}

func TestHeadSHAReadsPackedRefsAndADetachedHead(t *testing.T) {
	gitEnv(t)
	_, seed := remote(t)
	want := git(t, seed, "rev-parse", "HEAD")
	git(t, seed, "pack-refs", "--all")
	if head, err := HeadSHA(seed); err != nil || head != want {
		t.Errorf("packed: %q, %v", head, err)
	}
	git(t, seed, "checkout", "-q", "--detach")
	if head, err := HeadSHA(seed); err != nil || head != want {
		t.Errorf("detached: %q, %v", head, err)
	}
	empty := t.TempDir()
	git(t, empty, "init", "-q", "-b", "main")
	if head, err := HeadSHA(empty); err != nil || head != "" {
		t.Errorf("no commits: %q, %v", head, err)
	}
	if _, err := HeadSHA(t.TempDir()); err == nil {
		t.Error("not a repo: no error")
	}
}

func TestPathsAndRepoResolution(t *testing.T) {
	if got := CloneDir(filepath.FromSlash("/home/a"), "notes"); got != filepath.FromSlash("/home/a/.dashcrew/knowledge/notes") {
		t.Errorf("clone dir %s", got)
	}
	if got := PolicyClonePath("notes"); got != "~/.dashcrew/knowledge/notes" {
		t.Errorf("policy path %s", got)
	}
	if got := ExpandHome("~/.dashcrew/knowledge/notes", "/home/a"); got != filepath.FromSlash("/home/a/.dashcrew/knowledge/notes") {
		t.Errorf("expand %s", got)
	}
	project := filepath.FromSlash("/work/app")
	for repo, want := range map[string]string{
		"git@example.com:team/kb.git": "git@example.com:team/kb.git",
		"https://example.com/team/kb": "https://example.com/team/kb",
		"file:///srv/kb.git":          "file:///srv/kb.git",
		"../kb.git":                   filepath.Join(project, "..", "kb.git"),
		filepath.FromSlash("/srv/kb"): filepath.FromSlash("/srv/kb"),
		"./team:kb":                   filepath.Join(project, "team:kb"),
	} {
		if got := ResolveRepo(repo, project); got != want {
			t.Errorf("%s: %s, want %s", repo, got, want)
		}
	}
}

func TestResolveRepoOnWindowsPaths(t *testing.T) {
	project := `D:\work\app`
	for repo, want := range map[string]string{
		"git@example.com:team/kb.git": "git@example.com:team/kb.git",
		"https://example.com/team/kb": "https://example.com/team/kb",
		`C:\srv\kb`:                   `C:\srv\kb`,
		"c:/srv/kb":                   "c:/srv/kb",
		`\srv\kb`:                     `\srv\kb`,
		"/srv/kb":                     "/srv/kb",
		`\\server\share\kb`:           `\\server\share\kb`,
		"../kb.git":                   `D:\work\kb.git`,
		`..\kb.git`:                   `D:\work\kb.git`,
		"./team:kb":                   `D:\work\app\team:kb`,
	} {
		if got := resolveRepo(pathstyle.Windows, repo, project); got != want {
			t.Errorf("%s: %s, want %s", repo, got, want)
		}
	}
	if got := resolveRepo(pathstyle.POSIX, "../kb.git", "/work/app"); got != "/work/kb.git" {
		t.Errorf("POSIX: %s", got)
	}
}

func TestLastPullFallsBackToFetchHead(t *testing.T) {
	clone := t.TempDir()
	if _, ok := LastPull(clone); ok {
		t.Error("a folder without git has a last pull")
	}
	if err := os.MkdirAll(filepath.Join(clone, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := os.WriteFile(filepath.Join(clone, ".git", "FETCH_HEAD"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(filepath.Join(clone, ".git", "FETCH_HEAD"), stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if pulled, ok := LastPull(clone); !ok || !pulled.Equal(stamp) {
		t.Errorf("%v, %v", pulled, ok)
	}
}
