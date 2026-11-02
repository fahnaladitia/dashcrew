package lines

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

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

func commitFile(t *testing.T, dir, file, content string) {
	t.Helper()
	path := filepath.Join(dir, file)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitT(t, dir, "add", "-A")
	gitT(t, dir, "commit", "-q", "-m", file)
}

func releaseLines(t *testing.T) *profile.ReleaseLines {
	t.Helper()
	p, err := profile.Parse("toolkit: 1\nagents: [claude-code]\nproject:\n  release_lines:\n    main: develop\n"+
		"    release: 'release/(?P<version>[0-9.]+)'\n    hotfix: 'hotfix/(?P<version>[0-9.]+)'\n",
		profile.KnownOptions{"flutter": omap.New[[]string]()})
	if err != nil {
		t.Fatal(err)
	}
	return p.ReleaseLines
}

// fixture: develop (main line), release/2.4.0 with a fix, hotfix/2.4.5 cut from it, release/2.5.0 only on the
// remote, release/2.10.0 at develop's head, and feature branches.
func fixture(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitT(t, root, "init", "-q", "--bare", "-b", "develop", "origin.git")
	gitT(t, root, "clone", "-q", filepath.Join(root, "origin.git"), "app")
	app := filepath.Join(root, "app")
	commitFile(t, app, "a.txt", "a\n")
	gitT(t, app, "branch", "release/2.4.0")
	gitT(t, app, "checkout", "-q", "release/2.4.0")
	commitFile(t, app, "fix.txt", "fix\n")
	gitT(t, app, "checkout", "-q", "-b", "hotfix/2.4.5")
	commitFile(t, app, "hot.txt", "hot\n")
	gitT(t, app, "checkout", "-q", "develop")
	commitFile(t, app, "b.txt", "b\n")
	gitT(t, app, "push", "-q", "origin", "develop", "develop:release/2.5.0")
	gitT(t, app, "fetch", "-q", "origin")
	commitFile(t, app, "c.txt", "c\n")
	gitT(t, app, "branch", "release/2.10.0")
	gitT(t, app, "branch", "feature/not-a-line")
	return app
}

