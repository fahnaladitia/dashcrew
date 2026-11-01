package adapters

import (
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/projectfiles"
	"github.com/fahnaladitia/dashcrew/internal/render"
)

// AntigravityGuardKey is the key dashcrew owns in `.agents/hooks.json`; every other key belongs to someone else.
const AntigravityGuardKey = "dashcrew-guard"

// AntigravityKnownTools are the tool names of the Antigravity hooks reference; an unknown name may hang a subagent.
var AntigravityKnownTools = []string{
	"view_file", "write_to_file", "replace_file_content", "multi_replace_file_content", "list_dir", "find_by_name",
	"grep_search", "search_web", "read_url_content", "run_command", "manage_task", "schedule", "list_permissions",
	"ask_permission", "invoke_subagent", "define_subagent", "send_message", "manage_subagents", "ask_question",
	"generate_image",
}

var antigravityReadTools = []string{"view_file", "list_dir", "find_by_name", "grep_search", "run_command"}

// AntigravityToolsByCapability is the enforced `tools` allowlist per role capability; `write-plans` cannot be
// limited to the plans folder here, so that limit stays an instruction.
var AntigravityToolsByCapability = map[string][]string{
	"read":        antigravityReadTools,
	"write-plans": append(append([]string{}, antigravityReadTools...), "write_to_file"),
	"write-code":  append(append([]string{}, antigravityReadTools...), "write_to_file", "replace_file_content", "multi_replace_file_content"),
}

// AntigravityGuardHook is the value under AntigravityGuardKey: hooks name the tool in `toolCall.name`, and the
// command is a plain relative path because Antigravity documents no shell or working directory for it.
func AntigravityGuardHook() *omap.Map[any] {
	return omap.Of[any]("PreToolUse", []any{omap.Of[any](
		"matcher", `^(run_command|write_to_file|replace_file_content|multi_replace_file_content)$`,
		"hooks", []any{omap.Of[any]("type", "command", "command", "./.ai/bin/guard", "timeout", 30)},
	)})
}

// Antigravity writes `.agents/skills/<name>/SKILL.md` (shared with Codex) and `.agents/agents/<role>.md`, and
// merges `.agents/hooks.json` and `.agents/mcp_config.json`. Antigravity cuts every rule file at 24,000 bytes.
type Antigravity struct {
	catalog  *catalog.Catalog
	project  projectfiles.Project
	platform string
}

// NewAntigravity returns the adapter; platform's `mcp_servers` go to `.agents/mcp_config.json`.
func NewAntigravity(c *catalog.Catalog, projectDir, platform string) *Antigravity {
	return &Antigravity{catalog: c, project: projectfiles.New(projectDir), platform: platform}
}

// Agent is `antigravity`.
func (a *Antigravity) Agent() string { return "antigravity" }

// McpServers are the platform's servers as `.agents/mcp_config.json` entries.
func (a *Antigravity) McpServers() *omap.Map[any] {
	out := omap.New[any]()
	servers := platformServers(a.catalog, a.platform)
	for _, name := range servers.Keys() {
		server := servers.Value(name)
		out.Set(name, omap.Of[any]("command", server.Command, "args", stringList(server.Args)))
	}
	return out
}

// Files are the subagent files and the shared skill files.
func (a *Antigravity) Files(p *profile.Profile, targets []render.RenderedTarget, context map[string]any, lockedFiles map[string]string, warnings *[]string) (*omap.Map[string], error) {
	files := omap.New[string]()
	rolesIn(a.catalog, targets, context, func(role catalog.RoleMeta, body string) {
		files.Set(".agents/agents/"+role.Name+".md", antigravityRole(role, body))
	})
	docs, err := SkillDocs(a.catalog, p, targets, warnings)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		files.Set(".agents/skills/"+doc.Name+"/SKILL.md", AgentsSkillFile(doc.Name, doc.Meta, doc.Body))
	}
	return files, nil
}

// Merge sets AntigravityGuardKey in `.agents/hooks.json` and the platform servers in `.agents/mcp_config.json`.
func (a *Antigravity) Merge(ownedMcp map[string]bool, warnings *[]string) (MergeResult, error) {
	result := MergeResult{}
	config, err := readJSONObject(a.project, ".agents/hooks.json")
	if err != nil {
		return result, err
	}
	config.Set(AntigravityGuardKey, AntigravityGuardHook())
	changed, err := writeJSON(a.project, ".agents/hooks.json", config)
	if err != nil {
		return result, err
	}
	if changed {
		result.Written = append(result.Written, ".agents/hooks.json")
	}
	changed, owned, err := MergeJSONMcpServers(a.project, ".agents/mcp_config.json", a.McpServers(), ownedMcp, warnings)
	if err != nil {
		return result, err
	}
	if changed {
		result.Written = append(result.Written, ".agents/mcp_config.json")
	}
	result.OwnedMcp = owned
	return result, nil
}

func antigravityRole(role catalog.RoleMeta, body string) string {
	return "---\n" +
		"name: " + role.Name + "\n" +
		"description: " + jsonString(role.Description) + "\n" +
		"model: inherit\n" +
		"tools: [" + strings.Join(AntigravityToolsByCapability[role.Capability], ", ") + "]\n" +
		"---\n\n" +
		body
}
