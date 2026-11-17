package doctor

import "github.com/fahnaladitia/dashcrew/internal/omap"

// agentCheck is what doctor checks for one agent of the profile; agentChecks holds one per agent of
// internal/agents, and a test fails for one missing.
type agentCheck struct {
	// hook checks the guard hook from the committed files only; `dashcrew check` runs it in CI too.
	hook func(Doctor) Check
	// machine are the checks after hook that read this machine or ask the agent, in report order.
	machine func(d Doctor, lockedFiles *omap.Map[string]) []Check
	// alwaysOn adds, through add, the files the agent loads from the project at session start, and returns the
	// imports outside the project it does not count.
	alwaysOn func(d Doctor, add func(relative string) bool) (outside []string)
	// agentsMdLimit is the AGENTS.md size the agent reads in full.
	agentsMdLimit int
}

var agentChecks = map[string]agentCheck{
	"claude-code": {
		hook: Doctor.hook,
		machine: func(d Doctor, lockedFiles *omap.Map[string]) []Check {
			return []Check{d.claudeMd(), d.mcp(), d.skills(lockedFiles)}
		},
		alwaysOn:      claudeAlwaysOn,
		agentsMdLimit: AgentsMdLimit,
	},
	"codex": {
		hook: Doctor.codexHook,
		machine: func(d Doctor, _ *omap.Map[string]) []Check {
			return []Check{d.codexMcp(), d.codexSkillsBudget(), d.codexTrust()}
		},
		alwaysOn:      codexAlwaysOn,
		agentsMdLimit: AgentsMdLimit,
	},
	"antigravity": {
		hook: Doctor.antigravityHook,
		machine: func(d Doctor, _ *omap.Map[string]) []Check {
			return []Check{d.antigravityMcp(), d.antigravityRoles()}
		},
		alwaysOn: antigravityAlwaysOn,
		// Antigravity cuts every rule file at 24,000 bytes.
		agentsMdLimit: 24000,
	},
}
