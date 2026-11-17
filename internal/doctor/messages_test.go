package doctor

import "testing"

// TestMcpChecksSayWhenThereIsNothingToDefine pins the detail of the three MCP checks on a platform with no MCP
// servers: it used to read "defined: " with nothing after it.
func TestMcpChecksSayWhenThereIsNothingToDefine(t *testing.T) {
	f := setup(t, "toolkit: 1\nplatform: other\nagents: [claude-code, codex, antigravity]\nplatform_config:\n"+
		"  detect: [pyproject.toml]\n  commands: { analyze: ruff check ., test: pytest, test_scoped: pytest, deps: pip install -e . }\n")
	f.sync()

	checks := f.doctor()

	for _, id := range []string{"mcp", "codex-mcp", "antigravity-mcp"} {
		expect(t, checks, id, OK)
		if got := checks[id].Detail; got != "no MCP servers to define" {
			t.Errorf("%s: %q", id, got)
		}
	}
}
