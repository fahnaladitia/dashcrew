// Package worktree keeps one git worktree per parallel task, in `<main checkout>.worktrees/<name>`, with the same
// git commands, output lines and messages as the Dart engine.
//
// The only writing git commands are `worktree add --no-track -b`, `branch --unset-upstream`,
// `config branch.<b>.dashcrew-base` and `worktree remove`; nothing commits, pushes or deletes a branch.
package worktree

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Error is a refused worktree command; Message says why and what the user can do.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

// Info is one task worktree as `list` reports it; Upstream and Base are nil when unset.
type Info struct {
	Name         string
	Path         string
	Branch       string
	Base         *string
	Upstream     *string
	Dirty        int
	MissingLinks []string
}

// Worktrees are the task worktrees of one repository.
type Worktrees struct {
	// GitDir is the repository's shared git directory, also when Worktrees is created from inside a task worktree.
	GitDir string
	// Catalog resolves the platform's deps command; nil means the embedded catalog, loaded on first use.
	Catalog *catalog.Catalog
}

var toolkitEntries = []string{"AGENTS.md", "CLAUDE.md", ".claude", ".mcp.json", ".codex", ".agents", ".ai"}

// New finds the repository of projectDir; c may be nil.
func New(projectDir string, c *catalog.Catalog) (*Worktrees, error) {
	result := git(projectDir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if result.code != 0 {
		return nil, &Error{projectDir + " is not inside a git repository."}
	}
	return &Worktrees{GitDir: strings.TrimSpace(result.stdout), Catalog: c}, nil
}

// MainDir is the main checkout that holds GitDir.
func (w *Worktrees) MainDir() string { return filepath.Dir(w.GitDir) }

// Dir is the folder that holds the task worktrees.
func (w *Worktrees) Dir() string { return w.MainDir() + ".worktrees" }

type result struct {
	code           int
	stdout, stderr string
}

// run runs a command in dir; a command that cannot start reports exit code -1 and the error on stderr.
func run(dir, name string, args ...string) result {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		return result{exit.ExitCode(), stdout.String(), stderr.String()}
	case err != nil:
		return result{-1, stdout.String(), err.Error()}
	}
	return result{0, stdout.String(), stderr.String()}
}

func git(dir string, args ...string) result { return run(dir, "git", args...) }

func gitOrFail(dir string, args ...string) (string, error) {
	r := git(dir, args...)
	if r.code != 0 {
		return "", &Error{"git " + strings.Join(args, " ") + " failed: " + strings.TrimSpace(r.stderr)}
	}
	return r.stdout, nil
}

// join is Dart's `p.join` for two parts: no normalization, and an absolute second part wins.
func join(base, part string) string {
	if filepath.IsAbs(part) {
		return part
	}
	if strings.HasSuffix(base, string(filepath.Separator)) {
		return base + part
	}
	return base + string(filepath.Separator) + part
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	return line
}

