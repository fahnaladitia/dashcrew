package projectfiles

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteIfChangedWritesOnceAndCreatesFolders(t *testing.T) {
	project := New(t.TempDir())

	wrote, err := project.WriteIfChanged("a/b/c.txt", "x")
	if err != nil || !wrote {
		t.Fatalf("first write %v %v", wrote, err)
	}
	wrote, err = project.WriteIfChanged("a/b/c.txt", "x")
	if err != nil || wrote {
		t.Fatalf("second write %v %v", wrote, err)
	}
	if content, _ := project.Read("a/b/c.txt"); content != "x" {
		t.Errorf("content %q", content)
	}
}

func TestWriteThroughALinkReplacesTheLink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	source := filepath.Join(outside, "settings.json")
	if err := os.WriteFile(source, []byte("theirs"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(source, filepath.Join(root, ".claude", "settings.json")); err != nil {
		t.Fatal(err)
	}
	project := New(root)

	link := project.LinkWithin(".claude/settings.json")
	if link == nil || link.Path != ".claude/settings.json" || link.Target != source {
		t.Fatalf("link %+v", link)
	}
	if _, err := project.WriteIfChanged(".claude/settings.json", "ours"); err != nil {
		t.Fatal(err)
	}

	if data, _ := os.ReadFile(source); string(data) != "theirs" {
		t.Errorf("link target changed: %q", data)
	}
	if info, _ := os.Lstat(filepath.Join(root, ".claude", "settings.json")); info.Mode()&os.ModeSymlink != 0 {
		t.Error("still a link")
	}
	if project.LinkWithin(".claude/settings.json") != nil {
		t.Error("link remains")
	}
}
