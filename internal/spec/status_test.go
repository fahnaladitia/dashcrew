package spec

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, clone, folder, status string, rev, trdRev int, body string) {
	t.Helper()
	for name, r := range map[string]int{"prd.md": rev, "trd.md": trdRev} {
		file := filepath.Join(clone, filepath.FromSlash(folder), name)
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		text := "---\ntitle: Notes sync\nfeature: notes-sync\nrelease: \"1.4.0\"\nstatus: " + status + "\nrev: " + strconv.Itoa(r) +
			"\nrepos: [notes-api, notes-app, notes-web]\nowners: [product]\n---\n\n"
		if name == "prd.md" {
			text += body
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParseRef(t *testing.T) {
	for value, want := range map[string]Ref{
		"specs/1.4.0/notes-sync@rev2":        {Folder: "specs/1.4.0/notes-sync", Rev: 2},
		" specs/unreleased/shop-cart@rev12 ": {Folder: "specs/unreleased/shop-cart", Rev: 12},
	} {
		got, ok := ParseRef(value)
		if !ok || got != want {
			t.Errorf("%q: %+v %v", value, got, ok)
		}
		if ok && got.String() != strings.TrimSpace(value) {
			t.Errorf("%q: String %q", value, got.String())
		}
	}
	for _, value := range []string{"", "none", "specs/1.4.0/notes-sync", "specs/1.4.0/notes-sync@rev0", "specs/notes-sync@rev1",
		"features/x@rev1", "specs/1.4.0/notes-sync/prd.md@rev1", "specs/../x@rev1", "specs/1.4.0/notes-sync@revx"} {
		if got, ok := ParseRef(value); ok {
			t.Errorf("%q accepted: %+v", value, got)
		}
	}
}

func TestLoadReadsTheCloneState(t *testing.T) {
	clone := tempClone(t)
	writeSpec(t, clone, "specs/1.4.0/notes-sync", "approved", 3, 3,
		"- AC-1 Given a change, then the other device shows it.\n- ~~AC-2~~ dropped: later\n- AC-3 Given no network, then the edit is kept.\n")
	state, err := Load(clone, "specs/1.4.0/notes-sync")
	if err != nil {
		t.Fatal(err)
	}
	if state.Rev != 3 || state.Status != Approved || !reflect.DeepEqual(state.ACs, []string{"AC-1", "AC-3"}) ||
		!reflect.DeepEqual(state.Repos, []string{"notes-api", "notes-app", "notes-web"}) {
		t.Errorf("%+v", state)
	}
	writeSpec(t, clone, "specs/1.4.0/notes-sync", "approved", 4, 3, "- AC-1 x\n")
	if err := os.WriteFile(filepath.Join(clone, "specs/1.4.0/notes-sync/trd.md"),
		[]byte("---\ntitle: x\nstatus: review\nrev: 4\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if state, _ := Load(clone, "specs/1.4.0/notes-sync"); state.Rev != 4 || state.Status != Review {
		t.Errorf("mixed files: the highest rev and the earliest status: %+v", state)
	}
	if _, err := Load(clone, "specs/1.4.0/missing"); err == nil {
		t.Error("a missing spec loaded")
	}
}

func TestClaimsReadPlansThatNameTheSpec(t *testing.T) {
	project := tempClone(t)
	for path, text := range map[string]string{
		".ai/plans/1.4.0/sync-client/plan.md": "Spec: specs/1.4.0/notes-sync@rev2\n\nCovers AC-1 and AC-3.\n\nAC-1  test/sync_test  pulls changes\n",
		".ai/plans/sync-banner/plan.md":       "# Banner\n\nSpec: specs/1.4.0/notes-sync@rev1, AC-4.\n",
		".ai/plans/other/plan.md":             "Spec: specs/1.4.0/shop-cart@rev1\n\nAC-2\n",
		".ai/plans/notes/notes.txt":           "specs/1.4.0/notes-sync AC-5\n",
		".ai/plans/near/plan.md":              "specs/1.4.0/notes-sync-v2@rev1 AC-6\n",
	} {
		file := filepath.Join(project, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	claims, err := Claims(project, ".ai/plans", "specs/1.4.0/notes-sync")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"AC-1": {".ai/plans/1.4.0/sync-client/plan.md"},
		"AC-3": {".ai/plans/1.4.0/sync-client/plan.md"},
		"AC-4": {".ai/plans/sync-banner/plan.md"},
	}
	if !reflect.DeepEqual(claims, want) {
		t.Errorf("%v", claims)
	}
	if claims, err := Claims(project, "missing", "specs/1.4.0/notes-sync"); err != nil || len(claims) != 0 {
		t.Errorf("no plans folder: %v %v", claims, err)
	}
}

func TestDrift(t *testing.T) {
	recorded := Ref{Folder: "specs/1.4.0/notes-sync", Rev: 2}
	for _, c := range []struct {
		state State
		own   int
		want  []string
	}{
		{State{Rev: 2, Status: Approved}, 3, nil},
		{State{Rev: 4, Status: Review}, 0, []string{"spec moved to rev 4; re-read it"}},
		{State{Rev: 2, Status: Draft}, 0, nil},
		{State{Rev: 2, Status: Draft}, 1, []string{"spec is still draft while the branch has 1 commit past its base; get it reviewed"}},
		{State{Rev: 3, Status: Draft}, 2, []string{"spec moved to rev 3; re-read it", "spec is still draft while the branch has 2 commits past its base; get it reviewed"}},
	} {
		if got := Drift(recorded, c.state, c.own); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%+v %d: %q", c.state, c.own, got)
		}
	}
}

func TestRepoNamesAreTheFolderAndTheOriginRemote(t *testing.T) {
	dir := filepath.Join(tempClone(t), "work")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := RepoNames(dir); got != nil {
		t.Errorf("outside a repository: %v", got)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "git@example.com:team/notes-app.git"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}
	if got := RepoNames(dir); !reflect.DeepEqual(got, []string{"work", "notes-app"}) {
		t.Errorf("%v", got)
	}
}
