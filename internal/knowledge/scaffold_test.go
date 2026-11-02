package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestInitWritesTheLayoutIntoANewFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "team-knowledge")
	written, err := Init(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"README.md", "features/.gitkeep", "inbox/.gitkeep", "repos/.gitkeep", "templates/page.md"}
	if !reflect.DeepEqual(written, want) {
		t.Errorf("written %v", written)
	}
	for _, file := range want {
		if _, err := os.Stat(filepath.Join(dir, file)); err != nil {
			t.Error(err)
		}
	}
	readme, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	for _, part := range []string{"features/", "repos/", "inbox/", markerShape, "<repo>:<path>:<line>"} {
		if !strings.Contains(string(readme), part) {
			t.Errorf("README.md lacks %q", part)
		}
	}
	template, _ := os.ReadFile(filepath.Join(dir, "templates", "page.md"))
	page := Parse(string(template))
	if !page.FrontMatter || page.Title == "" || len(page.Headings) < 4 {
		t.Errorf("template parses as %+v", page)
	}
}

func TestInitUsesAnEmptyFolderAndRefusesAFolderWithFiles(t *testing.T) {
	empty := t.TempDir()
	if _, err := Init(empty); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(empty); !errors.Is(err, ErrNotEmpty) {
		t.Errorf("second init: %v", err)
	}
	readme, _ := os.ReadFile(filepath.Join(empty, "README.md"))
	if len(readme) == 0 {
		t.Error("README.md was emptied")
	}

	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(file); err == nil || !strings.Contains(err.Error(), "is not a folder") {
		t.Errorf("init on a file: %v", err)
	}

	hidden := t.TempDir()
	if err := os.Mkdir(filepath.Join(hidden, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(hidden); !errors.Is(err, ErrNotEmpty) {
		t.Errorf("a folder holding only .git: %v", err)
	}
}
