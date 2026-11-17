// Package projectfiles reads and writes files of one project by paths relative to its root.
// It never writes through a symbolic link inside the project: the link is replaced, its target left alone.
package projectfiles

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Project is a project root.
type Project struct {
	Root string
}

// New returns the project at root.
func New(root string) Project { return Project{Root: root} }

// Path is the absolute path of a slash-separated relative path.
func (p Project) Path(relative string) string {
	return filepath.Join(p.Root, filepath.FromSlash(relative))
}

// Exists reports whether a regular file (or a link to one) exists at relative.
func (p Project) Exists(relative string) bool {
	info, err := os.Stat(p.Path(relative))
	return err == nil && !info.IsDir()
}

// Read returns the file content; a missing file is an error wrapping fs.ErrNotExist.
func (p Project) Read(relative string) (string, error) {
	data, err := os.ReadFile(p.Path(relative))
	return string(data), err
}

// Link is a symbolic link found on the way to a path: its project-relative path and its target.
type Link struct {
	Path   string
	Target string
}

// LinkWithin returns the first part of relative that is a symbolic link; nil when none is.
func (p Project) LinkWithin(relative string) *Link {
	current := p.Root
	parts := strings.Split(filepath.ToSlash(relative), "/")
	for i, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			return nil
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			target, _ := os.Readlink(current)
			return &Link{Path: strings.Join(parts[:i+1], "/"), Target: target}
		}
		if !info.IsDir() {
			return nil
		}
	}
	return nil
}

// WriteIfChanged writes content unless the file already holds it; true when it wrote.
// A link on the way is removed first, so the write lands in the project and never in the link's target.
func (p Project) WriteIfChanged(relative, content string) (bool, error) {
	link := p.LinkWithin(relative)
	if link == nil {
		if current, err := p.Read(relative); err == nil && current == content {
			return false, nil
		}
	} else {
		if err := os.Remove(p.Path(link.Path)); err != nil {
			return false, err
		}
	}
	target := p.Path(relative)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return false, err
	}
	return true, WriteAtomic(target, []byte(content), 0o644)
}

// WriteAtomic replaces the file at path in one rename, so an interrupted run leaves the old content or the new,
// never part of it. An existing file keeps its permission bits; a new one gets perm. A link at path is followed:
// its target is replaced and the link stays.
func WriteAtomic(path string, data []byte, perm fs.FileMode) error {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		perm = info.Mode().Perm()
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".dashcrew-*")
	if err != nil {
		return err
	}
	if err := fill(temp, data, perm); err != nil {
		os.Remove(temp.Name())
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		os.Remove(temp.Name())
		return err
	}
	return nil
}

// fill writes data to file, flushes it to disk, sets perm and closes it.
func fill(file *os.File, data []byte, perm fs.FileMode) error {
	_, err := file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	if err == nil {
		err = file.Chmod(perm)
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}
