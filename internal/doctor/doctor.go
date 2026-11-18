// Package doctor is the read-only health check of the toolkit sync wrote into one project.
// It never writes; `dashcrew sync` is the repair path for everything it reports.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/agents"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/lines"
	"github.com/fahnaladitia/dashcrew/internal/me"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/sync"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// Status of one check. Info reports something worth knowing that needs no fix, such as files left from an
// older dashcrew.
type Status int

const (
	OK Status = iota
	Warn
	Fail
	Skip
	Info
)

// String is the name printed by the CLI: ok, warn, fail, skip or info.
func (s Status) String() string {
	return [...]string{"ok", "warn", "fail", "skip", "info"}[s]
}

// Check is the result of one check; Detail is a single line meant for the terminal.
type Check struct {
	ID     string
	Status Status
	Detail string
}

// AgentsMdLimit is the AGENTS.md size most agents read in full.
const AgentsMdLimit = 32 * 1024

// AgentsMdLimitFor is the AGENTS.md size every selected agent reads in full: the smallest of their limits.
func AgentsMdLimitFor(selected map[string]bool) int {
	limit := AgentsMdLimit
	for agent, on := range selected {
		if a, ok := agentChecks[agent]; ok && on && a.agentsMdLimit < limit {
			limit = a.agentsMdLimit
		}
	}
	return limit
}

// Doctor checks one project.
type Doctor struct {
	Catalog    *catalog.Catalog
	ProjectDir string
	// UserSkillsDir is where user-level skills live; empty means `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`.
	UserSkillsDir string
	// CodexHome is the Codex user config folder; empty means `$CODEX_HOME`, else `~/.codex`.
	CodexHome string
	// PathEnv is the PATH searched for the dashcrew the shim runs, and the shim's PATH; empty means this process's.
	PathEnv string
	// Home is the shim's HOME, where it caches a passed version check; empty means this process's.
	Home string
	// MachineChecks runs what depends on this machine or an agent (guard binary, hooks, MCP, skills, trust,
	// the knowledge link and its staleness); off in CI.
	MachineChecks bool
	// Fetch fetches the knowledge clone to report how many commits it is behind its remote.
	Fetch bool
	// Now dates the knowledge clone's last pull and its staleness; nil means time.Now.
	Now func() time.Time
	// Me is where the personal profile and its files live; nil skips the me check, so only the CLI reads them.
	Me *me.Places
}

var (
	skillReference = regexp.MustCompile("`([a-z0-9][a-z0-9-]*)` skill|skill `([a-z0-9][a-z0-9-]*)`")
	// indexEntry is one line of the AGENTS.md skill index: "- `x`: ..." or "- `x` (review): ...".
	indexEntry   = regexp.MustCompile("(?m)^- `([a-z0-9][a-z0-9-]*)`(?: \\(review\\))?: ")
	toolsLine    = regexp.MustCompile(`(?m)^tools: \[(` + lineChar + `*)\](?:$|\r|\x{2028}|\x{2029})`)
	projectTable = regexp.MustCompile(`(?m)^` + jsSpace + `*\[projects\.(?:"([^"]+)"|'([^']+)')\]` + jsSpace + `*$`)
	trustedLine  = regexp.MustCompile(`(?m)^` + jsSpace + `*trust_level` + jsSpace + `*=` + jsSpace + `*["']trusted["']`)
	nextTable    = regexp.MustCompile(`(?m)^` + jsSpace + `*\[`)
)

