package adapters

import (
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/agents"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// TestEveryAgentIsRegistered fails when an agent joins internal/agents without its adapter, skill folder and guard
// minimum here.
func TestEveryAgentIsRegistered(t *testing.T) {
	c := shipped(t)
	for _, agent := range agents.All() {
		adapter, err := AdapterFor(agent, c, t.TempDir(), "flutter", nil)
		if err != nil || adapter.Agent() != agent {
			t.Errorf("%s: adapter %v", agent, err)
		}
		if SkillFile(agent, "x") == "" {
			t.Errorf("%s: no skill folder", agent)
		}
		if !version.IsSemantic(GuardMinimum(agent)) {
			t.Errorf("%s: guard minimum %q", agent, GuardMinimum(agent))
		}
	}
}

func TestSkillFoldersAndGuardMinimums(t *testing.T) {
	for agent, want := range map[string][2]string{
		"claude-code": {".claude/skills/x/SKILL.md", version.GuardMinimum},
		"codex":       {".agents/skills/x/SKILL.md", version.GuardMinimum},
		"antigravity": {".agents/skills/x/SKILL.md", version.AntigravityGuardMinimum},
	} {
		if got := SkillFile(agent, "x"); got != want[0] {
			t.Errorf("%s: skill file %q, want %q", agent, got, want[0])
		}
		if got := GuardMinimum(agent); got != want[1] {
			t.Errorf("%s: guard minimum %q, want %q", agent, got, want[1])
		}
	}
}

func TestSkillTextIsEachAgentsFormat(t *testing.T) {
	meta := catalog.DocMeta{Description: "d"}
	if got, want := SkillText("claude-code", "x", meta, "b"), ClaudeSkillFile("x", meta, "b"); got != want {
		t.Errorf("claude-code: %q, want %q", got, want)
	}
	for _, agent := range []string{"codex", "antigravity"} {
		if got, want := SkillText(agent, "x", meta, "b"), AgentsSkillFile("x", meta, "b"); got != want {
			t.Errorf("%s: %q, want %q", agent, got, want)
		}
	}
}
