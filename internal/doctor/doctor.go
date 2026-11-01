// Package doctor is the read-only health check of the toolkit sync wrote into one project.
// It never writes; `dashcrew sync` is the repair path for everything it reports.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/guard"
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

// AgentsMdLimitFor is the AGENTS.md size the selected agents read in full: Antigravity cuts every rule file at
// 24,000 bytes.
func AgentsMdLimitFor(agents map[string]bool) int {
	if agents["antigravity"] {
		return 24000
	}
	return AgentsMdLimit
}

// Doctor checks one project.
type Doctor struct {
	Catalog    *catalog.Catalog
	ProjectDir string
	// GuardSource is the guard source hash this dashcrew builds (guard.SourceHash); empty skips the check that
	// the project expects the same guard.
	GuardSource string
	// UserSkillsDir is where user-level skills live; empty means `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`.
	UserSkillsDir string
	// CodexHome is the Codex user config folder; empty means `$CODEX_HOME`, else `~/.codex`.
	CodexHome string
	// GuardHome is where the shared guard is installed; empty means `DASHCREW_HOME`, else `~/.dashcrew`.
	GuardHome string
	// MachineChecks runs what depends on this machine or an agent (guard binary, hooks, MCP, skills, trust);
	// off in CI.
	MachineChecks bool
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
	profileCheck := d.profileCheck()
	lockCheck, lock := d.lockCheck()
	var lockedFiles *omap.Map[string]
	if lock != nil {
		if files, ok := lock.Value("files").(*omap.Map[any]); ok {
			lockedFiles = omap.New[string]()
			for _, path := range files.Keys() {
				hash, _ := files.Value(path).(string)
				lockedFiles.Set(path, hash)
			}
		}
	}
	agents := d.agents()
	checks := []Check{
		profileCheck,
		projectPacks,
		lockCheck,
		d.version(lock),
		d.drift(lockedFiles),
		d.stale(lockedFiles, profileCheck.Status != Fail),
		d.guardPolicy(),
	}
	if d.MachineChecks {
		checks = append(checks, d.guardBinary(lock))
		if agents["claude-code"] {
			checks = append(checks, d.hook(), d.claudeMd(), d.mcp(), d.skills(lockedFiles))
		}
		if agents["codex"] {
			checks = append(checks, d.codexHook(), d.codexMcp(), d.codexSkillsBudget(), d.codexTrust())
		}
		if agents["antigravity"] {
			checks = append(checks, d.antigravityHook(), d.antigravityMcp(), d.antigravityRoles())
		}
	}
	checks = append(checks, d.agentsMdSize(AgentsMdLimitFor(agents)), d.gitignore())
	if rosters, ok := d.handoffRosters(); ok {
		checks = append(checks, rosters)
	}
	return checks
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

func (d Doctor) antigravityHook() Check {
	const id = "antigravity-hook"
	if !d.exists(".agents/hooks.json") {
		return Check{id, Fail, ".agents/hooks.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".agents/hooks.json")
	if err != nil {
		return Check{id, Fail, ".agents/hooks.json invalid: " + err.Error()}
	}
	entry := config.Value(adapters.AntigravityGuardKey)
	if encode(entry) == encode(adapters.AntigravityGuardHook()) {
		return Check{id, OK, "PreToolUse guard hook present under " + adapters.AntigravityGuardKey}
	}
	problem := "guard hook differs from what sync writes"
	if entry == nil {
		problem = "guard hook missing"
	}
	return Check{id, Fail, problem + "; run dashcrew sync"}
}

func (d Doctor) antigravityMcp() Check {
	const id = "antigravity-mcp"
	if !d.exists(".agents/mcp_config.json") {
		return Check{id, Warn, ".agents/mcp_config.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".agents/mcp_config.json")
	var servers *omap.Map[any]
	if err == nil {
		servers, err = mcpServers(config)
	}
	if err != nil {
		return Check{id, Warn, ".agents/mcp_config.json invalid: " + err.Error()}
	}
	expected := adapters.NewAntigravity(d.Catalog, d.ProjectDir, d.platform()).McpServers().Keys()
	var missing []string
	for _, name := range expected {
		if !servers.Has(name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Check{id, Warn, "missing: " + strings.Join(missing, ", ") + "; run dashcrew sync"}
	}
	return Check{id, OK, "defined: " + strings.Join(expected, ", ")}
}

// antigravityRoles fails a role whose `tools` names a tool Antigravity does not know: the subagent may hang.
func (d Doctor) antigravityRoles() Check {
	const id = "antigravity-roles"
	dir := d.path(".agents/agents")
	if !isDir(dir) {
		return Check{id, Fail, ".agents/agents missing; run dashcrew sync"}
	}
	entries, _ := os.ReadDir(dir)
	var unknown []string
	count := 0
	for _, entry := range entries {
		file := filepath.Join(dir, entry.Name())
		if !strings.HasSuffix(entry.Name(), ".md") || !isFile(file) {
			continue
		}
		count++
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		tools := ""
		if match := toolsLine.FindStringSubmatch(string(data)); match != nil {
			tools = match[1]
		}
		for _, tool := range strings.Split(tools, ",") {
			tool = strings.TrimSpace(tool)
			if tool != "" && !contains(adapters.AntigravityKnownTools, tool) {
				unknown = append(unknown, strings.TrimSuffix(entry.Name(), ".md")+": "+tool)
			}
		}
	}
	if len(unknown) > 0 {
		return Check{id, Fail, "unknown tool names (the subagent may hang): " + strings.Join(unknown, ", ")}
	}
	return Check{id, OK, strconv.Itoa(count) + " roles, every tool name known"}
}

func (d Doctor) codexHook() Check {
	const id = "codex-hook"
	if !d.exists(".codex/hooks.json") {
		return Check{id, Fail, ".codex/hooks.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".codex/hooks.json")
	var entries []any
	if err == nil {
		entries, err = preToolUse(config)
	}
	if err != nil {
		return Check{id, Fail, ".codex/hooks.json invalid: " + err.Error()}
	}
	return guardEntryCheck(id, entries, encode(adapters.CodexGuardHook()), "PreToolUse guard hook present; Codex runs it once approved in /hooks")
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

func (d Doctor) codexMcp() Check {
	const id = "codex-mcp"
	toml := ""
	if d.exists(".codex/config.toml") {
		toml, _ = d.read(".codex/config.toml")
	}
	expected := adapters.NewCodex(d.Catalog, d.ProjectDir, d.platform()).McpServerNames()
	var missing []string
	for _, name := range expected {
		if !adapters.DefinesMcpServer(toml, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Check{id, Warn, ".codex/config.toml does not define " + strings.Join(missing, ", ") + "; run dashcrew sync"}
	}
	detail := "defined: " + strings.Join(expected, ", ")
	if !strings.Contains(toml, "# dashcrew:begin") {
		detail += " (by the user, outside the dashcrew block)"
	}
	return Check{id, OK, detail}
}

func (d Doctor) codexSkillsBudget() Check {
	const id = "codex-skills-budget"
	dir := d.path(".agents/skills")
	var contents []string
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			sub := filepath.Join(dir, entry.Name())
			skill := filepath.Join(sub, "SKILL.md")
			if !isDir(sub) || !isFile(skill) {
				continue
			}
			data, err := os.ReadFile(skill)
			if err != nil {
				continue
			}
			contents = append(contents, string(data))
		}
	}
	size := adapters.SkillListChars(contents)
	budget := adapters.SkillListBudget
	if size > budget {
		return Check{id, Warn, fmt.Sprintf("skill list is %d characters, over %d; Codex may drop descriptions", size, budget)}
	}
	return Check{id, OK, fmt.Sprintf("%d skills, list %d of %d characters", len(contents), size, budget)}
}

// codexTrust: Codex loads `.codex/` only for a trusted project; trust is set by Codex itself, never by dashcrew.
func (d Doctor) codexTrust() Check {
	const id = "codex-trust"
	home := d.CodexHome
	if home == "" {
		home = DefaultCodexHome(environment())
	}
	toml := ""
	if home != "" {
		if data, err := os.ReadFile(filepath.Join(home, "config.toml")); err == nil {
			toml = string(data)
		}
	}
	var trusted []string
	for _, match := range projectTable.FindAllStringSubmatchIndex(toml, -1) {
		rest := toml[match[1]:]
		if next := nextTable.FindStringIndex(rest); next != nil {
			rest = rest[:next[0]]
		}
		if !trustedLine.MatchString(rest) {
			continue
		}
		if match[2] >= 0 {
			trusted = append(trusted, toml[match[2]:match[3]])
		} else {
			trusted = append(trusted, toml[match[4]:match[5]])
		}
	}
	here := []string{d.ProjectDir}
	if isDir(d.ProjectDir) {
		if resolved, err := filepath.EvalSymlinks(d.ProjectDir); err == nil {
			here = append(here, resolved)
		}
	}
	for _, path := range trusted {
		for _, dir := range here {
			if dir == path || isWithin(path, dir) {
				return Check{id, OK, "project trusted in the Codex config"}
			}
		}
	}
	return Check{id, Warn, "project not trusted in the Codex config, so .codex/ is ignored; trust it when Codex asks, then approve the guard in /hooks"}
}

// isWithin is true when child is strictly inside parent.
func isWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
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

func (d Doctor) lockCheck() (Check, *omap.Map[any]) {
	const id = "lock"
	if !d.exists(sync.LockPath) {
		return Check{id, Fail, sync.LockPath + " missing; run dashcrew sync"}, nil
	}
	text, err := d.read(sync.LockPath)
	if err != nil {
		return Check{id, Fail, sync.LockPath + " is not JSON: " + err.Error()}, nil
	}
	value, err := dartjson.Decode(text)
	if err != nil {
		return Check{id, Fail, sync.LockPath + " is not JSON: " + err.Error()}, nil
	}
	lock, ok := value.(*omap.Map[any])
	if !ok || !isOne(lock.Value("schema")) {
		return Check{id, Fail, sync.LockPath + " has an unknown schema; run dashcrew sync --force"}, nil
	}
	return Check{id, OK, sync.LockPath + " schema 1"}, lock
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
func (d Doctor) version(lock *omap.Map[any]) Check {
	const id = "version"
	if lock == nil {
		return Check{id, Skip, "needs the lock"}
	}
	written, ok := lock.Value("dashcrew_version").(string)
	if !ok {
		return Check{id, Warn, "the lock has no dashcrew version; run dashcrew sync to record " + version.Version}
	}
	switch order := version.Compare(written, version.Version); {
	case order > 0:
		return Check{id, Warn, "the lock was written by dashcrew " + written + ", this is " + version.Version + "; update dashcrew to " + written}
	case order < 0:
		return Check{id, Warn, "the lock was written by dashcrew " + written + "; run dashcrew sync to move it to " + version.Version}
	}
	return Check{id, OK, "lock written by this dashcrew (" + version.Version + ")"}
}

func (d Doctor) drift(lockedFiles *omap.Map[string]) Check {
	const id = "drift"
	if lockedFiles == nil {
		return Check{id, Skip, "needs the lock"}
	}
	paths := lockedFiles.Keys()
	sort.Strings(paths)
	var missing, edited []string
	for _, path := range paths {
		if !d.exists(path) {
			missing = append(missing, path)
			continue
		}
		content, err := d.read(path)
		if err != nil || sync.FileHash(path, content) != lockedFiles.Value(path) {
			edited = append(edited, path)
		}
	}
	if len(missing) == 0 && len(edited) == 0 {
		return Check{id, OK, fmt.Sprintf("%d generated files match the lock", lockedFiles.Len())}
	}
	var parts []string
	if len(missing) > 0 {
		parts = append(parts, "missing: "+strings.Join(missing, ", "))
	}
	if len(edited) > 0 {
		parts = append(parts, "edited by hand: "+strings.Join(edited, ", "))
	}
	return Check{id, Fail, strings.Join(parts, "; ") + " (dashcrew sync --force restores them)"}
}

func (d Doctor) stale(lockedFiles *omap.Map[string], profileOK bool) Check {
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
	shown := strings.Join(changed, ", ")
	if len(changed) > 5 {
		shown = strings.Join(changed[:5], ", ") + ", ..."
	}
	return Check{id, Warn, fmt.Sprintf("%d files would change (%s); run dashcrew sync", len(changed), shown)}
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

// guardBinary runs the committed shim on a `git push` event, which every policy blocks; the guard fails open,
// so anything but exit 2 means no guard rule applies on this machine.
func (d Doctor) guardBinary(lock *omap.Map[any]) Check {
	const id = "guard-binary"
	shim := d.path(guard.ShimPath)
	if !isFile(shim) {
		return Check{id, Fail, ".ai/bin/guard missing; run dashcrew sync"}
	}
	var expected *string
	if lock != nil {
		if hash, ok := lock.Value("guard_source").(string); ok {
			expected = &hash
		}
	}
	home, hasHome := d.GuardHome, d.GuardHome != ""
	if !hasHome {
		home, hasHome = guard.DefaultHome(environment())
	}
	if expected != nil && hasHome && !isFile(guard.BinaryPath(home, *expected)) {
		return Check{id, Fail, "the guard for this project is not installed on this machine, so no guard rule applies here yet: " +
			"run dashcrew sync (an agent tool call also installs it when dashcrew is on PATH)"}
	}

	exitCode, err := runShim(shim, d.path(sync.GuardPolicyPath), d.ProjectDir, home, hasHome)
	if err != nil {
		return Check{id, Fail, ".ai/bin/guard cannot run: " + err.Error() + "; run dashcrew sync"}
	}
	if exitCode != 2 {
		return Check{id, Fail, fmt.Sprintf(".ai/bin/guard did not block git push (exit %d); run dashcrew sync", exitCode)}
	}
	if d.GuardSource != "" && (expected == nil || *expected != d.GuardSource) {
		return Check{id, Warn, "this dashcrew builds a different guard than the project expects; install the dashcrew ref the team uses"}
	}
	return Check{id, OK, ".ai/bin/guard blocks git push"}
}

// runShim feeds the shim a `git push` hook event and returns its exit code; -1 after 30 seconds.
// An error means the shim could not be started.
func runShim(shim, policy, projectDir, home string, hasHome bool) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shim, policy)
	cmd.Dir = projectDir
	cmd.Env = os.Environ()
	if hasHome {
		cmd.Env = append(cmd.Env, "DASHCREW_HOME="+home)
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

func (d Doctor) hook() Check {
	const id = "hook"
	if !d.exists(".claude/settings.json") {
		return Check{id, Fail, ".claude/settings.json missing; run dashcrew sync"}
	}
	settings, err := d.readObject(".claude/settings.json")
	var entries []any
	if err == nil {
		entries, err = preToolUse(settings)
	}
	if err != nil {
		return Check{id, Fail, ".claude/settings.json invalid: " + err.Error()}
	}
	return guardEntryCheck(id, entries, encode(adapters.ClaudeGuardHook()), "PreToolUse guard hook present")
}

func (d Doctor) claudeMd() Check {
	const id = "claude-md"
	if !d.exists("CLAUDE.md") {
		return Check{id, Fail, "CLAUDE.md missing; run dashcrew sync"}
	}
	if text, _ := d.read("CLAUDE.md"); !strings.Contains(text, "@AGENTS.md") {
		return Check{id, Fail, `CLAUDE.md does not import AGENTS.md; add a line "@AGENTS.md"`}
	}
	return Check{id, OK, "CLAUDE.md imports AGENTS.md"}
}

func (d Doctor) mcp() Check {
	const id = "mcp"
	if !d.exists(".mcp.json") {
		return Check{id, Warn, ".mcp.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".mcp.json")
	var servers *omap.Map[any]
	if err == nil {
		servers, err = mcpServers(config)
	}
	if err != nil {
		return Check{id, Warn, ".mcp.json invalid: " + err.Error()}
	}
	expected := adapters.NewClaudeCode(d.Catalog, d.ProjectDir, d.platform()).McpServers()
	var problems []string
	for _, name := range expected.Keys() {
		switch current := servers.Value(name); {
		case current == nil:
			problems = append(problems, name+" missing")
		case encode(current) != encode(expected.Value(name)):
			problems = append(problems, name+" defined differently")
		}
	}
	if len(problems) == 0 {
		return Check{id, OK, "defined: " + strings.Join(expected.Keys(), ", ")}
	}
	return Check{id, Warn, strings.Join(problems, ", ")}
}

// skills fails on a missing catalog skill and only warns on a missing external one, since plugins install
// skills elsewhere.
func (d Doctor) skills(lockedFiles *omap.Map[string]) Check {
	const id = "skills"
	if lockedFiles == nil {
		return Check{id, Skip, "needs the lock"}
	}
	referenced := map[string]bool{}
	for _, path := range lockedFiles.Keys() {
		if !strings.HasSuffix(path, ".md") || !d.exists(path) {
			continue
		}
		text, err := d.read(path)
		if err != nil {
			continue
		}
		for _, match := range skillReference.FindAllStringSubmatch(text, -1) {
			if match[1] != "" {
				referenced[match[1]] = true
			} else {
				referenced[match[2]] = true
			}
		}
		if path == "AGENTS.md" {
			for _, name := range skillIndexNames(text) {
				referenced[name] = true
			}
		}
	}

	platform := d.platform()
	dashcrewSkills := map[string]bool{}
	for name := range d.Catalog.SkillsFor(platform) {
		dashcrewSkills[name] = true
	}
	for name := range d.Catalog.WorkflowsFor(platform) {
		dashcrewSkills[name] = true
	}
	packs := d.Catalog.PacksFor(platform)
	for _, family := range packs.Keys() {
		options := packs.Value(family)
		for _, option := range options.Keys() {
			for name := range options.Value(option).Skills {
				dashcrewSkills[name] = true
			}
		}
	}
	userDir := d.UserSkillsDir
	if userDir == "" {
		userDir = DefaultUserSkillsDir(environment())
	}
	names := make([]string, 0, len(referenced))
	for name := range referenced {
		names = append(names, name)
	}
	sort.Strings(names)
	var missing, external []string
	for _, name := range names {
		if d.exists(".claude/skills/" + name + "/SKILL.md") {
			continue
		}
		if dashcrewSkills[name] {
			missing = append(missing, name)
		} else if userDir == "" || !isFile(filepath.Join(userDir, name, "SKILL.md")) {
			external = append(external, name)
		}
	}
	if len(missing) > 0 {
		return Check{id, Fail, "dashcrew skills missing: " + strings.Join(missing, ", ") + "; run dashcrew sync"}
	}
	if len(external) > 0 {
		return Check{id, Warn, "not found locally (install, or ignore if a plugin provides them): " + strings.Join(external, ", ")}
	}
	return Check{id, OK, fmt.Sprintf("%d referenced skills found", len(referenced))}
}

// skillIndexNames are the skill names listed under the `### Skill index` heading of AGENTS.md; empty when there
// is none.
func skillIndexNames(agentsMd string) []string {
	start := strings.Index(agentsMd, "### Skill index\n")
	if start < 0 {
		return nil
	}
	section := agentsMd[start:]
	if end := strings.Index(agentsMd[start+1:], "\n#"); end >= 0 {
		section = agentsMd[start : start+1+end]
	}
	var names []string
	for _, match := range indexEntry.FindAllStringSubmatch(section, -1) {
		names = append(names, match[1])
	}
	return names
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
