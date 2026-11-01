package adapters

import (
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/projectfiles"
	"github.com/fahnaladitia/dashcrew/internal/render"
)

const (
	// CodexGuardMatcher is the PreToolUse matcher of the guard entry.
	CodexGuardMatcher = `^(Bash|apply_patch|Edit|Write)$`
	// CodexGuardCommand finds the project root through git, since Codex runs hooks in the session's directory.
	CodexGuardCommand = `"$(git rev-parse --show-toplevel)/.ai/bin/guard" "$(git rev-parse --show-toplevel)/.ai/guard.json"`
	// SkillListBudget is the characters Codex gives the initial skill list when it does not know the context window.
	SkillListBudget = 8000

	codexBlockBegin = "# dashcrew:begin"
	codexBlockEnd   = "# dashcrew:end"
)

// CodexGuardHook is the PreToolUse entry sync keeps in `.codex/hooks.json`.
func CodexGuardHook() *omap.Map[any] {
	return omap.Of[any]("matcher", CodexGuardMatcher, "hooks", []any{omap.Of[any]("type", "command", "command", CodexGuardCommand, "timeout", 30)})
}

// Codex writes `.agents/skills/<name>/SKILL.md` and `.codex/agents/<role>.toml`, and merges the guard into
// `.codex/hooks.json` and MCP servers into a block of `.codex/config.toml`.
type Codex struct {
	catalog  *catalog.Catalog
	project  projectfiles.Project
	platform string
}

// NewCodex returns the adapter; platform's `mcp_servers` go to `.codex/config.toml`.
func NewCodex(c *catalog.Catalog, projectDir, platform string) *Codex {
	return &Codex{catalog: c, project: projectfiles.New(projectDir), platform: platform}
}

// Agent is `codex`.
func (a *Codex) Agent() string { return "codex" }

// McpServerNames are the platform's servers the dashcrew block of `.codex/config.toml` defines, in catalog order.
func (a *Codex) McpServerNames() []string { return platformServers(a.catalog, a.platform).Keys() }

