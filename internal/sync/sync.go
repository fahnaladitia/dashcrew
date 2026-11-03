// Package sync renders `.ai/project.yaml` into the agent files of one project.
//
// Generated files are tracked in `.ai/lock.json`; a file edited by hand stops the run unless forced. The guard
// shim runs `dashcrew guard` from PATH and accepts dashcrew version.GuardMinimum or newer, which the lock records.
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
	"github.com/fahnaladitia/dashcrew/internal/filemode"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/knowledge"
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
	// Home holds `.dashcrew/knowledge/<name>`, the knowledge clone; empty means the user's home folder. Read only
	// when the profile sets `knowledge`.
	Home string

	// profileText replaces .ai/project.yaml when set: the previous profile of a release line's hold.
	profileText *string
	// lock replaces .ai/lock.json when set: the merged lock of --after-merge.
	lock *omap.Map[any]
}

// Options of RunWith.
type Options struct {
	// Force overwrites files edited by hand and a lock from a newer dashcrew; Adopt keeps a foreign AGENTS.md.
	Force, Adopt bool
	// Upgrade lets a release line take this dashcrew's catalog; see LineHold.
	Upgrade bool
	// AfterMerge renders fresh the generated files dashcrew owns (per the lock) that hold conflict markers or
	// disagree with the lock, and lists every changed file in Result.Stage. It never stages or commits.
	AfterMerge bool
}

// Rendered is what Render produces: generated files in the order the Dart engine writes them,
// `family/option -> builtin|project` per rendered pack sorted by family, and the adapters whose Merge still runs.
type Rendered struct {
	Files    *omap.Map[string]
	Packs    *omap.Map[any]
	Adapters []adapters.Adapter
	// Knowledge is the profile's `knowledge`; nil without it.
	Knowledge *profile.Knowledge
	// Profile is the parsed profile the files were rendered from.
	Profile *profile.Profile
	// GuardMinimum is the oldest dashcrew the shim accepts for this policy (see guard.Policy.Minimum).
	GuardMinimum string
	// SkillModes are the supporting files of required project skills among Files, each with whether it is
	// executable in `.ai/skills`; Run sets or clears the execute bits to match after writing.
	SkillModes *omap.Map[bool]
}

// Result is what one Run changed. Notes are one-time information, such as P4MigrationNote; never a problem to fix.
type Result struct {
	Written  []string
	Removed  []string
	Warnings []string
	Notes    []string
	// Held are generated files a release line kept at the lock's toolkit, and HeldMessage says why and how to
	// proceed; empty when nothing was held.
	Held        []string
	HeldMessage string
	// Resolved are the owned files --after-merge rendered fresh; Stage is every file it changed, for the member
	// to stage. Both stay empty without AfterMerge.
	Resolved []string
	Stage    []string
}

func (s Sync) project() projectfiles.Project { return projectfiles.New(s.ProjectDir) }

// Run writes the rendered files that changed, removes locked files no longer generated, merges every adapter's
// config files and rewrites the lock. force overwrites files edited by hand and a lock from a newer dashcrew;
// adopt keeps a foreign AGENTS.md as the project block.
func (s Sync) Run(force, adopt bool) (*Result, error) {
	return s.RunWith(Options{Force: force, Adopt: adopt})
}

