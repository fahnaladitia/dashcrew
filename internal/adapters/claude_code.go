package adapters

import (
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/projectfiles"
	"github.com/fahnaladitia/dashcrew/internal/render"
)

const (
	// ClaudeGuardMatcher is the PreToolUse matcher of the guard entry.
	ClaudeGuardMatcher = "Bash|Write|Edit|MultiEdit|NotebookEdit"
	// ClaudeGuardCommand runs the guard from any working directory through CLAUDE_PROJECT_DIR.
	ClaudeGuardCommand = `"$CLAUDE_PROJECT_DIR/.ai/bin/guard" "$CLAUDE_PROJECT_DIR/.ai/guard.json"`
)

// ClaudeGuardHook is the PreToolUse entry sync keeps in `.claude/settings.json`.
func ClaudeGuardHook() *omap.Map[any] {
	return omap.Of[any]("matcher", ClaudeGuardMatcher, "hooks", []any{omap.Of[any]("type", "command", "command", ClaudeGuardCommand)})
}

var (
	claudeTools = map[string]string{
		"read":        "Read, Grep, Glob, Bash",
		"write-plans": "Read, Write, Grep, Glob, Bash",
		"write-code":  "Read, Write, Edit, Grep, Glob, Bash",
	}
	claudeModels = map[string]string{"high": "opus", "standard": "sonnet"}
)

// ClaudeCode writes `.claude/agents/<role>.md`, `.claude/skills/<name>/SKILL.md` (skills and workflows) and
// `CLAUDE.md`, and merges `.claude/settings.json` and `.mcp.json`.
type ClaudeCode struct {
	catalog  *catalog.Catalog
	project  projectfiles.Project
	platform string
}

// NewClaudeCode returns the adapter; platform's `mcp_servers` go to `.mcp.json`.
func NewClaudeCode(c *catalog.Catalog, projectDir, platform string) *ClaudeCode {
	return &ClaudeCode{catalog: c, project: projectfiles.New(projectDir), platform: platform}
}

// Agent is `claude-code`.
func (a *ClaudeCode) Agent() string { return "claude-code" }

// McpServers are the platform's servers as `.mcp.json` entries.
func (a *ClaudeCode) McpServers() *omap.Map[any] {
	out := omap.New[any]()
	servers := platformServers(a.catalog, a.platform)
	for _, name := range servers.Keys() {
		server := servers.Value(name)
		out.Set(name, omap.Of[any]("type", "stdio", "command", server.Command, "args", stringList(server.Args)))
	}
	return out
}

// Files writes CLAUDE.md only when it is missing or dashcrew wrote it; another CLAUDE.md that does not import
// AGENTS.md is reported in warnings.
func (a *ClaudeCode) Files(p *profile.Profile, targets []render.RenderedTarget, context map[string]any, lockedFiles map[string]string, warnings *[]string) (*omap.Map[string], error) {
	files := omap.New[string]()
	rolesIn(a.catalog, targets, context, func(role catalog.RoleMeta, body string) {
		files.Set(".claude/agents/"+role.Name+".md", claudeAgent(role, body))
	})
	docs, err := SkillDocs(a.catalog, p, targets, warnings)
	if err != nil {
		return nil, err
	}
	for _, doc := range docs {
		files.Set(".claude/skills/"+doc.Name+"/SKILL.md", claudeSkill(doc.Name, doc.Meta, doc.Body))
	}

	_, locked := lockedFiles["CLAUDE.md"]
	if !a.project.Exists("CLAUDE.md") || locked {
		files.Set("CLAUDE.md", "@AGENTS.md\n")
	} else {
		current, err := a.project.Read("CLAUDE.md")
		if err != nil {
			return nil, err
		}
		if !strings.Contains(current, "@AGENTS.md") {
			warn(warnings, `CLAUDE.md does not import AGENTS.md; add a line "@AGENTS.md" so Claude Code loads the rules.`)
		}
	}
	return files, nil
}

// Merge keeps one guard entry in `.claude/settings.json` and the platform servers in `.mcp.json`.
func (a *ClaudeCode) Merge(ownedMcp map[string]bool, warnings *[]string) (MergeResult, error) {
	result := MergeResult{}
	settings, err := readJSONObject(a.project, ".claude/settings.json")
	if err != nil {
		return result, err
	}
	if err := replaceGuardEntry(settings, ClaudeGuardHook(), ".claude/settings.json"); err != nil {
		return result, err
	}
	changed, err := writeJSON(a.project, ".claude/settings.json", settings)
	if err != nil {
		return result, err
	}
	if changed {
		result.Written = append(result.Written, ".claude/settings.json")
	}
	changed, owned, err := MergeJSONMcpServers(a.project, ".mcp.json", a.McpServers(), ownedMcp, warnings)
	if err != nil {
		return result, err
	}
	if changed {
		result.Written = append(result.Written, ".mcp.json")
	}
	result.OwnedMcp = owned
	return result, nil
}

func claudeAgent(role catalog.RoleMeta, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + role.Name + "\n")
	b.WriteString("description: " + jsonString(role.Description) + "\n")
	b.WriteString("tools: " + claudeTools[role.Capability] + "\n")
	if role.Model != nil {
		if model, ok := claudeModels[*role.Model]; ok {
			b.WriteString("model: " + model + "\n")
		}
	}
	if role.Effort != nil {
		b.WriteString("effort: " + *role.Effort + "\n")
	}
	b.WriteString("---\n")
	return b.String() + "\n" + body
}

func claudeSkill(name string, meta catalog.DocMeta, body string) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: " + name + "\n")
	b.WriteString("description: " + jsonString(meta.Description) + "\n")
	if meta.WhenToUse != nil {
		b.WriteString("when_to_use: " + jsonString(*meta.WhenToUse) + "\n")
	}
	if meta.ArgumentHint != nil {
		b.WriteString("argument-hint: " + jsonString(*meta.ArgumentHint) + "\n")
	}
	b.WriteString("---\n")
	return b.String() + "\n" + body
}
