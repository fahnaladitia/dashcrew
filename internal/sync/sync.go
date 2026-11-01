// Package sync renders `.ai/project.yaml` into the agent files of one project.
//
// Generated files are tracked in `.ai/lock.json`; a file edited by hand stops the run unless forced. The guard
// policy and shim are written as the Dart engine writes them. Installing the Dart guard binary is not ported: the
// shim switches to `dashcrew guard` in stage G7.
package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/projectfiles"
	"github.com/fahnaladitia/dashcrew/internal/render"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// Error is a refused sync; Message names the files or settings involved.
type Error struct {
	Message string
}

func (e *Error) Error() string { return e.Message }

const (
	// PrTemplatePath holds the required PR sections; `dashcrew check` reads the same headings.
	PrTemplatePath = ".github/pull_request_template.md"
	// GithubWorkflowPath is written when `ci.github` is on; its `check` job is meant to be a required status check.
	GithubWorkflowPath = ".github/workflows/dashcrew.yml"
	// GuardPolicyPath is the compiled guard policy.
	GuardPolicyPath = ".ai/guard.json"
	// LockPath is the lock file.
	LockPath = ".ai/lock.json"

	GeneratedMarker = "<!-- dashcrew:generated - edit .ai/project.yaml or the project block, then run dashcrew sync -->"
	ProjectBegin    = "<!-- dashcrew:project:begin -->"
	ProjectEnd      = "<!-- dashcrew:project:end -->"

	// P4MigrationNote is printed once by the sync that drops the pre-P4 role files; the lock records no version
	// that tells P3 from P4.
	P4MigrationNote = "Migrated to P4: the roles were removed (only reviewer and explorer remain), " +
		"the plan gate and dashcrew gate were removed, and orchestrate team is now the default chain with skills " +
		"picked at step 0. Plans, handoffs and project memory are untouched. See \"Migrating to P4\" in the dashcrew README."

	guardIgnoreLines = "# dashcrew: compiled guard, per machine\n.ai/bin/\n"
)

// preP4Roles are the subagents dashcrew generated before P4; one in the lock but no longer rendered marks a
// pre-P4 project.
var preP4Roles = map[string]bool{
	"dart-tdd-guide": true, "design-refiner": true, "flutter-architect": true, "flutter-build-error-resolver": true,
	"flutter-code-reviewer": true, "flutter-engineer": true, "flutter-planner": true, "flutter-security-reviewer": true,
	"local-storage-engineer": true, "product-manager": true, "qa-engineer": true, "tech-lead": true, "worktree-monitor": true,
}

// Sync renders one project against a catalog.
type Sync struct {
	Catalog    *catalog.Catalog
	ProjectDir string
}

// Rendered is what Render produces: generated files in the order the Dart engine writes them,
// `family/option -> builtin|project` per rendered pack sorted by family, and the adapters whose Merge still runs.
type Rendered struct {
	Files    *omap.Map[string]
	Packs    *omap.Map[any]
	Adapters []adapters.Adapter
}

// Result is what one Run changed. Notes are one-time information, such as P4MigrationNote; never a problem to fix.
type Result struct {
	Written  []string
	Removed  []string
	Warnings []string
	Notes    []string
}

func (s Sync) project() projectfiles.Project { return projectfiles.New(s.ProjectDir) }

