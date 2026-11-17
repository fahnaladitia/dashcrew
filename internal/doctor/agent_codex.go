package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
)

func (d Doctor) codexHook() Check {
	const id = "codex-hook"
	if !d.exists(".codex/hooks.json") {
		return Check{id, Fail, ".codex/hooks.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".codex/hooks.json")
	var entries []any
	if err == nil {
		entries, err = preToolUse(config)
	}
	if err != nil {
		return Check{id, Fail, ".codex/hooks.json invalid: " + err.Error()}
	}
	return guardEntryCheck(id, entries, encode(adapters.CodexGuardHook()), "PreToolUse guard hook present; Codex runs it once approved in /hooks")
}

func (d Doctor) codexMcp() Check {
	const id = "codex-mcp"
	toml := ""
	if d.exists(".codex/config.toml") {
		toml, _ = d.read(".codex/config.toml")
	}
	adapter := adapters.NewCodex(d.Catalog, d.ProjectDir, d.platform())
	adapter.Knowledge = d.profileKnowledge()
	expected := adapter.McpServerNames()
	var missing []string
	for _, name := range expected {
		if !adapters.DefinesMcpServer(toml, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Check{id, Warn, ".codex/config.toml does not define " + strings.Join(missing, ", ") + "; run dashcrew sync"}
	}
	detail := definedDetail(expected)
	if len(expected) > 0 && !strings.Contains(toml, "# dashcrew:begin") {
		detail += " (by the user, outside the dashcrew block)"
	}
	return Check{id, OK, detail}
}

func (d Doctor) codexSkillsBudget() Check {
	const id = "codex-skills-budget"
	dir := d.path(".agents/skills")
	var contents []string
	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			sub := filepath.Join(dir, entry.Name())
			skill := filepath.Join(sub, "SKILL.md")
			if !isDir(sub) || !isFile(skill) {
				continue
			}
			data, err := os.ReadFile(skill)
			if err != nil {
				continue
			}
			contents = append(contents, string(data))
		}
	}
	size := adapters.SkillListChars(contents)
	budget := adapters.SkillListBudget
	if size > budget {
		return Check{id, Warn, fmt.Sprintf("skill list is %d characters, over %d; Codex may drop descriptions", size, budget)}
	}
	return Check{id, OK, fmt.Sprintf("%d skills, list %d of %d characters", len(contents), size, budget)}
}

// codexTrust: Codex loads `.codex/` only for a trusted project; trust is set by Codex itself, never by dashcrew.
func (d Doctor) codexTrust() Check {
	const id = "codex-trust"
	home := d.CodexHome
	if home == "" {
		home = DefaultCodexHome(environment())
	}
	toml := ""
	if home != "" {
		if data, err := os.ReadFile(filepath.Join(home, "config.toml")); err == nil {
			toml = string(data)
		}
	}
	var trusted []string
	for _, match := range projectTable.FindAllStringSubmatchIndex(toml, -1) {
		rest := toml[match[1]:]
		if next := nextTable.FindStringIndex(rest); next != nil {
			rest = rest[:next[0]]
		}
		if !trustedLine.MatchString(rest) {
			continue
		}
		if match[2] >= 0 {
			trusted = append(trusted, toml[match[2]:match[3]])
		} else {
			trusted = append(trusted, toml[match[4]:match[5]])
		}
	}
	here := []string{d.ProjectDir}
	if isDir(d.ProjectDir) {
		if resolved, err := filepath.EvalSymlinks(d.ProjectDir); err == nil {
			here = append(here, resolved)
		}
	}
	for _, path := range trusted {
		for _, dir := range here {
			if dir == path || isWithin(path, dir) {
				return Check{id, OK, "project trusted in the Codex config"}
			}
		}
	}
	return Check{id, Warn, "project not trusted in the Codex config, so .codex/ is ignored; trust it when Codex asks, then approve the guard in /hooks"}
}

// isWithin is true when child is strictly inside parent.
func isWithin(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// codexAlwaysOn adds AGENTS.override.md, or AGENTS.md without one.
func codexAlwaysOn(_ Doctor, add func(relative string) bool) []string {
	if !add("AGENTS.override.md") {
		add("AGENTS.md")
	}
	return nil
}