// RunWith is Run with the release line and merge options.
func (s Sync) RunWith(opts Options) (*Result, error) {
	force, adopt := opts.Force, opts.Adopt
	result := &Result{Written: []string{}, Removed: []string{}, Warnings: []string{}, Notes: []string{},
		Held: []string{}, Resolved: []string{}, Stage: []string{}}
	lockConflicted := false
	if opts.AfterMerge {
		if text, err := s.project().Read(ProfilePath); err == nil && HasConflictMarkers(text) {
			return nil, &Error{Message: ProfilePath + " has conflict markers. It is the team's file: resolve it by hand, " +
				"then rerun dashcrew sync --after-merge."}
		}
		merged, conflicted, err := s.MergeLock()
		if err != nil {
			return nil, err
		}
		s.lock, lockConflicted = merged, conflicted
	}
	rendered, err := s.Render(adopt, &result.Warnings)
	if err != nil {
		return nil, err
	}
	files := rendered.Files
	lock, err := s.readLock()
	if err != nil {
		return nil, err
	}
	current := version.Current()
	if lockVersion, ok := lock.Value("dashcrew_version").(string); ok && version.Compare(lockVersion, current) > 0 && !force {
		return nil, &Error{Message: LockPath + " was written by dashcrew " + lockVersion + ", and this is " + current + ". " +
			"Update dashcrew, or rerun with --force to rewrite it with " + current + "."}
	}
	lockedFiles := lockFiles(lock)
	profileText := ""
	if rendered.Profile.ReleaseLines != nil {
		if profileText, err = s.profileSource(); err != nil {
			return nil, err
		}
	}
	held := map[string]bool{}
	lineHold := s.hold(rendered, lock, profileText, opts.Upgrade)
	if lineHold != nil && len(lineHold.Files) > 0 {
		result.Held = lineHold.Files
		result.HeldMessage = lineHold.Message()
		for _, path := range lineHold.Files {
			held[path] = true
		}
	}
	if opts.AfterMerge && HasConflictMarkers(files.Value("AGENTS.md")) {
		result.Warnings = append(result.Warnings, "AGENTS.md: the project block has conflict markers; it is the team's text, resolve it by hand.")
	}

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
		if opts.AfterMerge && isLocked && (HasConflictMarkers(content) || locked != FileHash(path, content)) {
			result.Resolved = append(result.Resolved, path)
			continue
		}
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
		if held[path] {
			continue
		}
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
		if files.Has(path) || held[path] || project.LinkWithin(path) != nil || !project.Exists(path) {
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
	if err := filemode.SetExecutable(project.Path(guard.ShimPath), true); err != nil {
		return nil, err
	}
	for _, file := range rendered.SkillModes.Keys() {
		if held[file] {
			continue
		}
		if err := filemode.SetExecutable(project.Path(file), rendered.SkillModes.Value(file)); err != nil {
			return nil, err
		}
	}
	previousKnowledge, _ := lock.Value("knowledge").(*omap.Map[any])
	knowledgeEntry, err := s.syncKnowledge(rendered.Knowledge, previousKnowledge, result)
	if err != nil {
		return nil, err
	}
	dropped, err := s.dropGuardIgnore()
	if err != nil {
		return nil, err
	}
	var ignored []string
	if rendered.Knowledge != nil {
		ignored = append(ignored, "/"+knowledge.LinkPath)
	}
	blockWritten, ignoreRemoved, err := s.writeIgnoreBlock(ignored)
	if err != nil {
		return nil, err
	}
	if (dropped || blockWritten) && !ignoreRemoved {
		result.Written = append(result.Written, ".gitignore")
	}
	if ignoreRemoved {
		result.Removed = append(result.Removed, ".gitignore")
	}

	hashes := omap.New[string]()
	for _, path := range union(files.Keys(), lockedFiles.Keys()) {
		switch locked, isLocked := lockedFiles.Get(path); {
		case held[path] && isLocked:
			hashes.Set(path, locked)
		case files.Has(path) && !held[path]:
			hashes.Set(path, FileHash(path, files.Value(path)))
		}
	}
	toolkit := lockToolkit{version: current, guardMinimum: rendered.GuardMinimum, catalog: s.Catalog.Digest}
	if lineHold != nil {
		toolkit.version, _ = lock.Value("dashcrew_version").(string)
		if toolkit.version == "" {
			toolkit.version = current
		}
		if minimum, ok := lock.Value("guard_min_version").(string); ok {
			toolkit.guardMinimum = minimum
		}
		toolkit.catalog, _ = lockInputs(lock)
	}
	lockText, err := lockJSON(hashes, rendered.Packs, nowOwnedMcp, knowledgeEntry, toolkit,
		lineInputs(rendered.Profile, toolkit.catalog, profileText))
	if err != nil {
		return nil, err
	}
	wroteLock, err := project.WriteIfChanged(LockPath, lockText)
	if err != nil {
		return nil, err
	}
	if opts.AfterMerge {
		if lockConflicted && !contains(result.Resolved, LockPath) {
			result.Resolved = append(result.Resolved, LockPath)
		}
		stage := append(append(append([]string{}, result.Resolved...), result.Written...), result.Removed...)
		if wroteLock || lockConflicted {
			stage = append(stage, LockPath)
		}
		result.Stage = union(stage, nil)
		sort.Strings(result.Stage)
	}
	return result, nil
}

// lockToolkit is the toolkit a lock records: the dashcrew version, the guard minimum and the catalog digest.
type lockToolkit struct {
	version, guardMinimum, catalog string
}

// profileSource is the profile text sync renders: the override, else .ai/project.yaml.
func (s Sync) profileSource() (string, error) {
	if s.profileText != nil {
		return *s.profileText, nil
	}
	return s.project().Read(ProfilePath)
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// lockJSON is `.ai/lock.json` with the Dart key order: files sorted by path, agents of mcp_servers sorted.
// `guard_min_version` is the oldest dashcrew the shim accepts. `knowledge` (name, link, state, target, head) is
// written only when the profile sets it, and `inputs` (catalog digest, profile hash) last, only with release_lines.
func lockJSON(hashes *omap.Map[string], packs *omap.Map[any], ownedMcp map[string][]string, knowledgeEntry *omap.Map[any],
	toolkit lockToolkit, inputs *omap.Map[any]) (string, error) {
	paths := hashes.Keys()
	sort.Strings(paths)
	fileHashes := omap.New[any]()
	for _, path := range paths {
		fileHashes.Set(path, hashes.Value(path))
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
		"dashcrew_version", toolkit.version,
		"files", fileHashes,
		"packs", packs,
		"mcp_servers", servers,
		"guard_min_version", toolkit.guardMinimum,
	)
	if knowledgeEntry != nil {
		lock.Set("knowledge", knowledgeEntry)
	}
	if inputs != nil {
		lock.Set("inputs", inputs)
	}
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
		adapter, err := adapters.AdapterFor(agent, c, s.ProjectDir, p.Platform, p.Knowledge)
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
	compiled := guard.CompilePolicy(p, c.ResolvedFor(p.Platform))
	policy, err := dartjson.EncodeIndent(compiled.ToJSON(), "  ")
	if err != nil {
		return nil, err
	}
	files := omap.New[string]()
	files.Set("AGENTS.md", agentsMd)
	files.Set(GuardPolicyPath, policy+"\n")
	files.Set(guard.ShimPath, guard.Shim(compiled.Minimum()))
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
	return &Rendered{Files: files, Packs: packs, Adapters: agentAdapters, Knowledge: p.Knowledge, Profile: p,
		GuardMinimum: compiled.Minimum(), SkillModes: skillModes(c, p.Agents, files)}, nil
}

// skillModes are the supporting files of required project skills that files holds, with their executable flag.
// It reads only paths the adapters produced, so a skill or agent that wrote nothing has no entry.
func skillModes(c *catalog.Catalog, agents []string, files *omap.Map[string]) *omap.Map[bool] {
	out := omap.New[bool]()
	for _, skill := range c.ProjectSkills {
		for _, file := range skill.Files {
			for _, agent := range agents {
				if target := adapters.SupportingFile(agent, skill.Name, file.Path); files.Has(target) {
					out.Set(target, file.Executable)
				}
			}
		}
	}
	return out
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

// Profile is the project's parsed `.ai/project.yaml`, read against the catalog with the project packs overlaid.
func (s Sync) Profile() (*profile.Profile, error) {
	c, err := s.Catalog.WithProjectPacks(s.ProjectDir)
	if err != nil {
		return nil, &Error{Message: "Project pack: " + err.Error()}
	}
	return s.profile(c)
}

func (s Sync) profile(c *catalog.Catalog) (*profile.Profile, error) {
	project := s.project()
	if s.profileText == nil && !project.Exists(".ai/project.yaml") {
		return nil, &Error{Message: "No .ai/project.yaml in " + s.ProjectDir + ". Run dashcrew init first."}
	}
	text, err := s.profileSource()
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
	if s.lock != nil {
		return s.lock, nil
	}
	project := s.project()
	if !project.Exists(LockPath) {
		return omap.New[any](), nil
	}
	text, err := project.Read(LockPath)
	if err != nil {
		return nil, err
	}
	if HasConflictMarkers(text) {
		return nil, &Error{Message: LockPath + " has conflict markers from a merge; run dashcrew sync --after-merge."}
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

// Actions in the generated workflow are pinned to the same commits as this repository's own CI;
// TestGithubWorkflowPinsMatchTheRepoCI fails when Dependabot moves one and not the other.
const (
	checkoutAction = "actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1"
	setupGoAction  = "actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0"
)

// GithubWorkflow runs `dashcrew check` on every pull request with dashcrew pinned at dashcrewRef, so CI judges
// a PR with the same dashcrew the team uses. The full history is fetched so the PR's commits can be linted.
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
      - uses: ` + checkoutAction + `
        with:
          fetch-depth: 0
          persist-credentials: false
      - uses: ` + setupGoAction + `
        with:
          go-version: stable
          cache: false
      - run: go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@` + dashcrewRef + `
      - run: dashcrew check
`
}