// Run writes the rendered files that changed, removes locked files no longer generated, merges every adapter's
// config files and rewrites the lock. force overwrites files edited by hand and a lock from a newer dashcrew;
// adopt keeps a foreign AGENTS.md as the project block.
func (s Sync) Run(force, adopt bool) (*Result, error) {
	result := &Result{Written: []string{}, Removed: []string{}, Warnings: []string{}, Notes: []string{}}
	rendered, err := s.Render(adopt, &result.Warnings)
	if err != nil {
		return nil, err
	}
	files := rendered.Files
	lock, err := s.readLock()
	if err != nil {
		return nil, err
	}
	if lockVersion, ok := lock.Value("dashcrew_version").(string); ok && version.Compare(lockVersion, version.Version) > 0 && !force {
		return nil, &Error{Message: LockPath + " was written by dashcrew " + lockVersion + ", and this is " + version.Version + ". " +
			"Update dashcrew, or rerun with --force to rewrite it with " + version.Version + "."}
	}
	lockedFiles := lockFiles(lock)

	project := s.project()
	if err := s.removeCompiledGuard(); err != nil {
		return nil, err
	}
	var conflicts []string
	for _, path := range union(files.Keys(), lockedFiles.Keys()) {
		if link := project.LinkWithin(path); link != nil {
			conflicts = append(conflicts, path+" ("+link.Path+" is a link to "+link.Target+"; --force replaces the link with a real file "+
				"and leaves its target alone)")
			continue
		}
		if !project.Exists(path) {
			continue
		}
		content, err := project.Read(path)
		if err != nil {
			return nil, err
		}
		locked, isLocked := lockedFiles.Get(path)
		adoptedAgentsMd := path == "AGENTS.md" && !isLocked
		generated, isGenerated := files.Get(path)
		if !adoptedAgentsMd && (!isLocked || locked != FileHash(path, content)) && (isLocked || !isGenerated || generated != content) {
			conflicts = append(conflicts, path)
		}
	}
	if len(conflicts) > 0 && !force {
		return nil, &Error{Message: "Files changed outside dashcrew (rerun with --force to overwrite):\n  " + strings.Join(conflicts, "\n  ")}
	}

	for _, path := range files.Keys() {
		wrote, err := project.WriteIfChanged(path, files.Value(path))
		if err != nil {
			return nil, err
		}
		if wrote {
			result.Written = append(result.Written, path)
		}
	}

	for _, path := range lockedFiles.Keys() {
		if !files.Has(path) && isPreP4Role(path) {
			result.Notes = append(result.Notes, P4MigrationNote)
			break
		}
	}

	for _, path := range lockedFiles.Keys() {
		if files.Has(path) || project.LinkWithin(path) != nil || !project.Exists(path) {
			continue
		}
		if err := os.Remove(project.Path(path)); err != nil {
			return nil, err
		}
		result.Removed = append(result.Removed, path)
	}

	ownedMcp := ownedMcp(lock)
	nowOwnedMcp := map[string][]string{}
	for _, adapter := range rendered.Adapters {
		merged, err := adapter.Merge(ownedMcp[adapter.Agent()], &result.Warnings)
		if err != nil {
			return nil, err
		}
		result.Written = append(result.Written, merged.Written...)
		owned := []string{}
		for name := range merged.OwnedMcp {
			owned = append(owned, name)
		}
		sort.Strings(owned)
		nowOwnedMcp[adapter.Agent()] = owned
	}
	if err := makeExecutable(project.Path(guard.ShimPath)); err != nil {
		return nil, err
	}
	dropped, err := s.dropGuardIgnore()
	if err != nil {
		return nil, err
	}
	if dropped {
		result.Written = append(result.Written, ".gitignore")
	}

	guardHash, err := guard.SourceHash()
	if err != nil {
		return nil, err
	}
	lockText, err := lockJSON(files, rendered.Packs, nowOwnedMcp, guardHash)
	if err != nil {
		return nil, err
	}
	if _, err := project.WriteIfChanged(LockPath, lockText); err != nil {
		return nil, err
	}
	return result, nil
}

// lockJSON is `.ai/lock.json` with the Dart key order: files sorted by path, agents of mcp_servers sorted.
func lockJSON(files *omap.Map[string], packs *omap.Map[any], ownedMcp map[string][]string, guardHash string) (string, error) {
	paths := files.Keys()
	sort.Strings(paths)
	fileHashes := omap.New[any]()
	for _, path := range paths {
		fileHashes.Set(path, FileHash(path, files.Value(path)))
	}
	agents := make([]string, 0, len(ownedMcp))
	for agent := range ownedMcp {
		agents = append(agents, agent)
	}
	sort.Strings(agents)
	servers := omap.New[any]()
	for _, agent := range agents {
		names := make([]any, len(ownedMcp[agent]))
		for i, name := range ownedMcp[agent] {
			names[i] = name
		}
		servers.Set(agent, names)
	}
	lock := omap.Of[any](
		"schema", 1,
		"dashcrew_version", version.Version,
		"files", fileHashes,
		"packs", packs,
		"mcp_servers", servers,
		"guard_source", guardHash,
	)
	text, err := dartjson.EncodeIndent(lock, "  ")
	if err != nil {
		return "", err
	}
	return text + "\n", nil
}

