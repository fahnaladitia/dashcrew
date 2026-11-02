package knowledge

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Pointer states that CheckPointers reports.
const (
	PointerFound     = "found"
	PointerMissing   = "missing"
	PointerUnchecked = "unchecked"
)

// PointerCheck is one code pointer and whether its path exists in its repo; RepoDir is empty when unchecked.
type PointerCheck struct {
	Pointer Pointer
	Status  string
	RepoDir string
}

// CheckPointers resolves each pointer's repo as Staleness does (the project, or a sibling folder of it) and
// looks its path up at HEAD. A repo it cannot find is unchecked. The line itself is never checked.
func CheckPointers(project string, pointers []Pointer) []PointerCheck {
	repos := newRepoResolver(project)
	out := make([]PointerCheck, 0, len(pointers))
	for _, pointer := range pointers {
		check := PointerCheck{Pointer: pointer, Status: PointerUnchecked}
		if dir, ok := repos.resolve(pointer.Repo); ok {
			check.RepoDir, check.Status = dir, PointerMissing
			if repos.has(dir, pointer.Path) {
				check.Status = PointerFound
			}
		}
		out = append(out, check)
	}
	return out
}

// PageFile resolves page, a slash path relative to the clone such as `features/share.md` (`.md` may be left
// out), to the page's slash path and its file. It refuses an absolute path, a `..` segment, the git folder,
// anything but a `.md` file, and a link whose real path leaves the clone.
func PageFile(clone, page string) (string, string, error) {
	page = strings.TrimSpace(page)
	outside := errors.New(page + " is not a page inside the knowledge clone; give a path such as features/<slug>.md")
	if page == "" || strings.Contains(page, `\`) || path.IsAbs(page) || filepath.IsAbs(page) || filepath.VolumeName(page) != "" {
		return "", "", outside
	}
	for _, segment := range strings.Split(page, "/") {
		if segment == ".." {
			return "", "", outside
		}
	}
	rel := path.Clean(page)
	if path.Ext(rel) != ".md" {
		rel += ".md"
	}
	if rel == ".git" || strings.HasPrefix(rel, ".git/") {
		return "", "", outside
	}
	file := filepath.Join(clone, filepath.FromSlash(rel))
	real, err := filepath.EvalSymlinks(file)
	if err != nil {
		return "", "", errors.New("no page " + rel + " in the knowledge clone; find one with knowledge_search")
	}
	root, err := filepath.EvalSymlinks(clone)
	if err != nil {
		return "", "", err
	}
	if inside, err := filepath.Rel(root, real); err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", "", outside
	}
	if info, err := os.Stat(real); err != nil || !info.Mode().IsRegular() {
		return "", "", errors.New("no page " + rel + " in the knowledge clone; find one with knowledge_search")
	}
	return rel, file, nil
}
