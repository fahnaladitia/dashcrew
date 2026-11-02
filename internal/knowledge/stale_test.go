package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// staleFixture is a project checked out as `work` whose origin is notes-app, a sibling clone of notes-api, and a
// knowledge clone with fresh, stale and unverified pages and an old and a new proposal.
func staleFixture(t *testing.T) (clone, project string) {
	t.Helper()
	gitEnv(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, api := filepath.Join(root, "work"), filepath.Join(root, "notes-api")
	for _, dir := range []string{project, api} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		git(t, dir, "init", "-q", "-b", "main")
	}
	git(t, project, "remote", "add", "origin", "git@example.com:team/notes-app.git")
	commitFile(t, project, "src/a.go", "package a\n")
	writeTree(t, project, map[string]string{"src/uncommitted.go": "package a\n"})
	commitFile(t, api, "src/share/token.go", "package share\n")

	clone = filepath.Join(root, "kb")
	writeTree(t, clone, map[string]string{
		"features/share.md": "---\ntitle: Share\n---\n\nUses `notes-app:src/a.go:1`, `notes-app:src/gone.go:3` and `notes-app:src/uncommitted.go:1`.\n" +
			"The token is in `notes-api:src/share/token.go:4`, the screen in `notes-web:src/x.ts:1`. <!-- verified notes-app@abcdef1 2026-02-01 -->\n",
		"repos/notes-app.md":         "---\ntitle: notes-app\n---\n\nOld. <!-- verified notes-app@abcdef1 2025-01-01 -->\n",
		"features/tags.md":           "---\ntitle: Tags\n---\n\nNever verified.\n",
		"inbox/.gitkeep":             "",
		"inbox/2026-02-01-old.md":    "x\n",
		"inbox/2026-02-20-recent.md": "x\n",
	})
	return clone, project
}

func TestStalenessFindsGonePointersOldProposalsAndUnverifiedPages(t *testing.T) {
	clone, project := staleFixture(t)
	report, err := Staleness(clone, project, time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	wantMissing := []StalePointer{
		{Page: "features/share.md", Pointer: Pointer{Repo: "notes-app", Path: "src/gone.go", CodeLine: 3, Line: 5}, RepoDir: project},
		{Page: "features/share.md", Pointer: Pointer{Repo: "notes-app", Path: "src/uncommitted.go", CodeLine: 1, Line: 5}, RepoDir: project},
	}
	if !reflect.DeepEqual(report.Missing, wantMissing) {
		t.Errorf("missing %+v", report.Missing)
	}
	if report.Checked != 4 || !reflect.DeepEqual(report.Unresolved, []string{"notes-web"}) {
		t.Errorf("checked %d, unresolved %v", report.Checked, report.Unresolved)
	}
	if !reflect.DeepEqual(report.OldProposals, []OldProposal{{Path: "inbox/2026-02-01-old.md", Days: 28}}) {
		t.Errorf("old proposals %+v", report.OldProposals)
	}
	wantUnverified := []UnverifiedPage{{Path: "features/tags.md"}, {Path: "repos/notes-app.md", Newest: "2025-01-01", Days: 424}}
	if !reflect.DeepEqual(report.Unverified, wantUnverified) {
		t.Errorf("unverified %+v", report.Unverified)
	}
}

func TestStalenessResolvesTheProjectByItsFolderNameAndFallsBackToFilesOutsideGit(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "notes-app")
	clone := filepath.Join(root, "kb")
	writeTree(t, project, map[string]string{"src/a.go": "package a\n"})
	writeTree(t, clone, map[string]string{
		"features/a.md":  "---\ntitle: A\n---\n\n`notes-app:src/a.go:1` `notes-app:src/b.go:1` <!-- verified notes-app@abcdef1 2026-02-01 -->\n",
		"inbox/.gitkeep": "",
	})
	report, err := Staleness(clone, project, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if report.Checked != 2 || len(report.Missing) != 1 || report.Missing[0].Pointer.Path != "src/b.go" {
		t.Errorf("%+v", report)
	}
}