// union is a, then the items of b not in a, each in its own order.
func union(a, b []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, list := range [][]string{a, b} {
		for _, item := range list {
			if !seen[item] {
				seen[item] = true
				out = append(out, item)
			}
		}
	}
	return out
}

// isPreP4Role is `.claude/agents/<role>.md`, `.codex/agents/<role>.toml` or `.agents/agents/<role>.md` of a
// pre-P4 role.
func isPreP4Role(file string) bool {
	parts := strings.Split(file, "/")
	if len(parts) != 3 || parts[1] != "agents" {
		return false
	}
	base := path.Base(file)
	return preP4Roles[strings.TrimSuffix(base, path.Ext(base))]
}

// makeExecutable adds the execute bits, as `chmod +x` does under the usual umask.
func makeExecutable(file string) error {
	info, err := os.Stat(file)
	if err != nil {
		return err
	}
	return os.Chmod(file, info.Mode().Perm()|0o111)
}

// Render returns the generated files without touching the disk. Project packs and platforms in `.ai/` are
// overlaid on the catalog. adopt keeps a foreign AGENTS.md as the project block; warnings may be nil.
func (s Sync) Render(adopt bool, warnings *[]string) (*Rendered, error) {
	c, err := s.Catalog.WithProjectPacks(s.ProjectDir)
	if err != nil {
		var catalogErr *catalog.Error
		if errors.As(err, &catalogErr) {
			return nil, &Error{Message: "Project pack: " + err.Error()}
		}
		return nil, err
	}
	p, err := s.profile(c)
	if err != nil {
		return nil, err
	}
	if warnings != nil {
		*warnings = append(*warnings, p.Warnings...)
	}
	var agentAdapters []adapters.Adapter
	for _, agent := range p.Agents {
		adapter, err := adapters.AdapterFor(agent, c, s.ProjectDir, p.Platform)
		if err != nil {
			return nil, &Error{Message: err.Error()}
		}
		agentAdapters = append(agentAdapters, adapter)
	}

	renderer := render.New(c)
	targets, err := renderer.Render(p)
	if err != nil {
		return nil, err
	}
	selected := renderer.SelectedPacks(p)
	sort.SliceStable(selected, func(i, j int) bool { return selected[i].Family < selected[j].Family })
	packs := omap.New[any]()
	for _, pack := range selected {
		origin := "builtin"
		if pack.FromProject {
			origin = "project"
		}
		packs.Set(pack.Family+"/"+pack.Option, origin)
	}
	context := renderer.ContextFor(p)
	lock, err := s.readLock()
	if err != nil {
		return nil, err
	}
	lockedFiles := map[string]string{}
	locked := lockFiles(lock)
	for _, path := range locked.Keys() {
		lockedFiles[path] = locked.Value(path)
	}

	agentsMd, err := s.agentsMd(targets, adopt)
	if err != nil {
		return nil, err
	}
	policy, err := dartjson.EncodeIndent(guard.CompilePolicy(p, c.ResolvedFor(p.Platform)).ToJSON(), "  ")
	if err != nil {
		return nil, err
	}
	guardHash, err := guard.SourceHash()
	if err != nil {
		return nil, err
	}
	files := omap.New[string]()
	files.Set("AGENTS.md", agentsMd)
	files.Set(GuardPolicyPath, policy+"\n")
	files.Set(guard.ShimPath, guard.Shim(guardHash))
	for _, target := range targets {
		if target.Target != "pr_template" {
			continue
		}
		if ok, err := s.ownedByDashcrew(PrTemplatePath, lockedFiles, "<!-- dashcrew:generated"); err != nil {
			return nil, err
		} else if ok {
			files.Set(PrTemplatePath, target.Content)
		} else if warnings != nil {
			*warnings = append(*warnings, PrTemplatePath+" exists and was not written by dashcrew; left as is. "+
				"dashcrew check still requires the headings dashcrew would write.")
		}
		break
	}
	if p.CI.Value("github") == true {
		if ok, err := s.ownedByDashcrew(GithubWorkflowPath, lockedFiles, "# Generated by dashcrew"); err != nil {
			return nil, err
		} else if ok {
			files.Set(GithubWorkflowPath, GithubWorkflow(p.CI.Value("dashcrew_ref").(string)))
		} else if warnings != nil {
			*warnings = append(*warnings, GithubWorkflowPath+" exists and was not written by dashcrew; left as is.")
		}
	}
	for _, adapter := range agentAdapters {
		adapterFiles, err := adapter.Files(p, targets, context, lockedFiles, warnings)
		if err != nil {
			return nil, err
		}
		for _, path := range adapterFiles.Keys() {
			content := adapterFiles.Value(path)
			if other, ok := files.Get(path); ok && other != content {
				return nil, fmt.Errorf("%s is written by two adapters with different content (%s)", path, adapter.Agent())
			}
			files.Set(path, content)
		}
	}
	return &Rendered{Files: files, Packs: packs, Adapters: agentAdapters}, nil
}

