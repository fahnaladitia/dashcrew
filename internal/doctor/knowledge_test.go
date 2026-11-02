package doctor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/knowledge"
)

const knowledgeProfile = "toolkit: 1\nagents: [claude-code]\nknowledge: { repo: ../notes.git }\n"

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// knowledgeRemote is a bare knowledge repo with one page and a working copy (seed) that pushes to it; git runs
// without the machine's config.
func knowledgeRemote(t *testing.T) (bare, seed string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	for _, key := range []string{"AUTHOR", "COMMITTER"} {
		t.Setenv("GIT_"+key+"_NAME", "t")
		t.Setenv("GIT_"+key+"_EMAIL", "t@example.com")
	}
	root := t.TempDir()
	bare, seed = filepath.Join(root, "notes.git"), filepath.Join(root, "seed")
	gitIn(t, root, "init", "-q", "--bare", "-b", "main", bare)
	gitIn(t, root, "clone", "-q", bare, seed)
	push(t, seed, "features/share.md")
	return bare, seed
}

func push(t *testing.T, seed, file string) {
	t.Helper()
	writeFile(t, filepath.Join(seed, filepath.FromSlash(file)), file+"\n")
	gitIn(t, seed, "add", "-A")
	gitIn(t, seed, "commit", "-q", "-m", file)
	gitIn(t, seed, "push", "-q", "origin", "main")
}

func (f *fixture) knowledgeDoctor(now time.Time, fetch bool) map[string]Check {
	f.t.Helper()
	return byID(Doctor{
		Catalog: shippedCatalog, ProjectDir: f.project, UserSkillsDir: f.userSkills, PathEnv: f.pathEnv(), Home: f.home,
		CodexHome: f.codexHome, MachineChecks: true, Fetch: fetch, Now: func() time.Time { return now },
	}.Run())
}

func TestKnowledgeCheckOnlyRunsWithKnowledge(t *testing.T) {
	f := setup(t, blocProfile)
	f.sync()
	if _, ok := f.doctor()["knowledge"]; ok {
		t.Error("knowledge check without knowledge")
	}
}

func TestKnowledgeWithoutACloneWarns(t *testing.T) {
	f := setup(t, knowledgeProfile)
	f.sync()
	expect(t, f.doctor(), "knowledge", Warn, "no clone of notes at ~/.dashcrew/knowledge/notes; run dashcrew knowledge pull")
}

func TestKnowledgeLinkAgeAndCommitsBehind(t *testing.T) {
	bare, seed := knowledgeRemote(t)
	f := setup(t, knowledgeProfile)
	clone := knowledge.CloneDir(f.home, "notes")
	pulled := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	if _, err := knowledge.Pull(bare, clone, pulled); err != nil {
		t.Fatal(err)
	}
	f.sync()

	expect(t, f.knowledgeDoctor(pulled.Add(20*time.Minute), false), "knowledge", OK,
		"linked to ~/.dashcrew/knowledge/notes, pulled less than an hour ago")
	expect(t, f.knowledgeDoctor(pulled.Add(time.Hour), false), "knowledge", OK, "pulled 1 hour ago")
	expect(t, f.knowledgeDoctor(pulled.Add(50*time.Hour), true), "knowledge", OK, "pulled 2 days ago, up to date with its remote")

	push(t, seed, "features/tags.md")
	push(t, seed, "repos/notes-api.md")
	expect(t, f.knowledgeDoctor(pulled, false), "knowledge", OK, "pulled less than an hour ago")
	expect(t, f.knowledgeDoctor(pulled, true), "knowledge", Warn, "2 commits behind its remote; run dashcrew knowledge pull")

	os.RemoveAll(filepath.Join(clone, ".git", "refs", "remotes"))
	writeFile(t, filepath.Join(clone, ".git", "config"), "[core]\n\trepositoryformatversion = 0\n")
	expect(t, f.knowledgeDoctor(pulled, true), "knowledge", Warn, "fetch failed")

	f.delete(".ai/knowledge")
	expect(t, f.knowledgeDoctor(pulled, false), "knowledge", Warn, ".ai/knowledge is missing; run dashcrew sync")
	if err := os.Symlink(t.TempDir(), filepath.Join(f.project, ".ai", "knowledge")); err != nil {
		t.Fatal(err)
	}
	expect(t, f.knowledgeDoctor(pulled, false), "knowledge", Warn, "is not the clone; run dashcrew sync")
	f.delete(".ai/knowledge")
	f.write(".ai/knowledge/mine.md", "mine\n")
	expect(t, f.knowledgeDoctor(pulled, false), "knowledge", Warn, ".ai/knowledge is not the link dashcrew made")
}

