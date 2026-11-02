// Package deps checks how each dependency in `project.links` is committed (P9): a git ref must be a tag or a
// commit on the dependency's default branch, a path must stay inside the repository, a Go `replace` must not point
// at a local folder, and the link file itself must never be committed. Hosted versions are left to `pub get` and
// `go mod download`, which already fail on a version that is not published.
package deps

import (
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Status of one dependency.
type Status int

const (
	// OK: committed in a state that is merged or released.
	OK Status = iota
	// Refused: committed against work that may not be merged, or against a local folder.
	Refused
	// Unreachable: the dependency's repository could not be read; the check fails.
	Unreachable
	// Absent: the package is not a dependency of this repository.
	Absent
)

// Result is the state of one listed package, or of the link file when Package is empty.
type Result struct {
	Package string
	Status  Status
	Detail  string
}

const (
	pubspecOverrides = "pubspec_overrides.yaml"
	goWork           = "go.work"
)

// Check reads the committed manifests in dir for every link of p. linkFile is the platform's link file; it decides
// which manifest is read (`pubspec.yaml` and `pubspec.lock`, or `go.mod`).
func Check(dir string, p *profile.Profile, linkFile string) []Result {
	var results []Result
	if tracked(dir, linkFile) {
		results = append(results, Result{"", Refused, linkFile + " is committed; it points at a local checkout. Remove it from git (git rm --cached " + linkFile + ")"})
	}
	remote := &remotes{seen: map[string]*remoteState{}}
	defer remote.close()
	for _, link := range p.Links {
		var result Result
		if linkFile == goWork {
			result = goModule(dir, link, remote)
		} else {
			result = pubPackage(dir, link, remote)
		}
		result.Package = link.Package
		results = append(results, result)
	}
	return results
}

func tracked(dir, file string) bool {
	return exec.Command("git", "-C", dir, "ls-files", "--error-unmatch", "--", file).Run() == nil
}

func pubPackage(dir string, link profile.Link, remote *remotes) Result {
	manifest, err := readYAML(filepath.Join(dir, "pubspec.yaml"))
	if err != nil {
		return Result{Status: Refused, Detail: "pubspec.yaml: " + err.Error()}
	}
	var spec any
	var section string
	for _, name := range []string{"dependency_overrides", "dependencies", "dev_dependencies"} {
		if deps, ok := manifest.Value(name).(*omap.Map[any]); ok && deps.Has(link.Package) {
			spec, section = deps.Value(link.Package), name
			break
		}
	}
	if section == "" {
		return Result{Status: Absent, Detail: "not a dependency in pubspec.yaml"}
	}
	fields, isMap := spec.(*omap.Map[any])
	if !isMap || !fields.Has("path") {
		if path, linked := lockedPath(dir, link.Package); linked {
			return Result{Status: Refused, Detail: "pubspec.lock resolves it from the path " + path + ", written while it was linked; " +
				"run dashcrew link remove (it runs pub get) so the lock goes back to " + section + ", then commit the lock"}
		}
	}
	switch {
	case !isMap || fields.Has("hosted") || (!fields.Has("git") && !fields.Has("path") && !fields.Has("sdk")):
		return Result{Status: OK, Detail: "hosted version in " + section + "; pub get checks that it is published"}
	case fields.Has("path"):
		target, _ := fields.Value("path").(string)
		if leaves(target) {
			return Result{Status: Refused, Detail: "path " + target + " in " + section + " leaves the repository; use a tag or a published version, and dashcrew link for local work"}
		}
		return Result{Status: OK, Detail: "path " + target + " inside the repository"}
	case fields.Has("git"):
		url, ref := gitSpec(fields.Value("git"))
		if link.Repo != "" {
			url = link.Repo
		}
		if url == "" {
			return Result{Status: Refused, Detail: "git dependency without a url"}
		}
		locked := lockedRef(dir, link.Package)
		return remote.verify(url, ref, locked, section)
	}
	return Result{Status: OK, Detail: "sdk dependency"}
}

func gitSpec(value any) (url, ref string) {
	switch v := value.(type) {
	case string:
		return v, ""
	case *omap.Map[any]:
		url, _ = v.Value("url").(string)
		ref, _ = v.Value("ref").(string)
	}
	return url, ref
}

// lockedRef is the commit pubspec.lock resolved the package to; empty without one.
func lockedRef(dir, pkg string) string {
	_, description := lockEntry(dir, pkg)
	if description == nil {
		return ""
	}
	ref, _ := description.Value("resolved-ref").(string)
	return ref
}

// lockedPath is the path pubspec.lock resolves the package from, as `pub get` writes it under a
// `pubspec_overrides.yaml`; false when the lock has no path entry for it.
func lockedPath(dir, pkg string) (string, bool) {
	source, description := lockEntry(dir, pkg)
	if source != "path" || description == nil {
		return "", false
	}
	path, _ := description.Value("path").(string)
	return path, true
}

// lockEntry is the package's `source` and `description` in pubspec.lock; empty and nil without one.
func lockEntry(dir, pkg string) (string, *omap.Map[any]) {
	lock, err := readYAML(filepath.Join(dir, "pubspec.lock"))
	if err != nil {
		return "", nil
	}
	packages, _ := lock.Value("packages").(*omap.Map[any])
	if packages == nil {
		return "", nil
	}
	entry, _ := packages.Value(pkg).(*omap.Map[any])
	if entry == nil {
		return "", nil
	}
	source, _ := entry.Value("source").(string)
	description, _ := entry.Value("description").(*omap.Map[any])
	return source, description
}

func readYAML(file string) (*omap.Map[any], error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, errors.New("cannot be read")
	}
	value, err := yamlutil.Parse(string(data))
	if err != nil {
		return nil, errors.New("not valid YAML")
	}
	root, ok := value.(*omap.Map[any])
	if !ok {
		return nil, errors.New("not a map")
	}
	return root, nil
}