// ownedByDashcrew is true when path is missing, in the lock, or starts with marker.
func (s Sync) ownedByDashcrew(path string, lockedFiles map[string]string, marker string) (bool, error) {
	project := s.project()
	if _, locked := lockedFiles[path]; locked || !project.Exists(path) {
		return true, nil
	}
	current, err := project.Read(path)
	if err != nil {
		return false, err
	}
	return strings.HasPrefix(current, marker), nil
}

func (s Sync) profile(c *catalog.Catalog) (*profile.Profile, error) {
	project := s.project()
	if !project.Exists(".ai/project.yaml") {
		return nil, &Error{Message: "No .ai/project.yaml in " + s.ProjectDir + ". Run dashcrew init first."}
	}
	text, err := project.Read(".ai/project.yaml")
	if err != nil {
		return nil, err
	}
	p, err := profile.Parse(text, c.KnownOptions())
	if err != nil {
		var profileErr *profile.Error
		if errors.As(err, &profileErr) {
			return nil, &Error{Message: err.Error()}
		}
		return nil, err
	}
	return p, nil
}

// readLock is `.ai/lock.json`; empty when there is no lock.
func (s Sync) readLock() (*omap.Map[any], error) {
	project := s.project()
	if !project.Exists(LockPath) {
		return omap.New[any](), nil
	}
	text, err := project.Read(LockPath)
	if err != nil {
		return nil, err
	}
	value, err := dartjson.Decode(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", LockPath, err)
	}
	lock, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fmt.Errorf("%s: expected a JSON object", LockPath)
	}
	return lock, nil
}

// lockFiles are the `files` of a lock, path -> hash, in lock order; empty when absent.
func lockFiles(lock *omap.Map[any]) *omap.Map[string] {
	out := omap.New[string]()
	files, _ := lock.Value("files").(*omap.Map[any])
	if files == nil {
		return out
	}
	for _, path := range files.Keys() {
		hash, _ := files.Value(path).(string)
		out.Set(path, hash)
	}
	return out
}

// ownedMcp are the MCP servers each agent's adapter wrote before; a lock that keeps a plain list predates Codex
// and means Claude Code.
func ownedMcp(lock *omap.Map[any]) map[string]map[string]bool {
	names := func(list []any) map[string]bool {
		out := map[string]bool{}
		for _, item := range list {
			if name, ok := item.(string); ok {
				out[name] = true
			}
		}
		return out
	}
	switch owned := lock.Value("mcp_servers").(type) {
	case []any:
		return map[string]map[string]bool{"claude-code": names(owned)}
	case *omap.Map[any]:
		out := map[string]map[string]bool{}
		for _, agent := range owned.Keys() {
			list, _ := owned.Value(agent).([]any)
			out[agent] = names(list)
		}
		return out
	}
	return map[string]map[string]bool{}
}

