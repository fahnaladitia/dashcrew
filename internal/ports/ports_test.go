package ports

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/lines"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

type repo struct {
	t   *testing.T
	dir string
}

func (r repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null")
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit writes file and commits it with message; it returns the short hash.
func (r repo) commit(file, content, message string) string {
	r.t.Helper()
	if err := os.WriteFile(filepath.Join(r.dir, file), []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
	r.git("add", "-A")
	r.git("commit", "-q", "-m", message)
	return r.git("rev-parse", "--short=7", "HEAD")
}

func testProfile(t *testing.T) *profile.Profile {
	t.Helper()
	p, err := profile.Parse("toolkit: 1\nagents: [claude-code]\nproject:\n  release_lines:\n    main: develop\n"+
		"    release: 'release/(?P<version>[0-9.]+)'\n    hotfix: 'hotfix/(?P<version>[0-9.]+)'\n",
		profile.KnownOptions{"flutter": omap.New[[]string]()})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// fixture: develop; release/2.4.0 with five fixes, a feature and a chore marked `Port: yes`; hotfix/2.4.5 cut
// from it with one fix; release/2.5.0 cut from develop later, where the fixes are ported by trailer, by
// cherry-pick -x and by patch-id, and one port is reverted. It returns the short hashes by name.
func fixture(t *testing.T) (repo, map[string]string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	r := repo{t, root}
	r.git("init", "-q", "-b", "develop")
	h := map[string]string{}
	r.commit("a.txt", "a\n", "chore: init")
	r.git("checkout", "-q", "-b", "release/2.4.0")
	h["missing"] = r.commit("m.txt", "m\n", "fix: missing on the newer line")
	h["trailer"] = r.commit("t.txt", "t\n", "fix: ported by trailer")
	h["pick"] = r.commit("c.txt", "c\n", "fix: ported by cherry-pick")
	h["patch"] = r.commit("p.txt", "p\n", "fix(cart): ported by patch-id")
	h["lost"] = r.commit("l.txt", "l\n", "fix: lost to a revert")
	r.commit("n.txt", "n\n", "feat: not a fix")
	h["marked"] = r.commit("y.txt", "y\n", "chore: marked\n\nPort: yes")
	h["dropped"] = r.commit("d.txt", "d\n", "fix: dropped on its own line")
	r.git("revert", "--no-edit", "HEAD")
	r.git("checkout", "-q", "-b", "hotfix/2.4.5")
	h["hot"] = r.commit("h.txt", "h\n", "fix: crash")
	r.git("checkout", "-q", "develop")
	r.commit("b.txt", "b\n", "feat: next")
	r.git("checkout", "-q", "-b", "release/2.5.0")
	h["trailer-port"] = r.commit("t.txt", "t, adapted\n", "fix: ported by trailer\n\nPorted-from: "+h["trailer"])
	r.git("cherry-pick", "-x", h["pick"])
	h["pick-port"] = r.git("rev-parse", "--short=7", "HEAD")
	h["patch-port"] = r.commit("p.txt", "p\n", "fix: same change, other words")
	r.git("cherry-pick", h["lost"])
	h["lost-port"] = r.git("rev-parse", "--short=7", "HEAD")
	r.git("revert", "--no-edit", "HEAD")
	h["revert"] = r.git("rev-parse", "--short=7", "HEAD")
	r.git("checkout", "-q", "develop")
	return r, h
}

func run(t *testing.T, r repo, from, to string) Pair {
	t.Helper()
	p := testProfile(t)
	all, err := lines.List(r.dir, p.ReleaseLines)
	if err != nil {
		t.Fatal(err)
	}
	pair, err := Check(r.dir, p.Commit, all, *lines.Find(all, from), *lines.Find(all, to))
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func statuses(pair Pair) []string {
	var out []string
	for _, fix := range pair.Fixes {
		out = append(out, short(fix.Hash)+" "+fix.Status.String()+" "+short(fix.By)+" "+short(fix.RevertedBy))
	}
	return out
}

func TestCheckFindsEachPortAndTheOnesMissingOrLost(t *testing.T) {
	r, h := fixture(t)
	pair := run(t, r, "2.4.0", "2.5.0")
	want := []string{
		h["missing"] + " missing  ",
		h["trailer"] + " trailer " + h["trailer-port"] + " ",
		h["pick"] + " cherry-pick " + h["pick-port"] + " ",
		h["patch"] + " patch-id " + h["patch-port"] + " ",
		h["lost"] + " lost " + h["lost-port"] + " " + h["revert"],
		h["marked"] + " missing  ",
	}
	if got := statuses(pair); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if pair.Problems() != 3 {
		t.Errorf("problems %d", pair.Problems())
	}
}

func TestAFixMergedUpwardIsFoundByAncestryAndARevertAfterTheMergeLosesIt(t *testing.T) {
	r, h := fixture(t)
	pair := run(t, r, "2.4.0", "2.4.5")
	for _, fix := range pair.Fixes {
		if fix.Status != Merged || fix.By != fix.Hash {
			t.Errorf("%s %s", short(fix.Hash), fix.Status)
		}
	}
	r.git("checkout", "-q", "hotfix/2.4.5")
	r.git("revert", "--no-edit", h["trailer"])
	revert := r.git("rev-parse", "--short=7", "HEAD")
	pair = run(t, r, "2.4.0", "2.4.5")
	for _, fix := range pair.Fixes {
		if short(fix.Hash) == h["trailer"] && (fix.Status != Lost || short(fix.RevertedBy) != revert) {
			t.Errorf("%s %s %s", short(fix.Hash), fix.Status, fix.RevertedBy)
		}
	}
	r.git("revert", "--no-edit", "HEAD")
	if pair = run(t, r, "2.4.0", "2.4.5"); pair.Problems() != 0 {
		t.Errorf("a reverted revert still loses the fix: %q", statuses(pair))
	}
}

// TestRevertingAPortMergeLosesTheFix is the case from the P6 live run: the port workflow merges the older line with
// --no-ff and a Ported-from trailer, and `git revert -m 1` of that merge on the newer line undoes the fix while the fix
// stays in its history. A later revert of that revert brings it back.
func TestRevertingAPortMergeLosesTheFix(t *testing.T) {
	r, h := fixture(t)
	r.git("checkout", "-q", "-b", "release/2.6.0", "develop")
	r.git("merge", "-q", "--no-ff", "--no-commit", "release/2.4.0")
	r.git("commit", "-q", "-m", "fix: port 2.4.0 fixes\n\nPorted-from: "+h["trailer"])
	merge := r.git("rev-parse", "--short=7", "HEAD")
	pair := run(t, r, "2.4.0", "2.6.0")
	for _, fix := range pair.Fixes {
		if fix.Status != Merged {
			t.Errorf("before the revert: %s %s", short(fix.Hash), fix.Status)
		}
	}
	r.git("revert", "--no-edit", "-m", "1", merge)
	revert := r.git("rev-parse", "--short=7", "HEAD")
	pair = run(t, r, "2.4.0", "2.6.0")
	if len(pair.Fixes) == 0 {
		t.Fatal("no fixes")
	}
	for _, fix := range pair.Fixes {
		if fix.Status != Lost || short(fix.RevertedBy) != revert {
			t.Errorf("after the revert: %s %s by %s", short(fix.Hash), fix.Status, short(fix.RevertedBy))
		}
	}
	r.git("revert", "--no-edit", "HEAD")
	if pair = run(t, r, "2.4.0", "2.6.0"); pair.Problems() != 0 {
		t.Errorf("a reverted revert still loses the fixes: %q", statuses(pair))
	}
}

func TestTheHotfixLineCarriesTheFixesItInheritedSoPairsChain(t *testing.T) {
	r, h := fixture(t)
	pair := run(t, r, "2.4.5", "2.5.0")
	var got []string
	for _, fix := range pair.Fixes {
		got = append(got, short(fix.Hash))
	}
	want := []string{h["missing"], h["trailer"], h["pick"], h["patch"], h["lost"], h["marked"], h["hot"]}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestAPortOfAPortMatchesTheOriginal(t *testing.T) {
	r, h := fixture(t)
	r.git("checkout", "-q", "-b", "port-of-port", "develop")
	r.commit("t.txt", "t, again\n", "fix: ported by trailer\n\nPorted-from: "+h["trailer-port"])
	r.git("checkout", "-q", "develop")
	r.git("merge", "-q", "--ff-only", "port-of-port")
	pair := run(t, r, "2.5.0", "develop")
	for _, fix := range pair.Fixes {
		if short(fix.Hash) == h["trailer-port"] && fix.Status != Trailer {
			t.Errorf("%s %s", short(fix.Hash), fix.Status)
		}
	}
}

func TestLinesPrintsOneLinePerFix(t *testing.T) {
	pair := Pair{From: lines.Line{Version: "2.4.0", Branch: "release/2.4.0"}, To: lines.Line{Kind: profile.MainLine, Branch: "develop"},
		Base: "1234567890", BaseFrom: "develop", Fixes: []Fix{
			{Hash: "aaaaaaa111", Subject: "fix: a", Status: Merged, By: "aaaaaaa111"},
			{Hash: "bbbbbbb222", Subject: "fix: b", Status: Trailer, By: "ccccccc333"},
			{Hash: "ddddddd444", Subject: "fix: d", Status: Missing},
			{Hash: "eeeeeee555", Subject: "fix: e", Status: Lost, By: "fffffff666", RevertedBy: "9999999777"},
		}}
	want := []string{
		"release/2.4.0 -> develop: 4 fixes since develop at 1234567, 2 missing or lost",
		"  ok       aaaaaaa fix: a (merged)",
		"  ok       bbbbbbb fix: b (Ported-from: in ccccccc)",
		"  missing  ddddddd fix: d",
		"  lost     eeeeeee fix: e (ported as fffffff, reverted by 9999999)",
	}
	if got := pair.Lines(); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
