// Package eject copies a built-in platform into a project's `.ai/platforms/<name>/` (`dashcrew platform new`), where
// it replaces the built-in one. The team owns the folder afterwards: it is written once and never updated by dashcrew.
package eject

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Error is a refusal meant for the user.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// Names are the built-in platforms in fsys (`platforms/<name>/`), sorted.
func Names(fsys fs.FS) []string {
	entries, _ := fs.ReadDir(fsys, "platforms")
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Options are the pack options built-in platform name ships for family, sorted; nil when it has none.
func Options(fsys fs.FS, name, family string) []string {
	entries, _ := fs.ReadDir(fsys, path.Join("platforms", name, "packs", family))
	var options []string
	for _, entry := range entries {
		if entry.IsDir() {
			options = append(options, entry.Name())
		}
	}
	sort.Strings(options)
	return options
}

// Write copies built-in platform name into projectDir/.ai/platforms/<name>/, its platform.yaml headed by a comment
// naming version, and returns the written paths relative to projectDir. It refuses when that folder exists.
func Write(fsys fs.FS, name, projectDir, version string) ([]string, error) {
	root := path.Join("platforms", name)
	if _, err := fs.Stat(fsys, root); err != nil || name == "" {
		return nil, &Error{"no built-in platform " + name + "; available: " + strings.Join(Names(fsys), ", ")}
	}
	target := filepath.Join(projectDir, ".ai", "platforms", name)
	if _, err := os.Lstat(target); err == nil {
		return nil, &Error{filepath.Join(".ai", "platforms", name) + " exists; it belongs to the team now, so dashcrew never overwrites it"}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var written []string
	err := fs.WalkDir(fsys, root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return err
		}
		relative := strings.TrimPrefix(file, root+"/")
		text := string(data)
		if relative == "platform.yaml" {
			text = header(name, version) + text
		}
		out := filepath.Join(target, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
			return err
		}
		written = append(written, filepath.ToSlash(filepath.Join(".ai", "platforms", name, relative)))
		return nil
	})
	return written, err
}

// header heads an ejected platform.yaml: where it came from, and that it no longer follows the built-in platform.
func header(name, version string) string {
	return "# Ejected from dashcrew " + version + ": this repository's own " + name + " platform, in place of the built-in\n" +
		"# one. Edit it to match the code here; dashcrew never overwrites it, so it no longer gets the built-in updates.\n"
}