// leaves is true for a path that is absolute or climbs out of the repository.
func leaves(target string) bool {
	target = filepath.ToSlash(target)
	if target == "" || path.IsAbs(target) || filepath.IsAbs(target) {
		return true
	}
	clean := path.Clean(target)
	return clean == ".." || strings.HasPrefix(clean, "../")
}

var (
	requireLine   = regexp.MustCompile(`^(?:require\s+)?(\S+)\s+(v\S+)(?:\s*//.*)?$`)
	replaceLine   = regexp.MustCompile(`^(?:replace\s+)?(\S+)(?:\s+v\S+)?\s+=>\s+(\S+)(?:\s+(v\S+))?(?:\s*//.*)?$`)
	pseudoVersion = regexp.MustCompile(`-(?:0\.)?(?:\d+\.)?\d{14}-([0-9a-f]{12})(?:\+incompatible)?$`)
)

func goModule(dir string, link profile.Link, remote *remotes) Result {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return Result{Status: Refused, Detail: "go.mod cannot be read"}
	}
	var version, replacement, replacementVersion string
	block := ""
	for _, raw := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == ")":
			block = ""
			continue
		case strings.HasSuffix(line, "(") && (strings.HasPrefix(line, "require") || strings.HasPrefix(line, "replace")):
			block = strings.Fields(line)[0]
			continue
		}
		isReplace := block == "replace" || strings.HasPrefix(line, "replace ")
		isRequire := block == "require" || strings.HasPrefix(line, "require ")
		if isReplace {
			if m := replaceLine.FindStringSubmatch(line); m != nil && m[1] == link.Package {
				replacement, replacementVersion = m[2], m[3]
			}
		} else if isRequire {
			if m := requireLine.FindStringSubmatch(line); m != nil && m[1] == link.Package {
				version = m[2]
			}
		}
	}
	if replacement != "" {
		if replacementVersion == "" {
			return Result{Status: Refused, Detail: "replace => " + replacement + " points at a local folder; drop it, and use dashcrew link for local work"}
		}
		return goVersion(replacement, replacementVersion, link, remote, "replace => "+replacement+" ")
	}
	if version == "" {
		return Result{Status: Absent, Detail: "not required in go.mod"}
	}
	return goVersion(link.Package, version, link, remote, "")
}

func goVersion(module, version string, link profile.Link, remote *remotes, prefix string) Result {
	match := pseudoVersion.FindStringSubmatch(version)
	if match == nil {
		return Result{Status: OK, Detail: prefix + version + " is a release; go mod download checks that it exists"}
	}
	url := link.Repo
	if url == "" {
		url = "https://" + module
	}
	result := remote.verify(url, "", match[1], "go.mod")
	result.Detail = prefix + version + ": " + result.Detail
	return result
}

