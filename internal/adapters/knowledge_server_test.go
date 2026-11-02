package adapters

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

var notesKnowledge = &profile.Knowledge{Repo: "git@example.com:team/notes.git", Name: "notes"}

func TestEveryAdapterRegistersTheKnowledgeServerAndDropsItWithKnowledge(t *testing.T) {
	cases := []struct {
		agent, path, entry string
	}{
		{"claude-code", ".mcp.json", `{"type":"stdio","command":"dashcrew","args":["knowledge","serve","--clone","~/.dashcrew/knowledge/notes"]}`},
		{"antigravity", ".agents/mcp_config.json", `{"command":"dashcrew","args":["knowledge","serve","--clone","~/.dashcrew/knowledge/notes"]}`},
	}
	for _, c := range cases {
		t.Run(c.agent, func(t *testing.T) {
			project := t.TempDir()
			writeFile(t, project, c.path, `{"mcpServers":{"graph":{"command":"graph"}}}`)
			with, err := AdapterFor(c.agent, shipped(t), project, "flutter", notesKnowledge)
			if err != nil {
				t.Fatal(err)
			}
			first, err := with.Merge(map[string]bool{}, nil)
			if err != nil {
				t.Fatal(err)
			}
			servers := decodeMap(t, readFile(t, project, c.path)).Value("mcpServers").(*omap.Map[any])
			if got := compact(t, servers.Value("dashcrew-knowledge")); got != c.entry {
				t.Errorf("entry %s", got)
			}
			if !reflect.DeepEqual(servers.Keys(), []string{"graph", "dart-mcp-server", "dashcrew-knowledge"}) {
				t.Errorf("servers %v", servers.Keys())
			}
			if !reflect.DeepEqual(first.OwnedMcp, map[string]bool{"dart-mcp-server": true, "dashcrew-knowledge": true}) {
				t.Errorf("owned %v", first.OwnedMcp)
			}

			without, err := AdapterFor(c.agent, shipped(t), project, "flutter", nil)
			if err != nil {
				t.Fatal(err)
			}
			second, err := without.Merge(first.OwnedMcp, nil)
			if err != nil {
				t.Fatal(err)
			}
			servers = decodeMap(t, readFile(t, project, c.path)).Value("mcpServers").(*omap.Map[any])
			if !reflect.DeepEqual(servers.Keys(), []string{"graph", "dart-mcp-server"}) || !contains(second.Written, c.path) {
				t.Errorf("after dropping knowledge: servers %v written %v", servers.Keys(), second.Written)
			}
			if !reflect.DeepEqual(second.OwnedMcp, map[string]bool{"dart-mcp-server": true}) {
				t.Errorf("owned %v", second.OwnedMcp)
			}
		})
	}
}

func TestAServerTheUserNamedLikeTheKnowledgeServerIsNeverRemoved(t *testing.T) {
	project := t.TempDir()
	writeFile(t, project, ".mcp.json", `{"mcpServers":{"dashcrew-knowledge":{"command":"mine"}}}`)
	result, err := NewClaudeCode(shipped(t), project, "flutter").Merge(map[string]bool{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readFile(t, project, ".mcp.json"), `"mine"`) || result.OwnedMcp["dashcrew-knowledge"] {
		t.Errorf("mcp %s owned %v", readFile(t, project, ".mcp.json"), result.OwnedMcp)
	}
}

func TestCodexBlockGainsAndLosesTheKnowledgeServer(t *testing.T) {
	project := t.TempDir()
	codex := NewCodex(shipped(t), project, "flutter")
	codex.Knowledge = notesKnowledge
	result, err := codex.Merge(map[string]bool{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(codexBlock, "# dashcrew:end\n", "[mcp_servers.dashcrew-knowledge]\ncommand = \"dashcrew\"\n"+
		"args = [\"knowledge\",\"serve\",\"--clone\",\"~/.dashcrew/knowledge/notes\"]\n# dashcrew:end\n", 1)
	if got := readFile(t, project, ".codex/config.toml"); got != want {
		t.Errorf("config %q", got)
	}
	if !reflect.DeepEqual(result.OwnedMcp, map[string]bool{"dart-mcp-server": true, "dashcrew-knowledge": true}) {
		t.Errorf("owned %v", result.OwnedMcp)
	}
	if got := codex.McpServerNames(); !reflect.DeepEqual(got, []string{"dart-mcp-server", "dashcrew-knowledge"}) {
		t.Errorf("names %v", got)
	}

	codex.Knowledge = nil
	if _, err := codex.Merge(result.OwnedMcp, nil); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, project, ".codex/config.toml"); got != codexBlock {
		t.Errorf("config after dropping knowledge %q", got)
	}
}