func TestACopyOlderThanTheCloneWarns(t *testing.T) {
	f := setup(t, knowledgeProfile)
	clone := knowledge.CloneDir(f.home, "notes")
	writeFile(t, filepath.Join(clone, ".git", "HEAD"), "ref: refs/heads/main\n")
	writeFile(t, filepath.Join(clone, ".git", "refs", "heads", "main"), "1111111111111111111111111111111111111111\n")
	f.sync()
	if err := os.Remove(filepath.Join(f.project, ".ai", "knowledge")); err != nil {
		t.Fatal(err)
	}
	f.write(".ai/knowledge/features/share.md", "share\n")
	lock := strings.Replace(f.read(".ai/lock.json"), `"state": "linked"`, `"state": "copied"`, 1)
	f.write(".ai/lock.json", lock)
	expect(t, f.knowledgeDoctor(time.Now(), false), "knowledge", OK, "copied from ~/.dashcrew/knowledge/notes")
	writeFile(t, filepath.Join(clone, ".git", "refs", "heads", "main"), "2222222222222222222222222222222222222222\n")
	expect(t, f.knowledgeDoctor(time.Now(), false), "knowledge", Warn, "the copy is older than the clone; run dashcrew sync")
}

func TestKnowledgeStaleCheckSummarizesKnowledgeDoctor(t *testing.T) {
	f := setup(t, knowledgeProfile)
	f.sync()
	if _, ok := f.doctor()["knowledge-stale"]; ok {
		t.Error("stale check without a clone")
	}
	clone := knowledge.CloneDir(f.home, "notes")
	repo := filepath.Base(f.project)
	writeFile(t, filepath.Join(clone, "features", "share.md"),
		"---\ntitle: Share\n---\n\nIn `"+repo+":.ai/project.yaml:1`. <!-- verified "+repo+"@abcdef1 2026-10-01 -->\n")
	writeFile(t, filepath.Join(clone, "inbox", "2026-10-01-tags.md"), "x\n")
	now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	expect(t, f.knowledgeDoctor(now, false), "knowledge-stale", OK, "no stale pointers, old proposals or unverified pages")

	writeFile(t, filepath.Join(clone, "features", "tags.md"), "---\ntitle: Tags\n---\n\n`"+repo+":lib/gone.go:3` `"+repo+":lib/also-gone.go:1`\n")
	expect(t, f.knowledgeDoctor(now.AddDate(0, 0, 20), false), "knowledge-stale", Warn,
		"2 stale pointers, 1 old proposal, 1 unverified page; run dashcrew knowledge doctor")
}

// specFixture is a synced knowledge project on branch feat/sync-client, committed on main, with a spec of notes-sync
// at rev, status in the profile's clone.
func specFixture(t *testing.T, status string, rev int) (*fixture, string) {
	f := setup(t, knowledgeProfile)
	clone := knowledge.CloneDir(f.home, "notes")
	writeSpecFiles(t, clone, status, rev)
	f.sync()
	f.git("init", "-q", "-b", "main")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "chore: init")
	f.git("checkout", "-q", "-b", "feat/sync-client")
	return f, clone
}

func writeSpecFiles(t *testing.T, clone, status string, rev int) {
	t.Helper()
	for _, name := range []string{"prd.md", "trd.md"} {
		writeFile(t, filepath.Join(clone, "specs", "1.4.0", "notes-sync", name), "---\ntitle: Notes sync\nfeature: notes-sync\n"+
			"release: \"1.4.0\"\nstatus: "+status+"\nrev: "+strconv.Itoa(rev)+"\nrepos: [notes-app]\nowners: [product]\n---\n\n- AC-1 x\n")
	}
}

func TestSpecCheckWarnsOnAnOlderRevisionOrADraftPastPlanning(t *testing.T) {
	f, clone := specFixture(t, "approved", 2)
	if _, ok := f.doctor()["spec"]; ok {
		t.Error("spec check without a recorded spec")
	}
	f.git("config", "branch.feat/sync-client.dashcrew-spec", "specs/1.4.0/notes-sync@rev2")
	expect(t, f.doctor(), "spec", OK, "specs/1.4.0/notes-sync@rev2, approved, current")

	writeSpecFiles(t, clone, "review", 3)
	expect(t, f.doctor(), "spec", Warn, "specs/1.4.0/notes-sync@rev2: spec moved to rev 3; re-read it")

	f.git("config", "branch.feat/sync-client.dashcrew-spec", "specs/1.4.0/notes-sync@rev3")
	writeSpecFiles(t, clone, "draft", 3)
	expect(t, f.doctor(), "spec", OK, "draft, current")
	f.write("lib/a.dart", "a\n")
	f.git("add", "-A")
	f.git("commit", "-q", "-m", "feat: a")
	expect(t, f.doctor(), "spec", Warn, "spec is still draft while the branch has 1 commit past its base")

	f.git("config", "branch.feat/sync-client.dashcrew-spec", "specs/1.4.0/missing@rev1")
	expect(t, f.doctor(), "spec", Warn, "no spec at specs/1.4.0/missing in the knowledge clone")
	f.git("config", "branch.feat/sync-client.dashcrew-spec", "notes-sync")
	expect(t, f.doctor(), "spec", Warn, "branch.feat/sync-client.dashcrew-spec is notes-sync, not specs/<release>/<feature>@rev<n>")
	f.git("config", "branch.feat/sync-client.dashcrew-spec", "none")
	if _, ok := f.doctor()["spec"]; ok {
		t.Error("spec check for none")
	}
	for _, check := range f.doctor() {
		if check.Status == Fail && check.ID == "spec" {
			t.Error("the spec check failed")
		}
	}
}