// FileHash is the hash stored in the lock for one generated file. AGENTS.md is hashed without the project block,
// so editing that block never counts as a hand edit.
func FileHash(path, content string) string {
	hashed := content
	if path == "AGENTS.md" {
		hashed = withoutProjectBlock(content)
	}
	sum := sha256.Sum256([]byte(hashed))
	return hex.EncodeToString(sum[:])
}

func withoutProjectBlock(content string) string {
	begin := strings.Index(content, ProjectBegin)
	end := strings.Index(content, ProjectEnd)
	if begin < 0 || end < begin {
		return content
	}
	return content[:begin+len(ProjectBegin)] + content[end:]
}

func (s Sync) agentsMd(targets []render.RenderedTarget, adopt bool) (string, error) {
	project := s.project()
	projectBlock := ""
	if project.Exists("AGENTS.md") {
		existing, err := project.Read("AGENTS.md")
		if err != nil {
			return "", err
		}
		begin := strings.Index(existing, ProjectBegin)
		end := strings.Index(existing, ProjectEnd)
		switch {
		case strings.Contains(existing, "dashcrew:generated") && begin >= 0 && end > begin:
			projectBlock = existing[begin+len(ProjectBegin) : end]
		case adopt:
			projectBlock = "\n" + render.TrimRight(existing) + "\n"
		default:
			return "", &Error{Message: "AGENTS.md exists and was not written by dashcrew. Rerun with --adopt to keep it as the project block."}
		}
	}
	var rules strings.Builder
	for _, target := range targets {
		if target.Target == "agents_md" {
			rules.WriteString(target.Content)
		}
	}
	if projectBlock == "" {
		projectBlock = "\n"
	}
	return GeneratedMarker + "\n\n" + render.TrimRight(rules.String()) + "\n\n" + ProjectBegin + projectBlock + ProjectEnd + "\n", nil
}

// removeCompiledGuard deletes a compiled guard from before the shim: it is binary, never in the lock, and
// dashcrew's own. A shim starts with `#!`.
func (s Sync) removeCompiledGuard() error {
	file := s.project().Path(guard.ShimPath)
	if info, err := os.Stat(file); err != nil || info.IsDir() {
		return nil
	}
	handle, err := os.Open(file)
	if err != nil {
		return err
	}
	head := make([]byte, 2)
	n, _ := io.ReadFull(handle, head)
	handle.Close()
	if n == 2 && head[0] == '#' && head[1] == '!' {
		return nil
	}
	return os.Remove(file)
}

// dropGuardIgnore removes the `.ai/bin/` lines dashcrew once added: the shim there is meant to be committed.
func (s Sync) dropGuardIgnore() (bool, error) {
	project := s.project()
	if !project.Exists(".gitignore") {
		return false, nil
	}
	current, err := project.Read(".gitignore")
	if err != nil {
		return false, err
	}
	next := strings.ReplaceAll(current, guardIgnoreLines, "")
	if next == current {
		return false, nil
	}
	return project.WriteIfChanged(".gitignore", next)
}

// GithubWorkflow runs `dashcrew check` on every pull request with dashcrew pinned at dashcrewRef, so CI judges
// a PR with the same dashcrew the team uses.
func GithubWorkflow(dashcrewRef string) string {
	return `# Generated by dashcrew sync. Change ci in .ai/project.yaml and run dashcrew sync instead of editing this file.
name: dashcrew check

on:
  pull_request:
    types: [opened, edited, synchronize, reopened]

permissions:
  contents: read
  pull-requests: read

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: dart-lang/setup-dart@v1
        with:
          sdk: stable
      - run: dart pub global activate --source git https://github.com/fahnaladitia/dashcrew --git-ref ` + dashcrewRef + `
      - run: dart pub global run dashcrew check
`
}
