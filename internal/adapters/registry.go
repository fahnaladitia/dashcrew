package adapters

import (
	"fmt"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/agents"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/projectfiles"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// registered is what sync needs to know of one agent before and beside its adapter.
type registered struct {
	// adapter builds the agent's adapter for a project; k adds the knowledge MCP server when set.
	adapter func(c *catalog.Catalog, projectDir, platform string, k *profile.Knowledge) Adapter
	// skillRoot is the folder holding `<name>/SKILL.md`, relative to the project.
	skillRoot string
	// guardMinimum is the oldest dashcrew whose guard answers this agent's hook correctly.
	guardMinimum string
	// skillText is a skill's SKILL.md in the agent's format.
	skillText func(name string, meta catalog.DocMeta, body string) string
	// hookFile holds the agent's guard hook, and mergeGuardHook writes the guard entry there, keeping the rest.
	hookFile       string
	mergeGuardHook func(project projectfiles.Project) (bool, error)
}

// registry holds every agent of internal/agents; a test fails for one missing here.
var registry = map[string]registered{
	"claude-code": {
		adapter: func(c *catalog.Catalog, projectDir, platform string, k *profile.Knowledge) Adapter {
			a := NewClaudeCode(c, projectDir, platform)
			a.Knowledge = k
			return a
		},
		skillRoot:      ".claude/skills",
		guardMinimum:   version.GuardMinimum,
		skillText:      ClaudeSkillFile,
		hookFile:       claudeHookFile,
		mergeGuardHook: mergeClaudeGuardHook,
	},
	"codex": {
		adapter: func(c *catalog.Catalog, projectDir, platform string, k *profile.Knowledge) Adapter {
			a := NewCodex(c, projectDir, platform)
			a.Knowledge = k
			return a
		},
		skillRoot:      ".agents/skills",
		guardMinimum:   version.GuardMinimum,
		skillText:      AgentsSkillFile,
		hookFile:       codexHookFile,
		mergeGuardHook: mergeCodexGuardHook,
	},
	"antigravity": {
		adapter: func(c *catalog.Catalog, projectDir, platform string, k *profile.Knowledge) Adapter {
			a := NewAntigravity(c, projectDir, platform)
			a.Knowledge = k
			return a
		},
		skillRoot: ".agents/skills",
		// agy fails closed on a hook that answers nothing, so it needs a guard that answers allow.
		guardMinimum:   version.AntigravityGuardMinimum,
		skillText:      AgentsSkillFile,
		hookFile:       antigravityHookFile,
		mergeGuardHook: mergeAntigravityGuardHook,
	},
}

// AdapterFor returns the adapter for agent on platform; a non-nil k adds the knowledge MCP server.
func AdapterFor(agent string, c *catalog.Catalog, projectDir, platform string, k *profile.Knowledge) (Adapter, error) {
	if r, ok := registry[agent]; ok {
		return r.adapter(c, projectDir, platform, k), nil
	}
	return nil, Unsupported(agent)
}

// Unsupported is the error for an agent dashcrew writes no files for.
func Unsupported(agent string) error {
	return fmt.Errorf("Agent adapter not available yet: %s. Supported: %s.", agent, strings.Join(agents.All(), ", "))
}

// SkillFile is where agent reads skill name, relative to the project: Claude Code's own folder, or the
// `.agents/skills` folder the other agents share, which an agent not registered gets too.
func SkillFile(agent, name string) string {
	root := ".agents/skills"
	if r, ok := registry[agent]; ok {
		root = r.skillRoot
	}
	return root + "/" + name + "/SKILL.md"
}

// GuardMinimum is the oldest dashcrew whose guard answers agent's hook correctly; version.GuardMinimum for an
// agent not registered.
func GuardMinimum(agent string) string {
	if r, ok := registry[agent]; ok {
		return r.guardMinimum
	}
	return version.GuardMinimum
}

// HookFile is the file that holds agent's guard hook, relative to the project; empty for an agent not registered.
func HookFile(agent string) string {
	return registry[agent].hookFile
}

// MergeGuardHook writes agent's guard hook entry into its hook file in projectDir, as a sync's Merge does, and
// keeps every other entry. It returns the file, relative to the project, and whether it changed.
func MergeGuardHook(agent, projectDir string) (string, bool, error) {
	r, ok := registry[agent]
	if !ok {
		return "", false, Unsupported(agent)
	}
	changed, err := r.mergeGuardHook(projectfiles.New(projectDir))
	return r.hookFile, changed, err
}

// SkillText is skill name's SKILL.md in agent's format: Claude Code's own front matter, or the `.agents/skills`
// format, which an agent not registered gets too.
func SkillText(agent, name string, meta catalog.DocMeta, body string) string {
	if r, ok := registry[agent]; ok {
		return r.skillText(name, meta, body)
	}
	return AgentsSkillFile(name, meta, body)
}
