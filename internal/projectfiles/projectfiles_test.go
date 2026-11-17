package projectfiles

import (
	"os"
	"path/filepath"
	"runtime"
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

func TestWriteIfChangedLeavesNoTempFileBehind(t *testing.T) {
	project := New(t.TempDir())

	for _, content := range []string{"one", "two"} {
		if _, err := project.WriteIfChanged("a/c.txt", content); err != nil {
			t.Fatal(err)
		}
	}

	if names := entries(t, project.Path("a")); len(names) != 1 || names[0] != "c.txt" {
		t.Errorf("folder holds %v", names)
	}
}

func TestWriteIfChangedKeepsTheModeOfAnExistingFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no execute bits on Windows")
	}
	project := New(t.TempDir())
	if err := os.WriteFile(project.Path("run.sh"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := project.WriteIfChanged("run.sh", "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := project.WriteIfChanged("fresh.txt", "new"); err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]os.FileMode{"run.sh": 0o755, "fresh.txt": 0o644} {
		if info, err := os.Stat(project.Path(path)); err != nil || info.Mode().Perm() != want {
			t.Errorf("%s: mode %v, %v", path, info.Mode().Perm(), err)
		}
	}
}

func TestWriteAtomicRemovesItsTempFileWhenTheRenameFails(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "taken")
	if err := os.MkdirAll(filepath.Join(target, "inside"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(target, []byte("x"), 0o644); err == nil {
		t.Fatal("rename over a folder succeeded")
	}

	if names := entries(t, dir); len(names) != 1 || names[0] != "taken" {
		t.Errorf("folder holds %v", names)
	}
}

func TestWriteAtomicWritesThroughALinkToItsTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on Windows")
	}
	dir, dotfiles := t.TempDir(), t.TempDir()
	source := filepath.Join(dotfiles, "AGENTS.md")
	if err := os.WriteFile(source, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "AGENTS.md")
	if err := os.Symlink(source, link); err != nil {
		t.Fatal(err)
	}

	if err := WriteAtomic(link, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("link replaced: %v", err)
	}
	if data, _ := os.ReadFile(source); string(data) != "new" {
		t.Errorf("target holds %q", data)
	}
	if names := entries(t, dotfiles); len(names) != 1 {
		t.Errorf("target folder holds %v", names)
	}
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, entry := range list {
		names = append(names, entry.Name())
	}
	return names
}