func lineCount(trimmed string) int {
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func lexists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// Create makes `<branch>` from base in a new worktree; base must exist and is recorded without `origin/`.
// With installDeps it runs the deps command of the main checkout's platform (`platform.commands.deps`).
func (w *Worktrees) Create(branch, base string, name *string, installDeps bool, out func(string)) (*Info, error) {
	folder := strings.ReplaceAll(branch, "/", "-")
	if name != nil {
		folder = *name
	}
	target := join(w.Dir(), folder)
	main := w.MainDir()
	if git(main, "check-ref-format", "--branch", branch).code != 0 {
		return nil, &Error{`"` + branch + `" is not a valid branch name.`}
	}
	if git(main, "rev-parse", "--verify", "--quiet", base+"^{commit}").code != 0 {
		return nil, &Error{"Base " + base + " not found. Fetch it or name an existing branch."}
	}
	if git(main, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).code == 0 {
		return nil, &Error{"Branch " + branch + " already exists."}
	}
	if lexists(target) {
		return nil, &Error{"Folder " + target + " already exists."}
	}

	if _, err := gitOrFail(main, "worktree", "add", "--quiet", "--no-track", "-b", branch, target, base); err != nil {
		return nil, err
	}
	if err := dropUpstream(target, out); err != nil {
		return nil, err
	}
	recordedBase := strings.TrimPrefix(base, "origin/")
	if _, err := gitOrFail(main, "config", "branch."+branch+".dashcrew-base", recordedBase); err != nil {
		return nil, err
	}
	linked, err := w.link(target, false)
	if err != nil {
		return nil, err
	}

	out("path: " + target)
	out("branch: " + branch + " (base " + recordedBase + ")")
	out("upstream: none")
	out("status: " + firstLine(git(target, "status", "-sb").stdout))
	if len(linked) == 0 {
		out("links: none")
	} else {
		out("links: " + strings.Join(linked, ", "))
	}
	if installDeps {
		out("deps: " + w.installDeps(target))
	} else {
		out("deps: skipped")
	}
	infos, err := w.List()
	if err != nil {
		return nil, err
	}
	for i := range infos {
		if infos[i].Path == target {
			return &infos[i], nil
		}
	}
	return nil, nil
}

// Link adds missing toolkit links to worktree name and drops an upstream it should not have; it returns the
// added links.
func (w *Worktrees) Link(name string, out func(string)) ([]string, error) {
	target, err := w.existing(name)
	if err != nil {
		return nil, err
	}
	if err := dropUpstream(target, out); err != nil {
		return nil, err
	}
	linked, err := w.link(target, false)
	if err != nil {
		return nil, err
	}
	if len(linked) == 0 {
		out("links: up to date")
	} else {
		out("linked: " + strings.Join(linked, ", "))
	}
	return linked, nil
}

// List is every task worktree, the main checkout excluded.
func (w *Worktrees) List() ([]Info, error) {
	porcelain, err := gitOrFail(w.MainDir(), "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	infos := []Info{}
	for _, block := range strings.Split(porcelain, "\n\n") {
		var worktreePath, branchLine string
		for _, line := range strings.Split(block, "\n") {
			if worktreePath == "" && strings.HasPrefix(line, "worktree ") {
				worktreePath = strings.Replace(line, "worktree ", "", 1)
			}
			if branchLine == "" && strings.HasPrefix(line, "branch ") {
				branchLine = line
			}
		}
		if worktreePath == "" || worktreePath == w.MainDir() || branchLine == "" || !isDir(worktreePath) {
			continue
		}
		branch := strings.Replace(branchLine, "branch refs/heads/", "", 1)
		info := Info{Name: filepath.Base(worktreePath), Path: worktreePath, Branch: branch, Base: w.BaseOf(branch)}
		if upstream := git(worktreePath, "rev-parse", "--abbrev-ref", "@{u}"); upstream.code == 0 {
			value := strings.TrimSpace(upstream.stdout)
			info.Upstream = &value
		}
		info.Dirty = lineCount(strings.TrimSpace(git(worktreePath, "status", "--porcelain").stdout))
		if info.MissingLinks, err = w.link(worktreePath, true); err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// Remove removes the clean worktree name; the branch and the linked toolkit in the main checkout stay.
func (w *Worktrees) Remove(name string, out func(string)) error {
	target, err := w.existing(name)
	if err != nil {
		return err
	}
	status := strings.TrimSpace(git(target, "status", "--porcelain").stdout)
	if status != "" {
		return &Error{name + " has uncommitted changes (" + strconv.Itoa(lineCount(status)) +
			" lines). Commit or discard them first."}
	}
	if _, err := gitOrFail(w.MainDir(), "worktree", "remove", target); err != nil {
		return err
	}
	out("removed: " + target + " (branch kept)")
	return nil
}

// BaseOf is the base Create recorded, without `origin/`; nil when none.
func (w *Worktrees) BaseOf(branch string) *string {
	r := git(w.MainDir(), "config", "--get", "branch."+branch+".dashcrew-base")
	if r.code != 0 {
		return nil
	}
	value := strings.TrimSpace(r.stdout)
	return &value
}

func (w *Worktrees) existing(name string) (string, error) {
	target := join(w.Dir(), name)
	infos, err := w.List()
	if err != nil {
		return "", err
	}
	for _, info := range infos {
		if info.Path == target {
			return target, nil
		}
	}
	return "", &Error{"No worktree named " + name + " in " + w.Dir() + "."}
}

func dropUpstream(dir string, out func(string)) error {
	if git(dir, "rev-parse", "--abbrev-ref", "@{u}").code != 0 {
		return nil
	}
	if _, err := gitOrFail(dir, "branch", "--unset-upstream"); err != nil {
		return err
	}
	out("upstream removed: a new branch tracks nothing until its first push")
	return nil
}

// isWithin is Dart's `p.isWithin` for normalized relative posix paths.
func isWithin(parent, child string) bool {
	if parent == child {
		return false
	}
	if parent == "." {
		return child != ".." && !strings.HasPrefix(child, "../")
	}
	return strings.HasPrefix(child, parent+"/")
}

// link adds the toolkit entries the main checkout ignores and the worktree lacks, recursing into other folders.
// An entry the main checkout does not ignore is never linked: the shared exclude file would hide it there too.
func (w *Worktrees) link(worktree string, dryRun bool) ([]string, error) {
	main := w.MainDir()
	linked := []string{}
	var visit func(relative string) error
	visit = func(relative string) error {
		source := join(main, filepath.FromSlash(relative))
		if !lexists(source) {
			return nil
		}
		target := join(worktree, filepath.FromSlash(relative))
		targetInfo, targetErr := os.Lstat(target)
		missing := targetErr != nil
		if missing && git(main, "check-ignore", "-q", relative).code == 0 {
			if !dryRun {
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				if err := os.Symlink(source, target); err != nil {
					return err
				}
			}
			linked = append(linked, relative)
		} else if (missing || targetInfo.IsDir()) && isDir(source) {
			entries, err := os.ReadDir(source)
			if err != nil {
				return err
			}
			children := []string{}
			for _, entry := range entries {
				children = append(children, entry.Name())
			}
			sort.Strings(children)
			for _, child := range children {
				if err := visit(path.Join(relative, child)); err != nil {
					return err
				}
			}
		}
		return nil
	}

	entries := []string{}
	for _, entry := range append(append([]string{}, toolkitEntries...), w.profilePaths()...) {
		duplicate := false
		for _, seen := range entries {
			duplicate = duplicate || seen == entry
		}
		if !duplicate {
			entries = append(entries, entry)
		}
	}
	for _, entry := range entries {
		nested := false
		for _, other := range entries {
			nested = nested || (other != entry && isWithin(other, entry))
		}
		if !nested {
			if err := visit(entry); err != nil {
				return nil, err
			}
		}
	}
	if !dryRun && len(linked) > 0 {
		if err := exclude(worktree, linked); err != nil {
			return nil, err
		}
	}
	return linked, nil
}

// profilePaths are `paths.plans`, `paths.pr` and `paths.memory` from the main checkout's profile, with defaults.
func (w *Worktrees) profilePaths() []string {
	defaults := [][2]string{{"plans", ".ai/plans"}, {"pr", ".ai/pr"}, {"memory", ".ai/memory"}}
	var paths *omap.Map[any]
	if text, err := os.ReadFile(filepath.Join(w.MainDir(), ".ai", "project.yaml")); err == nil {
		if parsed, err := yamlutil.Parse(string(text)); err == nil {
			if root, ok := parsed.(*omap.Map[any]); ok {
				paths, _ = root.Value("paths").(*omap.Map[any])
			}
		}
	}
	out := []string{}
	for _, entry := range defaults {
		value := entry[1]
		if paths != nil {
			if configured, ok := paths.Value(entry[0]).(string); ok {
				value = configured
			}
		}
		out = append(out, path.Clean(value))
	}
	return out
}

func exclude(worktree string, linked []string) error {
	excludePath := strings.TrimSpace(git(worktree, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude").stdout)
	data, _ := os.ReadFile(excludePath)
	current := string(data)
	lines := map[string]bool{}
	for _, line := range strings.Split(current, "\n") {
		lines[line] = true
	}
	missing := []string{}
	for _, entry := range linked {
		if !lines["/"+entry] {
			missing = append(missing, "/"+entry)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	prefix := current
	if current != "" && !strings.HasSuffix(current, "\n") {
		prefix += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(excludePath, []byte(prefix+strings.Join(missing, "\n")+"\n"), 0o644)
}

// installDeps runs the deps command in dir when one of the platform's detect files is there.
// A Flutter command runs through `fvm` when the worktree has `.fvmrc`.
func (w *Worktrees) installDeps(dir string) string {
	platform, err := w.platform()
	if err != nil {
		return "skipped (" + filepath.Join(".ai", "project.yaml") + ": " + err.Error() + ")"
	}
	command, ok := platform.Commands.Value("deps").(string)
	if !ok {
		return "skipped (platform " + platform.Name + " has no deps command)"
	}
	found := false
	for _, file := range platform.Detect {
		found = found || isFile(join(dir, file))
	}
	if !found {
		return "skipped (no " + strings.Join(platform.Detect, " or ") + ")"
	}
	shell := command
	if strings.HasPrefix(command, "flutter ") && isFile(filepath.Join(dir, ".fvmrc")) {
		shell = "fvm " + command
	}
	var r result
	if runtime.GOOS == "windows" {
		r = run(dir, "cmd", "/c", shell)
	} else {
		r = run(dir, "/bin/sh", "-c", shell)
	}
	if r.code == 0 {
		return "ok (" + shell + ")"
	}
	return "failed (exit " + strconv.Itoa(r.code) + "); rerun: cd " + dir + " && " + shell
}

// platform is the platform of the main checkout's profile; no profile reads as the default platform.
func (w *Worktrees) platform() (*catalog.PlatformSpec, error) {
	c := w.Catalog
	if c == nil {
		var err error
		if c, err = catalog.LoadEmbedded(); err != nil {
			return nil, err
		}
	}
	c, err := c.WithProjectPacks(w.MainDir())
	if err != nil {
		return nil, err
	}
	text, err := os.ReadFile(filepath.Join(w.MainDir(), ".ai", "project.yaml"))
	if err != nil {
		if spec := c.Platforms[profile.DefaultPlatform]; spec != nil {
			return spec, nil
		}
		return &catalog.PlatformSpec{Name: profile.DefaultPlatform, Detect: []string{}, Commands: omap.New[any]()}, nil
	}
	parsed, err := profile.Parse(string(text), c.KnownOptions())
	if err != nil {
		return nil, err
	}
	return c.PlatformFor(parsed), nil
}