func TestListOrdersLinesBySemanticVersionWithTheMainLineLast(t *testing.T) {
	app := fixture(t)
	all, err := List(app, releaseLines(t))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, line := range all {
		got = append(got, line.Kind+" "+line.Version+" "+line.Branch)
		if line.Head != gitT(t, app, "rev-parse", line.Ref) {
			t.Errorf("%s head %s", line.Branch, line.Head)
		}
	}
	want := []string{"release 2.4.0 release/2.4.0", "hotfix 2.4.5 hotfix/2.4.5", "release 2.5.0 origin/release/2.5.0",
		"release 2.10.0 release/2.10.0", "main  develop"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestALocalLineWinsOverItsRemoteTrackingCopy(t *testing.T) {
	app := fixture(t)
	gitT(t, app, "branch", "release/2.5.0", "origin/release/2.5.0")
	all, err := List(app, releaseLines(t))
	if err != nil {
		t.Fatal(err)
	}
	if all[2].Branch != "release/2.5.0" || all[2].Ref != "refs/heads/release/2.5.0" || len(all) != 5 {
		t.Errorf("got %+v", all)
	}
}

func TestResolve(t *testing.T) {
	app := fixture(t)
	rl := releaseLines(t)
	gitT(t, app, "checkout", "-q", "-b", "feat/recorded", "develop")
	if err := RecordRelease(app, "feat/recorded", "2.4.5"); err != nil {
		t.Fatal(err)
	}
	if got := gitT(t, app, "config", "branch.feat/recorded.dashcrew-release"); got != "2.4.5" {
		t.Errorf("recorded %q", got)
	}
	gitT(t, app, "checkout", "-q", "-b", "fix/from-hotfix", "hotfix/2.4.5")
	commitFile(t, app, "x.txt", "x\n")
	gitT(t, app, "checkout", "-q", "-b", "fix/from-old-release", "release/2.4.0")
	commitFile(t, app, "y.txt", "y\n")
	gitT(t, app, "checkout", "-q", "-b", "feat/from-develop", "develop")
	commitFile(t, app, "z.txt", "z\n")
	gitT(t, app, "checkout", "-q", "-b", "feat/unknown-release", "develop")
	if err := RecordRelease(app, "feat/unknown-release", "9.9.9"); err != nil {
		t.Fatal(err)
	}
	all, err := List(app, rl)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		branch    string
		how       How
		line      string
		version   string
		protected bool
	}{
		{"develop", MainBranch, "develop", "", false},
		{"release/2.4.0", LineBranch, "release/2.4.0", "2.4.0", true},
		{"origin/release/2.5.0", LineBranch, "origin/release/2.5.0", "2.5.0", true},
		{"feat/recorded", Recorded, "hotfix/2.4.5", "2.4.5", true},
		{"feat/unknown-release", Recorded, "", "9.9.9", true},
		{"fix/from-hotfix", Guessed, "hotfix/2.4.5", "2.4.5", false},
		{"fix/from-old-release", Guessed, "release/2.4.0", "2.4.0", false},
		{"feat/from-develop", Guessed, "develop", "", false},
	} {
		got := Resolve(app, rl, all, c.branch)
		line := ""
		if got.Line != nil {
			line = got.Line.Branch
		}
		if got.How != c.how || line != c.line || got.Version != c.version || got.Protected() != c.protected {
			t.Errorf("%s: got how %v line %q version %q protected %v", c.branch, got.How, line, got.Version, got.Protected())
		}
	}
	if got := Resolve(app, rl, all, ""); got.Branch != "feat/unknown-release" || got.How != Recorded {
		t.Errorf("current branch: %+v", got)
	}
	gitT(t, app, "checkout", "-q", "--detach")
	if got := Resolve(app, rl, all, ""); got.How != None || got.Protected() {
		t.Errorf("detached: %+v", got)
	}
}

func TestDescribe(t *testing.T) {
	line := &Line{Kind: profile.ReleaseLine, Version: "2.4.0", Branch: "release/2.4.0"}
	main := &Line{Kind: profile.MainLine, Branch: "develop"}
	for _, c := range []struct {
		r    Resolution
		want string
	}{
		{Resolution{Branch: "develop", How: MainBranch, Line: main}, "develop is the main line"},
		{Resolution{Branch: "release/2.4.0", How: LineBranch, Line: line, Version: "2.4.0"}, "release/2.4.0 is the release line 2.4.0"},
		{Resolution{Branch: "feat/x", How: Recorded, Line: line, Version: "2.4.0"},
			"feat/x records release 2.4.0 (branch.feat/x.dashcrew-release), line release/2.4.0"},
		{Resolution{Branch: "feat/x", How: Recorded, Version: "9.9.9"},
			"feat/x records release 9.9.9 (branch.feat/x.dashcrew-release), no line branch for it"},
		{Resolution{Branch: "feat/y", How: Guessed, Line: line, Version: "2.4.0"},
			"feat/y is nearest to release/2.4.0 by merge-base, a guess; record it with git config branch.feat/y.dashcrew-release 2.4.0"},
		{Resolution{Branch: "feat/y", How: Guessed, Line: main},
			"feat/y is nearest to the main line develop by merge-base, a guess"},
		{Resolution{Branch: "feat/z", How: None}, "feat/z: no line found"},
		{Resolution{How: None}, "detached HEAD: no line"},
	} {
		if got := c.r.Describe(); got != c.want {
			t.Errorf("got  %q\nwant %q", got, c.want)
		}
	}
}

func TestShowReadsAFileAtARevisionRelativeToTheFolder(t *testing.T) {
	app := fixture(t)
	if text, ok := Show(app, "develop:./a.txt"); !ok || text != "a\n" {
		t.Errorf("show %q %v", text, ok)
	}
	if _, ok := Show(app, "develop:./missing.txt"); ok {
		t.Error("missing file shown")
	}
	if _, ok := Show(t.TempDir(), "HEAD:./a.txt"); ok {
		t.Error("shown outside a repository")
	}
}

func TestFindTakesAVersionABranchOrMain(t *testing.T) {
	app := fixture(t)
	all, err := List(app, releaseLines(t))
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"2.4.5": "hotfix/2.4.5", "release/2.4.0": "release/2.4.0", "origin/release/2.5.0": "origin/release/2.5.0",
		"release/2.5.0": "origin/release/2.5.0", "main": "develop", "develop": "develop",
	} {
		if got := Find(all, name); got == nil || got.Branch != want {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if got := Find(all, "9.9.9"); got != nil {
		t.Errorf("9.9.9: %+v", got)
	}
}

func TestPairsAreAdjacentLinesUpToMain(t *testing.T) {
	all := []Line{{Version: "2.4.0", Branch: "a"}, {Version: "2.4.5", Branch: "b"}, {Kind: profile.MainLine, Branch: "m"}}
	var got []string
	for _, pair := range Pairs(all) {
		got = append(got, pair[0].Branch+">"+pair[1].Branch)
	}
	if want := []string{"a>b", "b>m"}; !reflect.DeepEqual(got, want) {
		t.Errorf("%q", got)
	}
	if len(Pairs(all[:1])) != 0 {
		t.Error("one line has a pair")
	}
}

func TestBranchPointIsWhereTheLineLeftTheMainLinesFirstParentChain(t *testing.T) {
	app := fixture(t)
	all, err := List(app, releaseLines(t))
	if err != nil {
		t.Fatal(err)
	}
	cut := gitT(t, app, "rev-parse", "release/2.4.0~1")
	gitT(t, app, "checkout", "-q", "develop")
	gitT(t, app, "merge", "-q", "--no-ff", "--no-edit", "release/2.4.0")
	all, _ = List(app, releaseLines(t))
	for _, name := range []string{"2.4.0", "2.4.5"} {
		base, label := BranchPoint(app, all, *Find(all, name))
		if base != cut || label != "develop" {
			t.Errorf("%s: %s %s, want %s develop", name, base, label, cut)
		}
	}
}

func TestPreviousIsTheNewestOlderLineOrVersionTag(t *testing.T) {
	app := fixture(t)
	all, err := List(app, releaseLines(t))
	if err != nil {
		t.Fatal(err)
	}
	if ref, label := Previous(app, all, *Find(all, "2.4.5")); ref != gitT(t, app, "rev-parse", "release/2.4.0") || label != "release/2.4.0" {
		t.Errorf("2.4.5: %s %s", ref, label)
	}
	gitT(t, app, "tag", "v2.4.7", "hotfix/2.4.5")
	gitT(t, app, "tag", "not-a-version", "develop")
	if ref, label := Previous(app, all, *Find(all, "2.5.0")); ref != gitT(t, app, "rev-parse", "hotfix/2.4.5") || label != "tag v2.4.7" {
		t.Errorf("2.5.0: %s %s", ref, label)
	}
	if _, label := Previous(app, all, *Find(all, "2.4.0")); label != "develop" {
		t.Errorf("2.4.0 falls back to the branch point: %s", label)
	}
}
