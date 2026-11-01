package adapters

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

const claudeProfile = `
toolkit: 1
agents: [claude-code]
stack:
  state: bloc
  di: get_it
  analytics: posthog
project: { local_packages: [core] }
`

func TestClaudeCodeWritesRolesSkillsAndClaudeMd(t *testing.T) {
	project := t.TempDir()
	c := shipped(t)

	out := files(t, c, NewClaudeCode(c, project, "flutter"), claudeProfile)

	if out["CLAUDE.md"] != "@AGENTS.md\n" {
		t.Errorf("CLAUDE.md %q", out["CLAUDE.md"])
	}
	reviewer := out[".claude/agents/reviewer.md"]
	if !strings.HasPrefix(reviewer, "---\nname: reviewer\n") || !strings.Contains(reviewer, "tools: Read, Grep, Glob, Bash\nmodel: opus\neffort: high\n") {
		t.Errorf("reviewer %q", reviewer[:200])
	}
	if !strings.Contains(out[".claude/agents/explorer.md"], "tools: Read, Grep, Glob, Bash\nmodel: sonnet\neffort: medium\n") {
		t.Error("explorer front matter")
	}
	if !strings.HasPrefix(out[".claude/skills/plan/SKILL.md"], "---\nname: plan\ndescription: ") {
		t.Error("plan skill")
	}
	if !strings.Contains(out[".claude/skills/bloc-patterns/SKILL.md"], "name: bloc-patterns") {
		t.Error("bloc-patterns skill")
	}
	agents := keysWithPrefix(out, ".claude/agents/")
	sort.Strings(agents)
	if !reflect.DeepEqual(agents, []string{".claude/agents/explorer.md", ".claude/agents/reviewer.md"}) {
		t.Errorf("agents %v", agents)
	}
}

func TestClaudeCodeKeepsAClaudeMdThatDoesNotImportAgentsMd(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, "CLAUDE.md", "Personal notes.\n")
	c := shipped(t)
	p, targets, context := renderFor(t, c, claudeProfile)
	var warnings []string

	out, err := NewClaudeCode(c, project, "flutter").Files(p, targets, context, map[string]string{}, &warnings)
	if err != nil {
		t.Fatal(err)
	}

	if out.Has("CLAUDE.md") {
		t.Error("CLAUDE.md generated")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "CLAUDE.md") {
		t.Errorf("warnings %v", warnings)
	}

	locked, err := NewClaudeCode(c, project, "flutter").Files(p, targets, context, map[string]string{"CLAUDE.md": "hash"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if locked.Value("CLAUDE.md") != "@AGENTS.md\n" {
		t.Error("a locked CLAUDE.md is not regenerated")
	}
}

func TestClaudeCodeMergeAddsTheGuardAndTheMcpServer(t *testing.T) {
	project := t.TempDir()
	adapter := NewClaudeCode(shipped(t), project, "flutter")
	var warnings []string

	result, err := adapter.Merge(map[string]bool{}, &warnings)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(result.Written, []string{".claude/settings.json", ".mcp.json"}) {
		t.Errorf("written %v", result.Written)
	}
	settings := readFile(t, project, ".claude/settings.json")
	if strings.Count(settings, ".ai/bin/guard") != 1 {
		t.Errorf("settings %s", settings)
	}
	if mcp := decodeMap(t, readFile(t, project, ".mcp.json")); !mcp.Value("mcpServers").(*omap.Map[any]).Has("dart-mcp-server") {
		t.Error("no dart-mcp-server")
	}
	if !result.OwnedMcp["dart-mcp-server"] {
		t.Errorf("owned %v", result.OwnedMcp)
	}
	again, err := adapter.Merge(result.OwnedMcp, &warnings)
	if err != nil || len(again.Written) != 0 {
		t.Errorf("second merge wrote %v (%v)", again.Written, err)
	}
}

func TestClaudeCodeMergePreservesForeignHooksAndServers(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".claude/settings.json", `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/usr/local/bin/other-hook"}]}],"Stop":[{"hooks":[{"type":"command","command":"notify"}]}]},"permissions":{"allow":["Bash(ls)"]}}`)
	writeFile(t, project, ".mcp.json", `{"mcpServers":{"graph":{"command":"graph","args":["serve"]}}}`)
	adapter := NewClaudeCode(shipped(t), project, "flutter")

	first, err := adapter.Merge(map[string]bool{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Merge(first.OwnedMcp, nil); err != nil {
		t.Fatal(err)
	}

	settings := readFile(t, project, ".claude/settings.json")
	for _, kept := range []string{"/usr/local/bin/other-hook", "notify", "Bash(ls)"} {
		if !strings.Contains(settings, kept) {
			t.Errorf("settings lost %s", kept)
		}
	}
	if strings.Count(settings, ".ai/bin/guard") != 1 {
		t.Errorf("guard entries: %d", strings.Count(settings, ".ai/bin/guard"))
	}
	if !strings.Contains(readFile(t, project, ".mcp.json"), `"graph"`) {
		t.Error("graph server lost")
	}
}

func TestClaudeCodeMergeReplacesAnOwnedServer(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".mcp.json", `{"mcpServers":{"dart-mcp-server":{"command":"old"}}}`)
	var warnings []string

	result, err := NewClaudeCode(shipped(t), project, "flutter").Merge(map[string]bool{"dart-mcp-server": true}, &warnings)
	if err != nil {
		t.Fatal(err)
	}

	if len(warnings) != 0 || strings.Contains(readFile(t, project, ".mcp.json"), `"old"`) || !result.OwnedMcp["dart-mcp-server"] {
		t.Errorf("warnings %v, mcp %s", warnings, readFile(t, project, ".mcp.json"))
	}
}

func TestClaudeCodeMergeLeavesAServerTheUserDefinedDifferently(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".mcp.json", `{"mcpServers":{"dart-mcp-server":{"command":"fvm"}}}`)
	var warnings []string

	result, err := NewClaudeCode(shipped(t), project, "flutter").Merge(map[string]bool{}, &warnings)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.OwnedMcp) != 0 || len(warnings) != 1 || warnings[0] != `.mcp.json already defines "dart-mcp-server" differently; left as is.` {
		t.Errorf("owned %v, warnings %v", result.OwnedMcp, warnings)
	}
}

func TestAdapterForNamesTheSupportedAgents(t *testing.T) {
	c := shipped(t)
	for _, agent := range []string{"claude-code", "codex", "antigravity"} {
		adapter, err := AdapterFor(agent, c, t.TempDir(), "flutter")
		if err != nil || adapter.Agent() != agent {
			t.Errorf("%s: %v", agent, err)
		}
	}
	_, err := AdapterFor("cursor", c, t.TempDir(), "flutter")
	if err == nil || err.Error() != "Agent adapter not available yet: cursor. Supported: claude-code, codex, antigravity." {
		t.Errorf("cursor: %v", err)
	}
}
