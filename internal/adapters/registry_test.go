package adapters

import (
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/agents"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/guard"
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

// TestGuardHookIsWhatMergeWrites: `dashcrew guard init` writes each agent's hook with MergeGuardHook, which must
// give the same file as a sync's Merge and keep the user's own entries.
func TestGuardHookIsWhatMergeWrites(t *testing.T) {
	c := shipped(t)
	user := map[string]string{
		".claude/settings.json": `{"hooks":{"PreToolUse":[{"matcher":"Read","hooks":[{"type":"command","command":"mine"}]}]},"model":"x"}`,
		".codex/hooks.json":     `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"mine"}]}]}}`,
		".agents/hooks.json":    `{"mine":{"PreToolUse":[]}}`,
	}
	for _, agent := range agents.All() {
		viaInit, viaMerge := t.TempDir(), t.TempDir()
		path, changed, err := MergeGuardHook(agent, viaInit)
		if err != nil || !changed {
			t.Fatalf("%s: MergeGuardHook = %q, %v, %v", agent, path, changed, err)
		}
		adapter, _ := AdapterFor(agent, c, viaMerge, "flutter", nil)
		if _, err := adapter.Merge(nil, nil); err != nil {
			t.Fatal(err)
		}
		if got, want := readFile(t, viaInit, path), readFile(t, viaMerge, path); got != want {
			t.Errorf("%s: %s from guard init:\n%s\nfrom sync:\n%s", agent, path, got, want)
		}
		if _, changed, _ := MergeGuardHook(agent, viaInit); changed {
			t.Errorf("%s: a second MergeGuardHook changed %s", agent, path)
		}

		mine := t.TempDir()
		writeFile(t, mine, path, user[path])
		if _, _, err := MergeGuardHook(agent, mine); err != nil {
			t.Fatal(err)
		}
		got := readFile(t, mine, path)
		if !strings.Contains(got, `"mine"`) || !strings.Contains(got, ".ai/bin/guard") {
			t.Errorf("%s: %s lost the user's entry or lacks the guard:\n%s", agent, path, got)
		}
	}
	if _, _, err := MergeGuardHook("cursor", t.TempDir()); err == nil {
		t.Error("MergeGuardHook accepted an agent dashcrew does not support")
	}
}

// TestClaudeHookNamesClaudeCode: the guard answers ask only to the agent its argument names, so the Claude Code
// hook must pass the argument the guard reads.
func TestClaudeHookNamesClaudeCode(t *testing.T) {
	if !strings.HasSuffix(ClaudeGuardCommand, " "+guard.ClaudeCodeArgument) {
		t.Errorf("ClaudeGuardCommand %q does not end with %q", ClaudeGuardCommand, guard.ClaudeCodeArgument)
	}
	for _, command := range []string{GitGuardCommand, AntigravityGuardCommand} {
		if strings.Contains(command, guard.ClaudeCodeArgument) {
			t.Errorf("another agent's hook names Claude Code: %q", command)
		}
	}
}