// Files are the role TOML files and the shared skill files.
func (a *Codex) Files(p *profile.Profile, targets []render.RenderedTarget, context map[string]any, lockedFiles map[string]string, warnings *[]string) (*omap.Map[string], error) {
	files := omap.New[string]()
	rolesIn(a.catalog, targets, context, func(role catalog.RoleMeta, body string) {
		files.Set(".codex/agents/"+role.Name+".toml", codexRole(role, body))
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

// Merge keeps one guard entry in `.codex/hooks.json` and the dashcrew block in `.codex/config.toml`.
func (a *Codex) Merge(ownedMcp map[string]bool, warnings *[]string) (MergeResult, error) {
	result := MergeResult{}
	config, err := readJSONObject(a.project, ".codex/hooks.json")
	if err != nil {
		return result, err
	}
	if err := replaceGuardEntry(config, CodexGuardHook(), ".codex/hooks.json"); err != nil {
		return result, err
	}
	changed, err := writeJSON(a.project, ".codex/hooks.json", config)
	if err != nil {
		return result, err
	}
	if changed {
		result.Written = append(result.Written, ".codex/hooks.json")
	}
	changed, owned, err := a.mergeMcp(warnings)
	if err != nil {
		return result, err
	}
	if changed {
		result.Written = append(result.Written, ".codex/config.toml")
	}
	result.OwnedMcp = owned
	return result, nil
}

// lineChar is a character other than a line terminator, like `.` in a Dart regular expression.
const lineChar = `[^\n\r\x{2028}\x{2029}]`

var (
	blockBeginPattern = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(codexBlockBegin))
	blockEndPattern   = regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(codexBlockEnd) + lineChar + `*\n?`)
)

// mergeMcp rewrites only the text between `# dashcrew:begin` and `# dashcrew:end`, kept at the end of the file:
// the block holds tables only, and TOML needs top-level keys before the first table.
func (a *Codex) mergeMcp(warnings *[]string) (bool, map[string]bool, error) {
	const path = ".codex/config.toml"
	current := ""
	if a.project.Exists(path) {
		text, err := a.project.Read(path)
		if err != nil {
			return false, nil, err
		}
		current = text
	}

	before, after := current, ""
	begin := blockBeginPattern.FindStringIndex(current)
	var end []int
	if begin != nil {
		end = blockEndPattern.FindStringIndex(current[begin[0]:])
	}
	hasBlock := begin != nil && end != nil
	if hasBlock {
		before = current[:begin[0]]
		after = current[begin[0]+end[1]:]
	}

	owned := map[string]bool{}
	var tables strings.Builder
	servers := platformServers(a.catalog, a.platform)
	for _, name := range servers.Keys() {
		if DefinesMcpServer(before+"\n"+after, name) {
			warn(warnings, `.codex/config.toml already defines "`+name+`"; left as is.`)
			continue
		}
		server := servers.Value(name)
		owned[name] = true
		args, err := dartjson.Encode(stringList(server.Args))
		if err != nil {
			return false, nil, err
		}
		tables.WriteString("[mcp_servers." + name + "]\n")
		tables.WriteString("command = " + jsonString(server.Command) + "\n")
		tables.WriteString("args = " + args + "\n")
	}
	block := ""
	if len(owned) > 0 {
		block = codexBlockBegin + " - written by dashcrew sync; edit outside this block\n" + tables.String() + codexBlockEnd + "\n"
	}

	var next string
	switch {
	case hasBlock:
		next = before + block + after
	case block == "":
		next = current
	default:
		separator := "\n\n"
		if current == "" {
			separator = ""
		} else if strings.HasSuffix(current, "\n") {
			separator = "\n"
		}
		next = current + separator + block
	}
	if next == current {
		return false, owned, nil
	}
	changed, err := a.project.WriteIfChanged(path, next)
	return changed, owned, err
}

// jsSpace is `\s` of a Dart regular expression.
const jsSpace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// DefinesMcpServer is true when toml defines MCP server name as a table, dotted key or inline table.
func DefinesMcpServer(toml, name string) bool {
	key := `"?` + regexp.QuoteMeta(name) + `"?`
	s := jsSpace
	patterns := []string{
		`(?m)^` + s + `*\[` + s + `*mcp_servers` + s + `*\.` + s + `*` + key + s + `*\]`,
		`(?m)^` + s + `*mcp_servers` + s + `*\.` + s + `*` + key + s + `*[.=]`,
		`(?m)^` + s + `*` + key + s + `*=` + s + `*\{`,
	}
	for _, pattern := range patterns {
		if regexp.MustCompile(pattern).MatchString(toml) {
			return true
		}
	}
	return false
}

var (
	skillNameLine        = regexp.MustCompile(`(?m)^name: (` + lineChar + `*)$`)
	skillDescriptionLine = regexp.MustCompile(`(?m)^description: (` + lineChar + `*)$`)
)

// SkillListChars is the size of the skill list Codex builds from SKILL.md files: name plus description of each,
// counted in UTF-16 code units as Dart strings are.
func SkillListChars(skillFiles []string) int {
	total := 0
	for _, content := range skillFiles {
		name, description := "", ""
		if match := skillNameLine.FindStringSubmatch(content); match != nil {
			name = match[1]
		}
		if match := skillDescriptionLine.FindStringSubmatch(content); match != nil {
			description = match[1]
		}
		if strings.HasPrefix(description, `"`) {
			if decoded, err := dartjson.Decode(description); err == nil {
				if text, ok := decoded.(string); ok {
					description = text
				}
			}
		}
		total += utf16Length(name) + utf16Length(description)
	}
	return total
}

func utf16Length(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// codexRole is the file Codex reads from `.codex/agents/`; the role name comes from `name`, not the file name.
func codexRole(role catalog.RoleMeta, body string) string {
	var b strings.Builder
	b.WriteString("name = " + jsonString(role.Name) + "\n")
	b.WriteString("description = " + jsonString(role.Description) + "\n")
	if role.Effort != nil {
		b.WriteString("model_reasoning_effort = " + jsonString(*role.Effort) + "\n")
	}
	b.WriteString("developer_instructions = " + TomlMultiline(body) + "\n")
	return b.String()
}

// TomlMultiline is text as a TOML multi-line basic string; backslashes and `"""` are escaped.
func TomlMultiline(text string) string {
	escaped := strings.ReplaceAll(strings.ReplaceAll(text, `\`, `\\`), `"""`, `""\"`)
	return "\"\"\"\n" + escaped + "\"\"\""
}
