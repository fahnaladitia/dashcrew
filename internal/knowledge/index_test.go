package knowledge

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestIndexGroupsPagesByRepoThenTag(t *testing.T) {
	clone := t.TempDir()
	writeTree(t, clone, map[string]string{
		"features/share.md": "---\ntitle: Note sharing\nrepos: [notes-api, notes-app]\ntags: [sharing]\n---\n\n# Note sharing\n\n## Purpose\n\n" +
			"Share a note\nwith a link. Links expire. <!-- verified notes-api@abcdef1 2026-02-01 -->\n\nMore. <!-- verified notes-app@abcdef1 2026-02-10 -->\n",
		"repos/notes-api.md": "---\ntitle: notes-api\nrepos: [notes-api]\n---\n\n- list first\n\nSetup: run `make dev` first\n",
		"features/orphan.md": "# Orphan\n\n```\ncode. here\n```\n",
		"features/.gitkeep":  "",
		"inbox/x.md":         "---\ntitle: Not a page\n---\n",
	})
	readme := "# Knowledge\n\nIntro.\n\n## Pages\n\nNone reviewed yet.\n"
	got, err := UpdateReadme(readme, clone)
	if err != nil {
		t.Fatal(err)
	}
	want := "# Knowledge\n\nIntro.\n\n## Pages\n\n" + indexNote + "\n\n" +
		"### notes-api\n\n#### sharing\n\n" +
		"- [Note sharing](features/share.md): Share a note with a link. Verified 2026-02-10.\n\n" +
		"#### (no tag)\n\n" +
		"- [notes-api](repos/notes-api.md): Setup: run `make dev` first. Not verified.\n\n" +
		"### notes-app\n\n#### sharing\n\n" +
		"- [Note sharing](features/share.md): Share a note with a link. Verified 2026-02-10.\n\n" +
		"### (no repo)\n\n#### (no tag)\n\n" +
		"- [Orphan](features/orphan.md): Not verified.\n"
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestIndexWithoutPagesOrAPagesSection(t *testing.T) {
	got, err := UpdateReadme("# Knowledge\n", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Knowledge\n\n## Pages\n\nNone reviewed yet.\n"; got != want {
		t.Errorf("got\n%s", got)
	}
}
