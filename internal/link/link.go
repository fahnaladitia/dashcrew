// Package link points a dependency listed in `project.links` at a local checkout, through the platform's override
// file (`pubspec_overrides.yaml` or `go.work`). The file is excluded from git and never committed; `dashcrew check`
// refuses a committed one.
package link

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/worktree"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Header is the first line of every override file dashcrew writes; a file without it is not dashcrew's.
const Header = "Written by dashcrew link; not committed. Change it with dashcrew link add or remove."

const (
	pubspecOverrides = "pubspec_overrides.yaml"
	goWork           = "go.work"
)

// Error is a refusal meant for the user.
type Error struct{ Message string }

func (e *Error) Error() string { return e.Message }

func refuse(message string) error { return &Error{message} }

// Entry is one link in the override file.
type Entry struct {
	Package string
	// Path is as written in the file: relative to the checkout root, slash-separated.
	Path string
}

// Links works on the override file of one checkout.
type Links struct {
	// Dir is the checkout root (`git rev-parse --show-toplevel`).
	Dir      string
	Profile  *profile.Profile
	Platform *catalog.PlatformSpec
}

// Open reads the profile of the checkout holding dir, with the project packs and platforms in `.ai/` applied.
func Open(dir string, c *catalog.Catalog) (*Links, error) {
	root, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, refuse(dir + " is not inside a git repository")
	}
	root = filepath.Clean(root)
	if c, err = c.WithProjectPacks(root); err != nil {
		return nil, err
	}
	text, err := os.ReadFile(filepath.Join(root, ".ai", "project.yaml"))
	if err != nil {
		return nil, refuse("No .ai/project.yaml in " + root + "; run dashcrew init first")
	}
	p, err := profile.Parse(string(text), c.KnownOptions())
	if err != nil {
		return nil, err
	}
	return &Links{Dir: root, Profile: p, Platform: c.PlatformFor(p)}, nil
}

// File is the override file name; an error when the platform declares none.
func (l *Links) File() (string, error) {
	if l.Platform.Link == nil || *l.Platform.Link == "" {
		hint := "its platform.yaml"
		if l.Profile.Platform == profile.OtherOption {
			hint = "platform_config.link"
		}
		return "", refuse("platform " + l.Platform.Name + " has no link file; set " + hint + " to " +
			strings.Join(profile.LinkFiles, " or "))
	}
	if name := *l.Platform.Link; name == pubspecOverrides || name == goWork {
		return name, nil
	}
	return "", refuse("platform " + l.Platform.Name + " links with " + *l.Platform.Link + ", which dashcrew link cannot write")
}

// Read returns the entries of the override file; nil without the file. An error when the file is not dashcrew's.
func (l *Links) Read() ([]Entry, error) {
	name, err := l.File()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(l.Dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !Managed(text) {
		return nil, refuse(name + " was not written by dashcrew link; move it away, or pass --force to replace it")
	}
	if name == goWork {
		return readGoWork(text), nil
	}
	return readPubspecOverrides(text)
}

// Managed is true when text starts with the dashcrew header, as a YAML or Go comment.
func Managed(text string) bool {
	first, _, _ := strings.Cut(text, "\n")
	return first == "# "+Header || first == "// "+Header
}

// Add points pkg at target, a folder holding that package, and returns the entries now in the file.
func (l *Links) Add(pkg, target string, force bool) ([]Entry, error) {
	name, err := l.File()
	if err != nil {
		return nil, err
	}
	if _, listed := l.Profile.LinkOf(pkg); !listed {
		return nil, refuse(pkg + " is not in project.links of .ai/project.yaml; add { package: " + pkg + " } there first")
	}
	if err := l.refuseTracked(name); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, refuse(target + " is not a folder")
	}
	if found, err := packageAt(abs, name); err != nil {
		return nil, err
	} else if found != pkg {
		return nil, refuse(target + " holds " + strconv.Quote(found) + ", not " + strconv.Quote(pkg))
	}
	if same(abs, l.Dir) {
		return nil, refuse(target + " is this checkout")
	}
	entries, err := l.Read()
	if err != nil {
		var refusal *Error
		if !force || !errors.As(err, &refusal) {
			return nil, err
		}
		entries = nil
	}
	relative, err := filepath.Rel(realPath(l.Dir), realPath(abs))
	if err != nil {
		relative = abs
	}
	entries = put(entries, Entry{pkg, filepath.ToSlash(relative)})
	if err := l.write(name, entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// Remove drops pkg from the file, and the file when nothing is left; the entries left are returned.
func (l *Links) Remove(pkg string) ([]Entry, error) {
	name, err := l.File()
	if err != nil {
		return nil, err
	}
	entries, err := l.Read()
	if err != nil {
		return nil, err
	}
	kept := entries[:0]
	for _, entry := range entries {
		if entry.Package != pkg {
			kept = append(kept, entry)
		}
	}
	if len(kept) == len(entries) {
		return nil, refuse(pkg + " is not linked")
	}
	if len(kept) == 0 {
		return nil, os.Remove(filepath.Join(l.Dir, name))
	}
	return kept, l.write(name, kept)
}

// Deps runs the platform's deps command in the checkout; see worktree.RunDeps.
func (l *Links) Deps() string { return worktree.RunDeps(l.Dir, l.Platform) }

func (l *Links) refuseTracked(name string) error {
	if _, err := git(l.Dir, "ls-files", "--error-unmatch", "--", name); err == nil {
		return refuse(name + " is committed in this repository; remove it from git first (git rm --cached " + name + ")")
	}
	return nil
}

func (l *Links) write(name string, entries []Entry) error {
	var text string
	if name == goWork {
		version, err := goVersion(l.Dir)
		if err != nil {
			return err
		}
		text = writeGoWork(version, entries)
	} else {
		text = writePubspecOverrides(entries)
	}
	excluded := []string{name}
	if name == goWork {
		excluded = append(excluded, "go.work.sum")
	}
	if err := worktree.Exclude(l.Dir, excluded); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.Dir, name), []byte(text), 0o644)
}

