package commit

import (
	"crypto/sha256"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// tempRepo is a git repo with one commit of a.txt and lib/b.txt, run with a fixed author and no system config.
type tempRepo struct {
	t   *testing.T
	dir string
}

func newTempRepo(t *testing.T) tempRepo {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{
		{"GIT_CONFIG_NOSYSTEM", "1"}, {"GIT_CONFIG_GLOBAL", filepath.Join(dir, ".no-global")},
		{"GIT_AUTHOR_NAME", "t"}, {"GIT_AUTHOR_EMAIL", "t@example.com"},
		{"GIT_COMMITTER_NAME", "t"}, {"GIT_COMMITTER_EMAIL", "t@example.com"},
	} {
		t.Setenv(kv[0], kv[1])
	}
	r := tempRepo{t, dir}
	r.git("init", "-q", "-b", "main")
	r.write("a.txt", "a\n")
	r.write("lib/b.txt", "b\n")
	r.git("add", "-A")
	r.git("commit", "-q", "-m", "chore: init")
	return r
}

func (r tempRepo) write(path, content string) {
	r.t.Helper()
	full := filepath.Join(r.dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r tempRepo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (r tempRepo) open() Repo {
	r.t.Helper()
	repo, err := Open(filepath.Join(r.dir, "lib"))
	if err != nil {
		r.t.Fatal(err)
	}
	return repo
}

func TestOpenFindsTheTopLevelFromASubfolder(t *testing.T) {
	r := newTempRepo(t)
	if got := r.open().Dir; got != r.dir {
		t.Errorf("dir %s", got)
	}
	if _, err := Open(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not inside a git repository") {
		t.Errorf("outside a repo: %v", err)
	}
}

func TestChangesListsStagedUnstagedAndUntrackedApart(t *testing.T) {
	r := newTempRepo(t)
	r.write("a.txt", "a2\n")
	r.write("lib/new.txt", "staged new\n")
	r.git("add", "lib/new.txt")
	r.git("mv", "lib/b.txt", "lib/renamed.txt")
	r.write("notes/draft.md", "untracked\n")
	r.write("odd name's $x.txt", "untracked\n")

	changed, untracked, err := r.open().Changes()
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.txt", "lib/new.txt", "lib/renamed.txt"}; !reflect.DeepEqual(changed, want) {
		t.Errorf("changed %q", changed)
	}
	if want := []string{"notes/draft.md", "odd name's $x.txt"}; !reflect.DeepEqual(untracked, want) {
		t.Errorf("untracked %q", untracked)
	}
}

func TestBranchIsEmptyWhenDetached(t *testing.T) {
	r := newTempRepo(t)
	if got := r.open().Branch(); got != "main" {
		t.Errorf("branch %q", got)
	}
	r.git("checkout", "-q", "--detach")
	if got := r.open().Branch(); got != "" {
		t.Errorf("detached %q", got)
	}
}

func fingerprint(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return string(sum[:]) + info.ModTime().String()
}

func TestWorkTreeHashUsesATemporaryIndexAndNeverTouchesTheRealOne(t *testing.T) {
	r := newTempRepo(t)
	r.write("a.txt", "a2\n")
	r.write("lib/staged.txt", "staged\n")
	r.git("add", "lib/staged.txt")
	r.write("lib/staged.txt", "staged then edited\n")
	r.write("untracked.txt", "u\n")
	index := filepath.Join(r.dir, ".git", "index")
	// git status refreshes the index itself, so both reads run before the fingerprint.
	cachedBefore := r.git("diff", "--cached", "--name-only")
	statusBefore := r.git("status", "--porcelain")
	before := fingerprint(t, index)
	time.Sleep(10 * time.Millisecond)

	tree, err := r.open().WorkTree()
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint(t, index) != before {
		t.Error("the real index changed")
	}

	if got := r.git("diff", "--cached", "--name-only"); got != cachedBefore {
		t.Errorf("staged files changed: %q", got)
	}
	if got := r.git("status", "--porcelain"); got != statusBefore {
		t.Errorf("status changed: %q", got)
	}
	if _, err := os.Stat(index + ".lock"); err == nil {
		t.Error("index.lock left behind")
	}
	if listed := r.git("ls-tree", "-r", "--name-only", tree); listed != "a.txt\nlib/b.txt\nlib/staged.txt\nuntracked.txt" {
		t.Errorf("tree lists %q", listed)
	}
	if content := r.git("show", tree+":lib/staged.txt"); content != "staged then edited" {
		t.Errorf("tree holds %q, not the working copy", content)
	}
}

func TestWorkTreeHashWithoutACommitOrAnIndex(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := tempRepo{t, dir}
	r.git("init", "-q", "-b", "main")
	r.write("x.txt", "x\n")
	repo, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.WorkTree(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); err == nil {
		t.Error("a real index was created")
	}
}

func TestVerifyRecordAndChangesSince(t *testing.T) {
	r := newTempRepo(t)
	repo := r.open()

	if _, known, err := repo.SinceVerify(); err != nil || known {
		t.Fatalf("no record: known %v, %v", known, err)
	}
	tree, err := repo.RecordVerify()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(r.dir, ".git", "dashcrew", "last-verify"))
	if err != nil || string(data) != tree+"\n" {
		t.Fatalf("record %q, %v", data, err)
	}
	if files, known, err := repo.SinceVerify(); err != nil || !known || len(files) != 0 {
		t.Errorf("right after: %v %v %v", files, known, err)
	}

	r.write("a.txt", "edited by hand\n")
	r.write("lib/new.txt", "new\n")
	files, known, err := repo.SinceVerify()
	if err != nil || !known || !reflect.DeepEqual(files, []string{"a.txt", "lib/new.txt"}) {
		t.Errorf("after edits: %v %v %v", files, known, err)
	}

	if err := os.WriteFile(filepath.Join(r.dir, ".git", "dashcrew", "last-verify"), []byte("0123456789012345678901234567890123456789\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, known, err := repo.SinceVerify(); err != nil || known {
		t.Errorf("a tree no longer in the repo reads as unknown: %v %v", known, err)
	}
}

func TestVerifyRecordIsPerWorktree(t *testing.T) {
	r := newTempRepo(t)
	other := filepath.Join(filepath.Dir(r.dir), filepath.Base(r.dir)+"-wt")
	r.git("worktree", "add", "-q", "-b", "feat/x", other)
	t.Cleanup(func() { os.RemoveAll(other) })
	main := r.open()
	wt, err := Open(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := main.RecordVerify(); err != nil {
		t.Fatal(err)
	}
	if _, known, _ := wt.SinceVerify(); known {
		t.Error("a worktree read the main checkout's record")
	}
}

func TestMessagesSkipMergesAndKeepOrder(t *testing.T) {
	r := newTempRepo(t)
	r.git("checkout", "-q", "-b", "feat/x")
	r.write("a.txt", "1\n")
	r.git("commit", "-q", "-am", "feat: one\n\nBody line.")
	r.git("checkout", "-q", "main")
	r.write("c.txt", "c\n")
	r.git("add", "c.txt")
	r.git("commit", "-q", "-m", "chore: base moved")
	r.git("checkout", "-q", "feat/x")
	r.git("merge", "-q", "--no-edit", "main")
	r.write("a.txt", "2\n")
	r.git("commit", "-q", "-am", "wip")

	commits, err := r.open().Messages("main..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range commits {
		if len(c.Hash) != 40 {
			t.Errorf("hash %q", c.Hash)
		}
		got = append(got, c.Message)
	}
	if want := []string{"feat: one\n\nBody line.\n", "wip\n"}; !reflect.DeepEqual(got, want) {
		t.Errorf("messages %q", got)
	}
	if _, err := r.open().Messages("nope..HEAD"); err == nil {
		t.Error("an unknown base passed")
	}
	if _, err := r.open().Messages("--all"); err == nil {
		t.Error("an option passed as a range")
	}
}

func TestLintRangePrefixesTheShortHash(t *testing.T) {
	r := newTempRepo(t)
	r.git("checkout", "-q", "-b", "feat/x")
	r.write("a.txt", "1\n")
	r.git("commit", "-q", "-am", "feat: one")
	r.write("a.txt", "2\n")
	r.git("commit", "-q", "-am", "wip: two")
	short := r.git("rev-parse", "--short=7", "HEAD")

	problems, commits, err := LintRange(r.open(), config(t, "").Commit, "main..HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if commits != 2 || !reflect.DeepEqual(problems, []string{short + ` type "wip" is not one of feat, fix, refactor, test, docs, chore, perf, build, ci`}) {
		t.Errorf("%d commits, %q", commits, problems)
	}
}