// remotes reads each repository once per check.
type remotes struct{ seen map[string]*remoteState }

type remoteState struct {
	err           error
	defaultBranch string
	heads, tags   map[string]bool
	fetched       string
}

var hexRef = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// verify checks ref (empty when the manifest names none) and locked, the commit the lock file pins (empty when
// none), against url.
func (r *remotes) verify(url, ref, locked, where string) Result {
	state := r.read(url)
	if state.err != nil {
		return Result{Status: Unreachable, Detail: "cannot read " + url + ": " + state.err.Error() +
			"; CI needs read access to it, the same credential pub get or go mod download uses"}
	}
	switch {
	case ref != "" && state.tags[ref]:
		return Result{Status: OK, Detail: "tag " + ref}
	case ref != "" && state.heads[ref]:
		return Result{Status: Refused, Detail: "ref " + ref + " in " + where + " is a branch; merge it, then pin a tag or a commit on " + state.defaultBranch}
	case ref != "" && !hexRef.MatchString(ref):
		return Result{Status: Refused, Detail: "ref " + ref + " in " + where + " is no tag, branch or commit of " + url}
	}
	commit := ref
	if commit == "" {
		commit = locked
	}
	if commit == "" {
		return Result{Status: Refused, Detail: "no ref and no locked commit; pin a tag or a commit on " + state.defaultBranch}
	}
	on, err := r.onDefault(url, state, commit)
	if err != nil {
		return Result{Status: Unreachable, Detail: "cannot fetch " + state.defaultBranch + " of " + url + ": " + err.Error()}
	}
	if !on {
		return Result{Status: Refused, Detail: "commit " + short(commit) + " is not on " + state.defaultBranch + " of " + url + "; merge it first"}
	}
	return Result{Status: OK, Detail: "commit " + short(commit) + " is on " + state.defaultBranch}
}

func short(sha string) string { return sha[:min(len(sha), 12)] }

func (r *remotes) read(url string) *remoteState {
	if state, ok := r.seen[url]; ok {
		return state
	}
	state := &remoteState{heads: map[string]bool{}, tags: map[string]bool{}}
	r.seen[url] = state
	out, err := gitOutput("", "ls-remote", "--symref", url)
	if err != nil {
		state.err = err
		return state
	}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "ref:" && fields[2] == "HEAD" {
			state.defaultBranch = strings.TrimPrefix(fields[1], "refs/heads/")
		} else if len(fields) == 2 {
			if name, ok := strings.CutPrefix(fields[1], "refs/heads/"); ok {
				state.heads[name] = true
			} else if name, ok := strings.CutPrefix(fields[1], "refs/tags/"); ok {
				state.tags[strings.TrimSuffix(name, "^{}")] = true
			}
		}
	}
	if state.defaultBranch == "" {
		state.err = errors.New("no default branch (HEAD) advertised")
	}
	return state
}

// onDefault fetches the default branch's commits into a temporary repository once, and tells whether commit is
// one of them.
func (r *remotes) onDefault(url string, state *remoteState, commit string) (bool, error) {
	if state.fetched == "" {
		dir, err := os.MkdirTemp("", "dashcrew-deps-")
		if err != nil {
			return false, err
		}
		if _, err := gitOutput(dir, "init", "-q", "--bare"); err != nil {
			os.RemoveAll(dir)
			return false, err
		}
		refspec := "refs/heads/" + state.defaultBranch + ":refs/remotes/dep/default"
		if _, err := gitOutput(dir, "fetch", "-q", "--filter=tree:0", url, refspec); err != nil {
			if _, err := gitOutput(dir, "fetch", "-q", url, refspec); err != nil {
				os.RemoveAll(dir)
				return false, err
			}
		}
		state.fetched = dir
	}
	cmd := exec.Command("git", "-C", state.fetched, "merge-base", "--is-ancestor", commit, "refs/remotes/dep/default")
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit):
		return false, nil
	}
	return false, err
}

// close removes the temporary repositories fetched into.
func (r *remotes) close() {
	for _, state := range r.seen {
		if state.fetched != "" {
			os.RemoveAll(state.fetched)
		}
	}
}

func gitOutput(dir string, args ...string) (string, error) {
	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if line, _, _ := strings.Cut(strings.TrimSpace(string(exit.Stderr)), "\n"); line != "" {
				return "", errors.New(line)
			}
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
