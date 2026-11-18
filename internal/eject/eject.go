// Package starter writes a platform starter (P10) into a project's `.ai/platforms/<name>/`. The team owns the folder
// afterwards: the starter is written once and never updated by dashcrew.
package starter

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Error is a refusal meant for the user.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

// Names are the starters in fsys (`starters/<name>/`), sorted.
func Names(fsys fs.FS) []string {
	entries, _ := fs.ReadDir(fsys, "starters")
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

// Options are the pack options the starter ships for family, sorted; nil when it has none.
func Options(fsys fs.FS, name, family string) []string {
	entries, _ := fs.ReadDir(fsys, path.Join("starters", name, "packs", family))
	var options []string
	for _, entry := range entries {
		if entry.IsDir() {
			options = append(options, entry.Name())
		}
	}
	sort.Strings(options)
	return options
}

// Write copies starter name into projectDir/.ai/platforms/<name>/, with `{{version}}` replaced by version, and
// returns the written paths relative to projectDir. It refuses when that folder exists.
func Write(fsys fs.FS, name, projectDir, version string) ([]string, error) {
	root := path.Join("starters", name)
	if _, err := fs.Stat(fsys, root); err != nil {
		return nil, &Error{"no starter " + name + "; available: " + strings.Join(Names(fsys), ", ")}
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
		text := strings.ReplaceAll(string(data), "{{version}}", version)
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

var requires = regexp.MustCompile(`(?m)^\s*(?:require\s+)?([A-Za-z0-9.\-_/]+)\s+v[0-9]`)

// GoDefaults are the http and db packs the go.mod in projectDir points at; "nethttp" and "none" when it names none
// of the libraries the starter knows, and when there is no go.mod.
func GoDefaults(projectDir string) (http, db string) {
	http, db = "nethttp", "none"
	data, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		return http, db
	}
	modules := map[string]bool{}
	for _, match := range requires.FindAllStringSubmatch(string(data), -1) {
		modules[match[1]] = true
	}
	has := func(prefix string) bool {
		for module := range modules {
			if module == prefix || strings.HasPrefix(module, prefix+"/") {
				return true
			}
		}
		return false
	}
	switch {
	case has("github.com/go-chi/chi"):
		http = "chi"
	case has("github.com/gin-gonic/gin"):
		http = "gin"
	case has("github.com/labstack/echo"):
		http = "echo"
	case has("github.com/gofiber/fiber"):
		http = "fiber"
	}
	switch {
	case fileExists(filepath.Join(projectDir, "sqlc.yaml")) || fileExists(filepath.Join(projectDir, "sqlc.yml")) || fileExists(filepath.Join(projectDir, "sqlc.json")):
		db = "sqlc"
	case has("gorm.io/gorm"):
		db = "gorm"
	case has("github.com/jackc/pgx"):
		db = "pgx"
	}
	return http, db
}

func fileExists(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}