func put(entries []Entry, entry Entry) []Entry {
	for i := range entries {
		if entries[i].Package == entry.Package {
			entries[i] = entry
			return entries
		}
	}
	entries = append(entries, entry)
	sort.Slice(entries, func(i, j int) bool { return entries[i].Package < entries[j].Package })
	return entries
}

func writePubspecOverrides(entries []Entry) string {
	var b strings.Builder
	b.WriteString("# " + Header + "\ndependency_overrides:\n")
	for _, entry := range entries {
		b.WriteString("  " + entry.Package + ":\n    path: " + strconv.Quote(entry.Path) + "\n")
	}
	return b.String()
}

func readPubspecOverrides(text string) ([]Entry, error) {
	value, err := yamlutil.Parse(text)
	if err != nil {
		return nil, refuse(pubspecOverrides + " is not valid YAML: " + err.Error())
	}
	root, _ := value.(*omap.Map[any])
	if root == nil {
		return nil, nil
	}
	overrides, _ := root.Value("dependency_overrides").(*omap.Map[any])
	if overrides == nil {
		return nil, nil
	}
	var entries []Entry
	for _, name := range overrides.Keys() {
		spec, _ := overrides.Value(name).(*omap.Map[any])
		if spec == nil {
			continue
		}
		if path, ok := spec.Value("path").(string); ok {
			entries = append(entries, Entry{name, path})
		}
	}
	return entries, nil
}

var useLine = regexp.MustCompile(`^use (\S+) // (\S+)$`)

func writeGoWork(version string, entries []Entry) string {
	var b strings.Builder
	b.WriteString("// " + Header + "\ngo " + version + "\n\nuse .\n")
	for _, entry := range entries {
		b.WriteString("use " + entry.Path + " // " + entry.Package + "\n")
	}
	return b.String()
}

func readGoWork(text string) []Entry {
	var entries []Entry
	for _, line := range strings.Split(text, "\n") {
		if match := useLine.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
			entries = append(entries, Entry{match[2], match[1]})
		}
	}
	return entries
}

var (
	goLine     = regexp.MustCompile(`(?m)^go\s+(\S+)\s*$`)
	moduleLine = regexp.MustCompile(`(?m)^module\s+(\S+)\s*$`)
)

func goVersion(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", refuse("no go.mod in " + dir + "; go.work needs this checkout's Go version")
	}
	match := goLine.FindSubmatch(data)
	if match == nil {
		return "", refuse("go.mod in " + dir + " has no go line")
	}
	return string(match[1]), nil
}

// packageAt is the name of the package in dir: the pubspec name for pubspec_overrides.yaml, the module path for
// go.work.
func packageAt(dir, file string) (string, error) {
	if file == goWork {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			return "", refuse(dir + " has no go.mod")
		}
		match := moduleLine.FindSubmatch(data)
		if match == nil {
			return "", refuse(filepath.Join(dir, "go.mod") + " has no module line")
		}
		return strings.Trim(string(match[1]), `"`), nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "pubspec.yaml"))
	if err != nil {
		return "", refuse(dir + " has no pubspec.yaml")
	}
	value, err := yamlutil.Parse(string(data))
	if err != nil {
		return "", refuse(filepath.Join(dir, "pubspec.yaml") + " is not valid YAML")
	}
	if root, ok := value.(*omap.Map[any]); ok {
		if name, ok := root.Value("name").(string); ok && name != "" {
			return name, nil
		}
	}
	return "", refuse(filepath.Join(dir, "pubspec.yaml") + " has no name")
}

// realPath is path with its symbolic links resolved, or path itself when that fails.
func realPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func same(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// Branch is the current branch of the checkout at dir; empty when detached or not a checkout.
func Branch(dir string) string {
	out, err := git(dir, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return out
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
