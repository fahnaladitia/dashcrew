package profile

import (
	"regexp"
	"strconv"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// Link is one `project.links` entry: a dependency that may be pointed at a local checkout while working.
type Link struct {
	// Package is the dependency's name in the manifest: the pubspec name, or the Go module path.
	Package string
	// Repo is the dependency's git URL, ssh or https; empty when not set. `check` reads refs from it.
	Repo string
}

// LinkFiles are the override files `link` writes: the values a platform's `link` field may take.
var LinkFiles = []string{"pubspec_overrides.yaml", "go.work"}

var (
	linkKeys    = []string{"package", "repo"}
	linkPackage = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_./~-]*$`)
	linkRepo    = regexp.MustCompile(`^(https://[^\s@]+|ssh://[^\s]+|[A-Za-z0-9_.-]+@[A-Za-z0-9_.-]+:[^\s]+)$`)
)

// parseLinks reads and removes `project.links`; nil without it. Kept out of Project like Commit; templates see only
// `has_links`.
func parseLinks(project *omap.Map[any]) ([]Link, error) {
	value := project.Value("links")
	project.Delete("links")
	if value == nil {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok || len(list) == 0 {
		return nil, fail("project.links", "expected a non-empty list of { package, repo }")
	}
	links := make([]Link, 0, len(list))
	seen := map[string]bool{}
	for i, item := range list {
		path := "project.links[" + strconv.Itoa(i) + "]"
		entry, ok := item.(*omap.Map[any])
		if !ok {
			return nil, fail(path, "expected a map with package and an optional repo")
		}
		for _, key := range entry.Keys() {
			if !contains(linkKeys, key) {
				return nil, fail(path+"."+key, "unknown key")
			}
		}
		name, ok := entry.Value("package").(string)
		if !ok || !linkPackage.MatchString(name) {
			return nil, fail(path+".package", "required, the dependency's name in pubspec.yaml or its Go module path")
		}
		if seen[name] {
			return nil, fail(path+".package", strconv.Quote(name)+" is listed twice")
		}
		seen[name] = true
		link := Link{Package: name}
		if entry.Has("repo") {
			repo, ok := entry.Value("repo").(string)
			if !ok || !linkRepo.MatchString(repo) {
				return nil, fail(path+".repo", "expected a git URL, https://... or git@host:owner/repo.git")
			}
			link.Repo = repo
		}
		links = append(links, link)
	}
	return links, nil
}

// LinkOf is the entry for package; false when it is not listed.
func (p *Profile) LinkOf(name string) (Link, bool) {
	for _, link := range p.Links {
		if link.Package == name {
			return link, true
		}
	}
	return Link{}, false
}