const (
	// lineChar is `.` of a Dart regular expression, and jsSpace its `\s`.
	lineChar = `[^\n\r\x{2028}\x{2029}]`
	jsSpace  = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`
)

// Run returns every check, with the project packs in `.ai/packs/` overlaid on the catalog when they parse.
func (d Doctor) Run() []Check {
	overlaid, err := d.Catalog.WithProjectPacks(d.ProjectDir)
	if err != nil {
		return d.checks(Check{"project-packs", Fail, err.Error()})
	}
	d.Catalog = overlaid
	return d.checks(d.projectPacks())
}

func (d Doctor) checks(projectPacks Check) []Check {
	if d.guardOnly() {
		return d.guardOnlyChecks()
	}
	profileCheck := d.profileCheck()
	lockCheck, lock, lockConflicted := d.lockCheck()
	filesFrom := lock
	if lockConflicted {
		filesFrom, _, _ = sync.Sync{Catalog: d.Catalog, ProjectDir: d.ProjectDir}.MergeLock()
	}
	var lockedFiles *omap.Map[string]
	if filesFrom != nil {
		if files, ok := filesFrom.Value("files").(*omap.Map[any]); ok {
			lockedFiles = omap.New[string]()
			for _, path := range files.Keys() {
				hash, _ := files.Value(path).(string)
				lockedFiles.Set(path, hash)
			}
		}
	}
	selected := d.agents()
	line := d.releaseLine()
	checks := []Check{
		profileCheck,
		projectPacks,
		lockCheck,
		d.version(lock, line),
		d.drift(lockedFiles, lockConflicted),
		d.stale(lockedFiles, profileCheck.Status != Fail && !lockConflicted, line),
		d.guardPolicy(),
	}
	if d.MachineChecks {
		checks = append(checks, d.guardBinary(lock))
		if guardLog, ok := d.guardLog(); ok {
			checks = append(checks, guardLog)
		}
		for _, agent := range agents.All() {
			if a, ok := agentChecks[agent]; ok && selected[agent] {
				checks = append(checks, a.hook(d))
				checks = append(checks, a.machine(d, lockedFiles)...)
			}
		}
		if knowledgeCheck, ok := d.knowledge(lock); ok {
			checks = append(checks, knowledgeCheck)
		}
		if staleCheck, ok := d.knowledgeStale(); ok {
			checks = append(checks, staleCheck)
		}
		if specCheck, ok := d.specDrift(); ok {
			checks = append(checks, specCheck)
		}
		if lineCheck, ok := d.releaseLineCheck(lock, line); ok {
			checks = append(checks, lineCheck)
		}
		if linksCheck, ok := d.links(); ok {
			checks = append(checks, linksCheck)
		}
		if trackerCheck, ok := d.githubTracker(); ok {
			checks = append(checks, trackerCheck)
		}
		if d.Me != nil {
			checks = append(checks, d.me())
		}
	}
	checks = append(checks, d.agentsMdSize(AgentsMdLimitFor(selected)), d.alwaysOn(selected))
	if memory, ok := d.memoryIndex(); ok {
		checks = append(checks, memory)
	}
	checks = append(checks, d.gitignore())
	if rosters, ok := d.handoffRosters(); ok {
		checks = append(checks, rosters)
	}
	return checks
}

// guardOnly is true for a project that `dashcrew guard init` set up and nothing more: no profile, and the
// default policy as it wrote it.
func (d Doctor) guardOnly() bool {
	if d.exists(".ai/project.yaml") {
		return false
	}
	text, err := d.read(sync.GuardPolicyPath)
	return err == nil && text == guard.InitPolicyText()
}

// guardOnlyChecks are the guard checks of a guard-only project: the policy, the shim, the log, and the hook of
// each agent whose hook file exists (Claude Code's when none does). Their fixes name `guard init`, since sync needs
// a profile.
func (d Doctor) guardOnlyChecks() []Check {
	checks := []Check{d.guardPolicy()}
	if d.MachineChecks {
		checks = append(checks, d.guardBinary(nil))
		if guardLog, ok := d.guardLog(); ok {
			checks = append(checks, guardLog)
		}
		hooked := false
		for _, agent := range agents.All() {
			if a, ok := agentChecks[agent]; ok && d.exists(adapters.HookFile(agent)) {
				checks = append(checks, a.hook(d))
				hooked = true
			}
		}
		if !hooked {
			checks = append(checks, agentChecks["claude-code"].hook(d))
		}
	}
	checks = append(checks, d.gitignore())
	for i := range checks {
		checks[i].Detail = strings.ReplaceAll(checks[i].Detail, "run dashcrew sync", "run dashcrew guard init")
	}
	return append(checks, Check{"toolkit", Info, "guard only (dashcrew guard init); run dashcrew init and dashcrew sync " +
		"for the rules, skills and the rest of the toolkit"})
}

func (d Doctor) path(relative string) string {
	return filepath.Join(d.ProjectDir, filepath.FromSlash(relative))
}

// isFile is a regular file, or a link to one, like Dart's `File.existsSync`.
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (d Doctor) exists(relative string) bool { return isFile(d.path(relative)) }

func (d Doctor) read(relative string) (string, error) {
	data, err := os.ReadFile(d.path(relative))
	return string(data), err
}

func (d Doctor) parseProfile() (*profile.Profile, error) {
	text, err := d.read(".ai/project.yaml")
	if err != nil {
		return nil, err
	}
	return profile.Parse(text, d.Catalog.KnownOptions())
}

// agents of the profile; Claude Code alone when the profile cannot be read, as before Codex support.
func (d Doctor) agents() map[string]bool {
	out := map[string]bool{}
	p, err := d.parseProfile()
	if err != nil {
		out["claude-code"] = true
		return out
	}
	for _, agent := range p.Agents {
		out[agent] = true
	}
	return out
}

// platform of the profile; the default one when the profile cannot be read.
func (d Doctor) platform() string {
	p, err := d.parseProfile()
	if err != nil {
		return profile.DefaultPlatform
	}
	return p.Platform
}

// profileKnowledge is the profile's `knowledge`; nil without it or when the profile cannot be read.
func (d Doctor) profileKnowledge() *profile.Knowledge {
	p, err := d.parseProfile()
	if err != nil {
		return nil
	}
	return p.Knowledge
}

// readObject decodes a JSON file whose top level must be an object.
func (d Doctor) readObject(relative string) (*omap.Map[any], error) {
	text, err := d.read(relative)
	if err != nil {
		return nil, err
	}
	value, err := dartjson.Decode(text)
	if err != nil {
		return nil, err
	}
	object, ok := value.(*omap.Map[any])
	if !ok {
		return nil, errors.New("the top level is not a JSON object")
	}
	return object, nil
}

func encode(value any) string {
	text, _ := dartjson.Encode(value)
	return text
}

// preToolUse is `hooks.PreToolUse` of a hooks config; empty when either is absent.
func preToolUse(config *omap.Map[any]) ([]any, error) {
	hooks := config.Value("hooks")
	if hooks == nil {
		return nil, nil
	}
	hooksMap, ok := hooks.(*omap.Map[any])
	if !ok {
		return nil, errors.New("hooks is not a map")
	}
	entries := hooksMap.Value("PreToolUse")
	if entries == nil {
		return nil, nil
	}
	list, ok := entries.([]any)
	if !ok {
		return nil, errors.New("hooks.PreToolUse is not a list")
	}
	return list, nil
}

// mcpServers is `mcpServers` of an MCP config; empty when absent.
func mcpServers(config *omap.Map[any]) (*omap.Map[any], error) {
	value := config.Value("mcpServers")
	if value == nil {
		return omap.New[any](), nil
	}
	servers, ok := value.(*omap.Map[any])
	if !ok {
		return nil, errors.New("mcpServers is not a map")
	}
	return servers, nil
}

// searchPath is the PATH doctor searches: PathEnv, else this process's.
func (d Doctor) searchPath() string {
	if d.PathEnv != "" {
		return d.PathEnv
	}
	return os.Getenv("PATH")
}

// HookChecks are the guard hook checks of the profile's agents, read from the committed files only (no agent is
// asked): what `dashcrew check` adds in CI, where a hand edit or a merge can drop a hook without anyone noticing.
func (d Doctor) HookChecks() []Check {
	selected := d.agents()
	var checks []Check
	for _, agent := range agents.All() {
		if a, ok := agentChecks[agent]; ok && selected[agent] {
			checks = append(checks, a.hook(d))
		}
	}
	return checks
}

// guardEntryCheck passes when one of entries is the guard hook sync writes.
func guardEntryCheck(id string, entries []any, ours, okDetail string) Check {
	edited := false
	for _, entry := range entries {
		text := encode(entry)
		if text == ours {
			return Check{id, OK, okDetail}
		}
		if strings.Contains(text, ".ai/bin/guard") {
			edited = true
		}
	}
	if edited {
		return Check{id, Fail, "PreToolUse guard hook differs from what sync writes; run dashcrew sync"}
	}
	return Check{id, Fail, "PreToolUse guard hook missing; run dashcrew sync"}
}

func (d Doctor) profileCheck() Check {
	const id = "profile"
	if !d.exists(".ai/project.yaml") {
		return Check{id, Fail, ".ai/project.yaml missing; run dashcrew init"}
	}
	p, err := d.parseProfile()
	if err != nil {
		return Check{id, Fail, ".ai/project.yaml invalid: " + err.Error()}
	}
	if len(p.Warnings) > 0 {
		return Check{id, Warn, strings.Join(p.Warnings, "; ")}
	}
	return Check{id, OK, ".ai/project.yaml valid"}
}

func (d Doctor) projectPacks() Check {
	const id = "project-packs"
	var names []string
	for _, family := range d.Catalog.Packs.Keys() {
		options := d.Catalog.Packs.Value(family)
		for _, option := range options.Keys() {
			if pack := options.Value(option); pack.FromProject {
				names = append(names, pack.Family+"/"+pack.Option)
			}
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return Check{id, OK, "no project packs in .ai/packs"}
	}
	var unused []string
	if p, err := d.parseProfile(); err == nil {
		for _, name := range names {
			family, option, _ := strings.Cut(name, "/")
			if choice, ok := p.Stack.Get(family); !ok || choice.Option != option {
				unused = append(unused, name)
			}
		}
	}
	if len(unused) > 0 {
		return Check{id, Warn, "not selected in .ai/project.yaml: " + strings.Join(unused, ", ")}
	}
	return Check{id, OK, fmt.Sprintf("%d project packs: %s", len(names), strings.Join(names, ", "))}
}

// lockCheck reads the lock; conflicted is true when it holds merge conflict markers.
func (d Doctor) lockCheck() (check Check, lock *omap.Map[any], conflicted bool) {
	const id = "lock"
	if !d.exists(sync.LockPath) {
		return Check{id, Fail, sync.LockPath + " missing; run dashcrew sync"}, nil, false
	}
	text, err := d.read(sync.LockPath)
	if err != nil {
		return Check{id, Fail, sync.LockPath + " is not JSON: " + err.Error()}, nil, false
	}
	if sync.HasConflictMarkers(text) {
		return Check{id, Fail, sync.LockPath + " has conflict markers; run dashcrew sync --after-merge, then stage and commit what it lists"}, nil, true
	}
	value, err := dartjson.Decode(text)
	if err != nil {
		return Check{id, Fail, sync.LockPath + " is not JSON: " + err.Error()}, nil, false
	}
	lock, ok := value.(*omap.Map[any])
	if !ok || !isOne(lock.Value("schema")) {
		return Check{id, Fail, sync.LockPath + " has an unknown schema; run dashcrew sync --force"}, nil, false
	}
	return Check{id, OK, sync.LockPath + " schema 1"}, lock, false
}

// isOne is Dart's `value == 1`, which a JSON 1.0 also meets.
func isOne(value any) bool {
	switch v := value.(type) {
	case int:
		return v == 1
	case float64:
		return v == 1
	}
	return false
}

// version warns when teammates on different dashcrew versions would rewrite each other's generated files.
func (d Doctor) version(lock *omap.Map[any], line *releaseLine) Check {
	const id = "version"
	if lock == nil {
		return Check{id, Skip, "needs the lock"}
	}
	written, ok := lock.Value("dashcrew_version").(string)
	if !ok {
		return Check{id, Warn, "the lock has no dashcrew version; run dashcrew sync to record " + version.Current()}
	}
	current := version.Current()
	switch order := version.Compare(written, current); {
	case order > 0:
		return Check{id, Warn, "the lock was written by dashcrew " + written + ", this is " + current + "; update dashcrew to " + written}
	case order < 0 && line != nil && line.resolution.Protected():
		return Check{id, Info, "the lock was written by dashcrew " + written + "; this release line keeps it until dashcrew sync --upgrade"}
	case order < 0:
		return Check{id, Warn, "the lock was written by dashcrew " + written + "; run dashcrew sync to move it to " + version.Current()}
	}
	return Check{id, OK, "lock written by this dashcrew (" + version.Current() + ")"}
}

// drift fails generated files that are missing, carry merge conflict markers or differ from the lock. After a
// merge (markers, a conflicted lock or a MERGE_HEAD) the fix is `sync --after-merge`, else `sync --force`.
func (d Doctor) drift(lockedFiles *omap.Map[string], lockConflicted bool) Check {
	const id = "drift"
	if lockedFiles == nil {
		return Check{id, Skip, "needs the lock"}
	}
	paths := lockedFiles.Keys()
	sort.Strings(paths)
	var missing, conflicted, edited []string
	for _, path := range paths {
		if !d.exists(path) {
			missing = append(missing, path)
			continue
		}
		content, err := d.read(path)
		switch {
		case err == nil && sync.HasConflictMarkers(content):
			conflicted = append(conflicted, path)
		case err != nil || sync.FileHash(path, content) != lockedFiles.Value(path):
			edited = append(edited, path)
		}
	}
	if len(missing) == 0 && len(conflicted) == 0 && len(edited) == 0 {
		return Check{id, OK, fmt.Sprintf("%d generated files match the lock", lockedFiles.Len())}
	}
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(missing, ", "))
	}
	if len(conflicted) > 0 {
		parts = append(parts, "conflict markers: "+strings.Join(conflicted, ", "))
	}
	if len(edited) > 0 {
		parts = append(parts, "edited by hand: "+strings.Join(edited, ", "))
	}
	if len(conflicted) > 0 || lockConflicted || lines.MergeInProgress(d.ProjectDir) {
		return Check{id, Fail, strings.Join(parts, "; ") + " (dashcrew sync --after-merge renders them fresh; stage and commit what it lists)"}
	}
	return Check{id, Fail, strings.Join(parts, "; ") + " (dashcrew sync --force restores them)"}
}

func (d Doctor) stale(lockedFiles *omap.Map[string], profileOK bool, line *releaseLine) Check {
	const id = "stale"
	if lockedFiles == nil || !profileOK {
		return Check{id, Skip, "needs a valid profile and the lock"}
	}
	rendered, err := sync.Sync{Catalog: d.Catalog, ProjectDir: d.ProjectDir}.Render(false, nil)
	if err != nil {
		return Check{id, Fail, err.Error()}
	}
	expected := rendered.Files
	seen := map[string]bool{}
	var paths []string
	for _, path := range append(expected.Keys(), lockedFiles.Keys()...) {
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	var changed []string
	for _, path := range paths {
		content, generated := expected.Get(path)
		locked, isLocked := lockedFiles.Get(path)
		if !generated || !isLocked || sync.FileHash(path, content) != locked {
			changed = append(changed, path)
		}
	}
	if len(changed) == 0 {
		return Check{id, OK, "output matches the current profile and catalog"}
	}
	held := map[string]bool{}
	if line != nil && line.resolution.Protected() {
		if hold, err := (sync.Sync{Catalog: d.Catalog, ProjectDir: d.ProjectDir}).Hold(); err == nil && hold != nil {
			for _, path := range hold.Files {
				held[path] = true
			}
		}
	}
	var heldFiles, writable []string
	for _, path := range changed {
		if held[path] {
			heldFiles = append(heldFiles, path)
		} else {
			writable = append(writable, path)
		}
	}
	shown := func(paths []string) string {
		if len(paths) > 5 {
			return strings.Join(paths[:5], ", ") + ", ..."
		}
		return strings.Join(paths, ", ")
	}
	if len(writable) == 0 {
		return Check{id, Info, "held at the lock's toolkit on this release line: " + shown(heldFiles) + "; dashcrew sync --upgrade writes them"}
	}
	detail := fmt.Sprintf("%d files would change (%s); run dashcrew sync", len(writable), shown(writable))
	if len(heldFiles) > 0 {
		detail += "; held on this release line until dashcrew sync --upgrade: " + shown(heldFiles)
	}
	return Check{id, Warn, detail}
}

// releaseLine is the branch's place among the profile's release lines.
type releaseLine struct {
	all        []lines.Line
	resolution lines.Resolution
}

// releaseLine is nil without `project.release_lines` or outside a git repository.
func (d Doctor) releaseLine() *releaseLine {
	p, err := d.parseProfile()
	if err != nil || p.ReleaseLines == nil {
		return nil
	}
	all, err := lines.List(d.ProjectDir, p.ReleaseLines)
	if err != nil {
		return nil
	}
	return &releaseLine{all: all, resolution: lines.Resolve(d.ProjectDir, p.ReleaseLines, all, "")}
}

// releaseLineCheck compares a release line's toolkit version with the main line's lock, read from git; false off
// a release line or when the main line's lock cannot be read.
func (d Doctor) releaseLineCheck(lock *omap.Map[any], line *releaseLine) (Check, bool) {
	const id = "release-line"
	if line == nil || !line.resolution.Protected() || lock == nil {
		return Check{}, false
	}
	main := lines.Main(line.all)
	if main == nil {
		return Check{}, false
	}
	text, ok := lines.Show(d.ProjectDir, main.Ref+":./"+sync.LockPath)
	if !ok {
		return Check{}, false
	}
	value, err := dartjson.Decode(text)
	mainLock, _ := value.(*omap.Map[any])
	if err != nil || mainLock == nil {
		return Check{}, false
	}
	mainVersion, _ := mainLock.Value("dashcrew_version").(string)
	ours, _ := lock.Value("dashcrew_version").(string)
	if mainVersion == "" || ours == "" {
		return Check{}, false
	}
	prefix := line.resolution.Describe() + ": toolkit "
	switch order := version.Compare(ours, mainVersion); {
	case order < 0:
		return Check{id, Info, prefix + ours + ", the main line " + main.Branch + " has " + mainVersion +
			"; dashcrew sync --upgrade moves this line"}, true
	case order > 0:
		return Check{id, Warn, prefix + ours + " is newer than the main line " + main.Branch + "'s " + mainVersion +
			"; upgrade the main line too"}, true
	}
	return Check{id, OK, prefix + ours + ", same as the main line " + main.Branch}, true
}

func (d Doctor) guardPolicy() Check {
	const id = "guard-policy"
	if !d.exists(sync.GuardPolicyPath) {
		return Check{id, Fail, ".ai/guard.json missing; run dashcrew sync"}
	}
	text, err := d.read(sync.GuardPolicyPath)
	if err == nil {
		_, err = guard.PolicyFromJSON(text)
	}
	if err != nil {
		return Check{id, Fail, ".ai/guard.json invalid: " + err.Error()}
	}
	return Check{id, OK, ".ai/guard.json valid"}
}

// guardBinary checks that the shim can run `dashcrew guard`: dashcrew on PATH (PathEnv), the Go build and not
// older than the lock's `guard_min_version`, then the shim blocks a `git push` event, which every policy blocks.
// The shim refuses every tool call without such a dashcrew, so a missing one fails and an old one warns.
func (d Doctor) guardBinary(lock *omap.Map[any]) Check {
	const id = "guard-binary"
	shim := d.path(guard.ShimPath)
	if !isFile(shim) {
		return Check{id, Fail, ".ai/bin/guard missing; run dashcrew sync"}
	}
	minimum := version.GuardMinimum
	if shimText, err := d.read(guard.ShimPath); err == nil && guard.ShimMinimum(shimText) != "" {
		minimum = guard.ShimMinimum(shimText)
	}
	if lock != nil {
		if recorded, ok := lock.Value("guard_min_version").(string); ok {
			minimum = recorded
		}
	}
	pathEnv := d.searchPath()
	install := "install dashcrew " + minimum + " or newer (brew install fahnaladitia/tap/dashcrew)"
	found, ok := lookPath("dashcrew", pathEnv)
	if !ok {
		return Check{id, Fail, "dashcrew is not on PATH, so .ai/bin/guard refuses every tool call: " + install}
	}
	if isDartLauncher(found) {
		return Check{id, Warn, "the dashcrew first on PATH (" + found + ") is the Dart one, so .ai/bin/guard refuses every tool call: " +
			install + ", then dart pub global deactivate dashcrew"}
	}
	if reported := reportedVersion(found, pathEnv); !version.IsSemantic(reported) || version.Compare(reported, minimum) < 0 {
		described := "reports no version"
		if version.IsSemantic(reported) {
			described = "is " + reported
		}
		return Check{id, Warn, "the dashcrew first on PATH (" + found + ") " + described + ", older than " + minimum +
			", so .ai/bin/guard refuses every tool call: " + install}
	}

	exitCode, err := runShim(shim, d.path(sync.GuardPolicyPath), d.ProjectDir, pathEnv, d.Home)
	if err != nil {
		return Check{id, Fail, ".ai/bin/guard cannot run: " + err.Error() + "; run dashcrew sync"}
	}
	if exitCode != 2 {
		return Check{id, Fail, fmt.Sprintf(".ai/bin/guard did not block git push (exit %d); run dashcrew sync", exitCode)}
	}
	return Check{id, OK, ".ai/bin/guard blocks git push"}
}

// lookPath is the first executable regular file named name in the folders of pathEnv; an empty folder is the
// working directory, as for `sh`.
func lookPath(name, pathEnv string) (string, bool) {
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			dir = "."
		}
		candidate := filepath.Join(dir, name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate, true
		}
	}
	return "", false
}

// isDartLauncher is true for the script `dart pub global activate` writes into the pub cache.
func isDartLauncher(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	head := make([]byte, 4096)
	n, _ := io.ReadFull(file, head)
	text := string(head[:n])
	return strings.HasPrefix(text, "#!") && strings.Contains(text, "pub global run dashcrew")
}

// reportedVersion is the last word `<path> --version` prints, as the shim reads it; empty after 10 seconds or
// when it fails.
func reportedVersion(path, pathEnv string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	words := strings.Fields(string(out))
	if len(words) == 0 {
		return ""
	}
	return words[len(words)-1]
}

// guardLog warns when the guard let tool calls through unchecked since the last sync: a hook payload it could not
// read (G4b, Go only) or a policy it could not use, so a rule may not have applied. False when there is none.
func (d Doctor) guardLog() (Check, bool) {
	count, path := guard.MalformedSinceSync(d.ProjectDir)
	if count == 0 {
		return Check{}, false
	}
	calls := "tool calls"
	if count == 1 {
		calls = "tool call"
	}
	return Check{"guard-log", Warn, fmt.Sprintf("the guard let %d %s through unchecked since the last sync, logged in %s "+
		"with the reasons; update dashcrew if an agent changed its hook format, run dashcrew sync for a broken policy",
		count, calls, path)}, true
}

// runShim feeds the shim a `git push` hook event and returns its exit code; -1 after 30 seconds.
// An error means the shim could not be started.
func runShim(shim, policy, projectDir, pathEnv, home string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shim, policy)
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)
	if home != "" {
		cmd.Env = append(cmd.Env, "HOME="+home)
	}
	event, _ := dartjson.Encode(omap.Of[any]("tool_name", "Bash", "tool_input", omap.Of[any]("command", "git push"), "cwd", projectDir))
	cmd.Stdin = strings.NewReader(event)
	err := cmd.Run()
	if ctx.Err() != nil {
		return -1, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return 0, err
	}
	return 0, nil
}

func (d Doctor) agentsMdSize(limit int) Check {
	const id = "agents-md-size"
	info, err := os.Stat(d.path("AGENTS.md"))
	if err != nil || info.IsDir() {
		return Check{id, Skip, "no AGENTS.md"}
	}
	size := int(info.Size())
	if size > limit {
		return Check{id, Warn, fmt.Sprintf("AGENTS.md is %d bytes, over %d; some agents truncate it", size, limit)}
	}
	return Check{id, OK, fmt.Sprintf("AGENTS.md %d of %d bytes", size, limit)}
}

// handoffRosters reports the roster files the pre-P4 team flow wrote under the plans path; they are the team's
// files, so they are only reported. False when there is none.
func (d Doctor) handoffRosters() (Check, bool) {
	plans, _ := profile.PathDefaults().Value("plans").(string)
	if p, err := d.parseProfile(); err == nil {
		if value, ok := p.Paths.Value("plans").(string); ok {
			plans = value
		}
	}
	dir := plans
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(d.ProjectDir, plans)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Check{}, false
	}
	var rosters []string
	for _, entry := range entries {
		plan := filepath.Join(dir, entry.Name())
		if isDir(plan) && isFile(filepath.Join(plan, "handoff", "00-roster.md")) {
			rosters = append(rosters, filepath.ToSlash(filepath.Join(plans, entry.Name(), "handoff", "00-roster.md")))
		}
	}
	if len(rosters) == 0 {
		return Check{}, false
	}
	sort.Strings(rosters)
	return Check{"handoff-rosters", Info, strings.Join(rosters, ", ") + " left from the pre-P4 team flow; nothing reads them now, delete them when done"}, true
}

func (d Doctor) gitignore() Check {
	const id = "gitignore"
	if text, err := d.read(".gitignore"); err == nil {
		for _, line := range strings.Split(text, "\n") {
			if strings.TrimSpace(line) == ".ai/bin/" {
				return Check{id, Warn, ".ai/bin/ is in .gitignore; remove it so the guard shim is committed"}
			}
		}
	}
	return Check{id, OK, ".ai/bin/guard is not ignored"}
}

// environment is the process environment as a map.
func environment() map[string]string {
	out := map[string]string{}
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok {
			out[key] = value
		}
	}
	return out
}

// homeDir is HOME, else USERPROFILE; false when neither is set.
func homeDir(env map[string]string) (string, bool) {
	if home, ok := env["HOME"]; ok {
		return home, true
	}
	home, ok := env["USERPROFILE"]
	return home, ok
}

// DefaultCodexHome is `$CODEX_HOME`, else `~/.codex`; empty when no home directory is known.
func DefaultCodexHome(env map[string]string) string {
	if codexHome := env["CODEX_HOME"]; codexHome != "" {
		return codexHome
	}
	if home, ok := homeDir(env); ok {
		return filepath.Join(home, ".codex")
	}
	return ""
}

// DefaultUserSkillsDir is `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`; empty when no home directory is
// known.
func DefaultUserSkillsDir(env map[string]string) string {
	if configDir := env["CLAUDE_CONFIG_DIR"]; configDir != "" {
		return filepath.Join(configDir, "skills")
	}
	if home, ok := homeDir(env); ok {
		return filepath.Join(home, ".claude", "skills")
	}
	return ""
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
