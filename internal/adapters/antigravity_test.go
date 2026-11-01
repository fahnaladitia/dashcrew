package adapters

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

func TestAntigravitySkillsMatchTheCodexFiles(t *testing.T) {
	c := shipped(t)
	const yaml = "toolkit: 1\nagents: [antigravity]\nstack: { state: bloc }\n"
	codex := files(t, c, NewCodex(c, t.TempDir(), "flutter"), yaml)

	antigravity := files(t, c, NewAntigravity(c, t.TempDir(), "flutter"), yaml)

	for _, path := range keysWithPrefix(antigravity, ".agents/skills/") {
		if antigravity[path] != codex[path] {
			t.Errorf("%s differs", path)
		}
	}
	if _, ok := antigravity[".agents/skills/bloc-patterns/SKILL.md"]; !ok {
		t.Error("bloc-patterns")
	}
}

func TestAntigravitySubagentsHaveTheReadToolsAllowlist(t *testing.T) {
	c := shipped(t)
	out := files(t, c, NewAntigravity(c, t.TempDir(), "flutter"), "toolkit: 1\nagents: [antigravity]\n")

	reviewer := out[".agents/agents/reviewer.md"]
	if !strings.HasPrefix(reviewer, "---\nname: reviewer\ndescription: \"") ||
		!strings.Contains(reviewer, "\nmodel: inherit\ntools: [view_file, list_dir, find_by_name, grep_search, run_command]\n---\n\n") {
		t.Errorf("reviewer %q", reviewer[:200])
	}
	if !strings.Contains(out[".agents/agents/explorer.md"], "tools: [view_file, list_dir, find_by_name, grep_search, run_command]\n") {
		t.Error("explorer tools")
	}
	agents := keysWithPrefix(out, ".agents/agents/")
	sort.Strings(agents)
	if !reflect.DeepEqual(agents, []string{".agents/agents/explorer.md", ".agents/agents/reviewer.md"}) {
		t.Errorf("agents %v", agents)
	}
	if want := append(append([]string{}, AntigravityToolsByCapability["read"]...), "write_to_file"); !reflect.DeepEqual(AntigravityToolsByCapability["write-plans"], want) {
		t.Errorf("write-plans %v", AntigravityToolsByCapability["write-plans"])
	}
	for _, tool := range []string{"replace_file_content", "multi_replace_file_content"} {
		if !contains(AntigravityToolsByCapability["write-code"], tool) {
			t.Errorf("write-code lacks %s", tool)
		}
	}
}

func TestEveryAllowlistedToolIsADocumentedAntigravityTool(t *testing.T) {
	for capability, tools := range AntigravityToolsByCapability {
		for _, tool := range tools {
			if !contains(AntigravityKnownTools, tool) {
				t.Errorf("%s: %s", capability, tool)
			}
		}
	}
	var capabilities []string
	for capability := range AntigravityToolsByCapability {
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	want := append([]string{}, catalog.Capabilities...)
	sort.Strings(want)
	if !reflect.DeepEqual(capabilities, want) {
		t.Errorf("capabilities %v", capabilities)
	}
}

func TestAntigravityGuardHookKeepsItsOwnKey(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".agents/hooks.json", `{"my-linter-hook":{"PostToolUse":[{"matcher":"","hooks":[{"type":"command","command":"./lint.sh"}]}]}}`)
	merge := func() []string {
		result, err := NewAntigravity(shipped(t), project, "flutter").Merge(map[string]bool{}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return result.Written
	}

	if !contains(merge(), ".agents/hooks.json") || contains(merge(), ".agents/hooks.json") {
		t.Error("hooks.json not written exactly once")
	}

	config := decodeMap(t, readFile(t, project, ".agents/hooks.json"))
	if !reflect.DeepEqual(config.Keys(), []string{"my-linter-hook", "dashcrew-guard"}) {
		t.Errorf("keys %v", config.Keys())
	}
	if compact(t, config.Value("dashcrew-guard")) != compact(t, AntigravityGuardHook()) {
		t.Errorf("guard %s", compact(t, config.Value("dashcrew-guard")))
	}
}

func TestAntigravityMcpServerIsAddedNextToUserServers(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".agents/mcp_config.json", `{"mcpServers":{"graph":{"command":"graph"}}}`)
	adapter := NewAntigravity(shipped(t), project, "flutter")

	result, err := adapter.Merge(map[string]bool{}, nil)
	if err != nil {
		t.Fatal(err)
	}

	servers := decodeMap(t, readFile(t, project, ".agents/mcp_config.json")).Value("mcpServers").(*omap.Map[any])
	if compact(t, servers.Value("graph")) != `{"command":"graph"}` || compact(t, servers.Value("dart-mcp-server")) != `{"command":"dart","args":["mcp-server"]}` {
		t.Errorf("servers %s", compact(t, servers))
	}
	if !reflect.DeepEqual(result.OwnedMcp, map[string]bool{"dart-mcp-server": true}) {
		t.Errorf("owned %v", result.OwnedMcp)
	}
	again, err := adapter.Merge(result.OwnedMcp, nil)
	if err != nil || contains(again.Written, ".agents/mcp_config.json") {
		t.Errorf("second merge %v %v", again.Written, err)
	}
}

func TestAntigravityServerTheUserDefinedDifferentlyIsLeft(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".agents/mcp_config.json", `{"mcpServers":{"dart-mcp-server":{"command":"fvm"}}}`)
	var warnings []string

	result, err := NewAntigravity(shipped(t), project, "flutter").Merge(map[string]bool{}, &warnings)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.OwnedMcp) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0], ".agents/mcp_config.json") {
		t.Errorf("owned %v warnings %v", result.OwnedMcp, warnings)
	}
	if !strings.Contains(readFile(t, project, ".agents/mcp_config.json"), `"fvm"`) {
		t.Error("user server changed")
	}
}
