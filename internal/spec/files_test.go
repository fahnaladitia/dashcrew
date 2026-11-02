package spec

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func tempClone(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(root, "notes")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatal(err)
	}
	return clone
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNewWritesTheTemplatesOnFirstUseThenTheSpec(t *testing.T) {
	clone := tempClone(t)
	written, err := New(clone, "notes-sync", "1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"templates/prd.md", "specs/1.4.0/notes-sync/prd.md", "templates/trd.md", "specs/1.4.0/notes-sync/trd.md"}
	if !reflect.DeepEqual(written, want) {
		t.Errorf("%v", written)
	}
	for _, name := range Files {
		doc := Parse(read(t, filepath.Join(clone, "specs", "1.4.0", "notes-sync", name)))
		if doc.Title != "Notes sync" || doc.Feature != "notes-sync" || doc.Release != "1.4.0" || doc.Status != Draft || doc.Rev != 1 {
			t.Errorf("%s: %+v", name, doc)
		}
		if problems := Lint(doc, nil, true); len(problems) != 0 {
			t.Errorf("%s: a new spec lints clean: %v", name, problems)
		}
	}
	if written, err := New(clone, "notes-sync", "1.4.0"); !errors.Is(err, ErrExists) || len(written) != 0 {
		t.Errorf("existing folder: %v %v", written, err)
	}
}

func TestNewUsesTheCloneTemplatesAndForcesDraftRev1(t *testing.T) {
	clone := tempClone(t)
	if err := writeFile(filepath.Join(clone, "templates", "prd.md"), "---\ntitle: {{title}}\nfeature: {{feature}}\nrelease: {{release}}\nstatus: approved\nrev: 7\n---\nTeam PRD\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(clone, "templates", "trd.md"), "---\ntitle: {{title}}\n---\nTeam TRD\n"); err != nil {
		t.Fatal(err)
	}
	written, err := New(clone, "todo", Unreleased)
	if err != nil || !reflect.DeepEqual(written, []string{"specs/unreleased/todo/prd.md", "specs/unreleased/todo/trd.md"}) {
		t.Fatalf("%v %v", written, err)
	}
	if got := read(t, filepath.Join(clone, "specs", "unreleased", "todo", "prd.md")); got != "---\ntitle: Todo\nfeature: todo\nrelease: unreleased\nstatus: draft\nrev: 1\n---\nTeam PRD\n" {
		t.Errorf("prd:\n%s", got)
	}
	if got := read(t, filepath.Join(clone, "specs", "unreleased", "todo", "trd.md")); got != "---\ntitle: Todo\nstatus: draft\nrev: 1\n---\nTeam TRD\n" {
		t.Errorf("trd:\n%s", got)
	}
}

func TestNewRefusesBadNamesAndAMissingClone(t *testing.T) {
	clone := tempClone(t)
	for _, c := range [][2]string{{"Notes", "1.0.0"}, {"notes_sync", "1.0.0"}, {"notes", "../x"}, {"notes", "1..0"}, {"notes", ""}} {
		if _, err := New(clone, c[0], c[1]); err == nil {
			t.Errorf("%v accepted", c)
		}
	}
	if _, err := New(filepath.Join(clone, "nope"), "notes", "1.0.0"); err == nil || !strings.Contains(err.Error(), "dashcrew knowledge pull") {
		t.Errorf("%v", err)
	}
}

func approvedSpec(t *testing.T, clone string, prdRev, trdRev int) string {
	t.Helper()
	if _, err := New(clone, "notes-sync", "1.4.0"); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(clone, "specs", "1.4.0", "notes-sync")
	for name, rev := range map[string]int{"prd.md": prdRev, "trd.md": trdRev} {
		file := filepath.Join(folder, name)
		text := setField(setField(read(t, file), "status", Approved), "rev", string(rune('0'+rev)))
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return folder
}

func TestReviseSetsBothFilesToReviewPastTheHighestRev(t *testing.T) {
	clone := tempClone(t)
	folder := approvedSpec(t, clone, 2, 1)
	before := read(t, filepath.Join(folder, "trd.md"))
	r, err := Revise(filepath.Join(folder, "prd.md"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Root != clone || r.Folder != "specs/1.4.0/notes-sync" || r.Rev != 3 || !reflect.DeepEqual(r.From, []int{2, 1}) ||
		!reflect.DeepEqual(r.Files, []string{"specs/1.4.0/notes-sync/prd.md", "specs/1.4.0/notes-sync/trd.md"}) {
		t.Errorf("%+v", r)
	}
	after := read(t, filepath.Join(folder, "trd.md"))
	if want := strings.Replace(strings.Replace(before, "status: approved", "status: review", 1), "rev: 1", "rev: 3", 1); after != want {
		t.Errorf("trd:\n%s", after)
	}
	if _, err := Revise(folder); err == nil || !strings.Contains(err.Error(), "prd.md is review, not approved") {
		t.Errorf("second revise: %v", err)
	}
}

func TestReviseWritesNothingUnlessEveryFileIsApproved(t *testing.T) {
	clone := tempClone(t)
	folder := approvedSpec(t, clone, 1, 1)
	trd := filepath.Join(folder, "trd.md")
	if err := os.WriteFile(trd, []byte(setField(read(t, trd), "status", Superseded)), 0o644); err != nil {
		t.Fatal(err)
	}
	prd := read(t, filepath.Join(folder, "prd.md"))
	if _, err := Revise(folder); err == nil || !strings.Contains(err.Error(), "trd.md is superseded") {
		t.Errorf("%v", err)
	}
	if read(t, filepath.Join(folder, "prd.md")) != prd {
		t.Error("prd.md was written")
	}
	for _, path := range []string{filepath.Join(clone, "specs"), clone, filepath.Join(clone, "missing")} {
		if _, err := Revise(path); err == nil {
			t.Errorf("%s accepted", path)
		}
	}
}

func TestRootAndDisplay(t *testing.T) {
	root, relative, ok := Root("/home/a/specs-old/kb/specs/1.0/x/prd.md")
	// Root makes the path absolute first, which on Windows puts it on the current drive.
	want, err := filepath.Abs(filepath.FromSlash("/home/a/specs-old/kb"))
	if err != nil {
		t.Fatal(err)
	}
	if !ok || root != want || relative != "specs/1.0/x/prd.md" {
		t.Errorf("%s %s %v", root, relative, ok)
	}
	if got := Display("notes/prd.md"); got != "notes/prd.md" {
		t.Errorf("%s", got)
	}
}

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestLintFilesAgainstAGitRef(t *testing.T) {
	clone := tempClone(t)
	folder := approvedSpec(t, clone, 1, 1)
	gitIn(t, clone, "init", "-q", "-b", "main")
	gitIn(t, clone, "add", "-A")
	gitIn(t, clone, "commit", "-q", "-m", "spec")
	prd := filepath.Join(folder, "prd.md")
	text := strings.Replace(read(t, prd), "- AC-1 Given", "- AC-2 Given", 1)
	if err := os.WriteFile(prd, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(filepath.Join(clone, "specs", "2.0.0", "todo", "prd.md"), strings.Replace(text, "rev: 1", "rev: 4", 1)); err != nil {
		t.Fatal(err)
	}
	files, err := FilesUnder(filepath.Join(clone, "specs"))
	if err != nil {
		t.Fatal(err)
	}
	results, err := LintFiles(files, "main")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range results {
		for _, p := range r.Problems {
			got = append(got, r.Path+": "+p.String())
		}
	}
	want := []string{
		"specs/1.4.0/notes-sync/prd.md: AC-1 is gone (it was on line 32 at main); keep it struck through: `~~AC-1~~ dropped: <why>`",
		"specs/1.4.0/notes-sync/prd.md: line 6: the file was approved at main and changed without a rev increase; a member runs dashcrew spec revise first",
		"specs/1.4.0/notes-sync/prd.md: line 32: AC-2 holds the text of AC-1 at main; IDs are never renumbered",
		"specs/2.0.0/todo/prd.md: line 32: AC IDs skip AC-1: a new spec numbers them from AC-1 without gaps",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%q", got)
	}
	if _, err := LintFiles(files, "nope"); err == nil || !strings.Contains(err.Error(), "unknown git ref nope") {
		t.Errorf("%v", err)
	}
	if _, err := LintFiles([]string{filepath.Join(t.TempDir(), "prd.md")}, "main"); err == nil {
		t.Error("a missing file linted")
	}
}
