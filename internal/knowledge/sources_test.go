package knowledge

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCheckPointersMarksEachFoundMissingOrUnchecked(t *testing.T) {
	clone, project := staleFixture(t)
	data, err := os.ReadFile(filepath.Join(clone, "features", "share.md"))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, check := range CheckPointers(project, Parse(string(data)).Pointers) {
		got = append(got, check.Pointer.String()+" "+check.Status)
	}
	want := []string{
		"notes-app:src/a.go:1 found",
		"notes-app:src/gone.go:3 missing",
		"notes-app:src/uncommitted.go:1 missing",
		"notes-api:src/share/token.go:4 found",
		"notes-web:src/x.ts:1 unchecked",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("checks %v", got)
	}
}

func TestPageFileKeepsPagesInsideTheClone(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clone := filepath.Join(root, "kb")
	writeTree(t, clone, map[string]string{
		"features/share.md":  "---\ntitle: Share\n---\n",
		"features/notes.txt": "x\n",
		".git/HEAD":          "ref: refs/heads/main\n",
		".git/notes.md":      "x\n",
	})
	writeTree(t, root, map[string]string{"secret.md": "x\n"})
	if err := os.Symlink(filepath.Join(root, "secret.md"), filepath.Join(clone, "features", "escape.md")); err != nil {
		t.Fatal(err)
	}

	for _, page := range []string{"features/share.md", "features/share", " features/share.md ", "./features/share.md"} {
		rel, file, err := PageFile(clone, page)
		if err != nil || rel != "features/share.md" || file != filepath.Join(clone, "features", "share.md") {
			t.Errorf("%q: %q %q %v", page, rel, file, err)
		}
	}
	for _, page := range []string{
		"", "../secret.md", "./features/../features/share.md", "features/../../secret.md", "/etc/passwd", filepath.Join(root, "secret.md"),
		`features\..\..\secret.md`, ".git/notes.md", "features/notes.txt", "features/escape.md", "features/none.md",
	} {
		if rel, _, err := PageFile(clone, page); err == nil {
			t.Errorf("%q accepted as %q", page, rel)
		}
	}
	if _, _, err := PageFile(clone, "../secret.md"); err == nil || !strings.Contains(err.Error(), "inside the knowledge clone") {
		t.Errorf("traversal error %v", err)
	}
}
