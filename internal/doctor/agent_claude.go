package doctor

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

func (d Doctor) hook() Check {
	const id = "hook"
	if !d.exists(".claude/settings.json") {
		return Check{id, Fail, ".claude/settings.json missing; run dashcrew sync"}
	}
	settings, err := d.readObject(".claude/settings.json")
	var entries []any
	if err == nil {
		entries, err = preToolUse(settings)
	}
	if err != nil {
		return Check{id, Fail, ".claude/settings.json invalid: " + err.Error()}
	}
	return guardEntryCheck(id, entries, encode(adapters.ClaudeGuardHook()), "PreToolUse guard hook present")
}

func (d Doctor) claudeMd() Check {
	const id = "claude-md"
	if !d.exists("CLAUDE.md") {
		return Check{id, Fail, "CLAUDE.md missing; run dashcrew sync"}
	}
	if text, _ := d.read("CLAUDE.md"); !strings.Contains(text, "@AGENTS.md") {
		return Check{id, Fail, `CLAUDE.md does not import AGENTS.md; add a line "@AGENTS.md"`}
	}
	return Check{id, OK, "CLAUDE.md imports AGENTS.md"}
}

func (d Doctor) mcp() Check {
	const id = "mcp"
	if !d.exists(".mcp.json") {
		return Check{id, Warn, ".mcp.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".mcp.json")
	var servers *omap.Map[any]
	if err == nil {
		servers, err = mcpServers(config)
	}
	if err != nil {
		return Check{id, Warn, ".mcp.json invalid: " + err.Error()}
	}
	adapter := adapters.NewClaudeCode(d.Catalog, d.ProjectDir, d.platform())
	adapter.Knowledge = d.profileKnowledge()
	expected := adapter.McpServers()
	var problems []string
	for _, name := range expected.Keys() {
		switch current := servers.Value(name); {
		case current == nil:
			problems = append(problems, name+" missing")
		case encode(current) != encode(expected.Value(name)):
			problems = append(problems, name+" defined differently")
		}
	}
	if len(problems) == 0 {
		return Check{id, OK, definedDetail(expected.Keys())}
	}
	return Check{id, Warn, strings.Join(problems, ", ")}
}

// skills fails on a missing catalog skill and warns on a required project skill left out. An outside skill the
// generated files name (`idea-refine` and the like) is used only when installed, and plugins install skills
// elsewhere, so a missing one is a note on an ok check.
func (d Doctor) skills(lockedFiles *omap.Map[string]) Check {
	const id = "skills"
	if lockedFiles == nil {
		return Check{id, Skip, "needs the lock"}
	}
	// referenced are the skills the files name; byUser those named only in the AGENTS.md project block, which the
	// team writes, where a name no skill answers to is likely a typo rather than an optional outside skill.
	referenced := map[string]bool{}
	byUser := map[string]bool{}
	names := func(text string, into map[string]bool) {
		for _, match := range skillReference.FindAllStringSubmatch(text, -1) {
			if match[1] != "" {
				into[match[1]] = true
			} else {
				into[match[2]] = true
			}
		}
	}
	for _, path := range lockedFiles.Keys() {
		if !strings.HasSuffix(path, ".md") || !d.exists(path) {
			continue
		}
		text, err := d.read(path)
		if err != nil {
			continue
		}
		generated := text
		if path == "AGENTS.md" {
			if begin, end := strings.Index(text, sync.ProjectBegin), strings.Index(text, sync.ProjectEnd); begin >= 0 && end > begin {
				generated = text[:begin] + text[end:]
				names(text[begin:end], byUser)
			}
		}
		names(generated, referenced)
		if path == "AGENTS.md" {
			for _, name := range skillIndexNames(text) {
				referenced[name] = true
			}
		}
	}
	for name := range byUser {
		if referenced[name] {
			delete(byUser, name)
		} else {
			referenced[name] = true
		}
	}

	platform := d.platform()
	dashcrewSkills := map[string]bool{}
	for name := range d.Catalog.SkillsFor(platform) {
		dashcrewSkills[name] = true
	}
	for name := range d.Catalog.WorkflowsFor(platform) {
		dashcrewSkills[name] = true
	}
	for _, skill := range d.Catalog.ProjectSkills {
		if !skill.Optional {
			dashcrewSkills[skill.Name] = true
		}
	}
	packs := d.Catalog.PacksFor(platform)
	for _, family := range packs.Keys() {
		options := packs.Value(family)
		for _, option := range options.Keys() {
			for name := range options.Value(option).Skills {
				dashcrewSkills[name] = true
			}
		}
	}
	userDir := d.UserSkillsDir
	if userDir == "" {
		userDir = DefaultUserSkillsDir(environment())
	}
	sortedNames := make([]string, 0, len(referenced))
	for name := range referenced {
		sortedNames = append(sortedNames, name)
	}
	sort.Strings(sortedNames)
	var missing, external, unknown []string
	for _, name := range sortedNames {
		if d.exists(".claude/skills/" + name + "/SKILL.md") {
			continue
		}
		if dashcrewSkills[name] {
			missing = append(missing, name)
		} else if userDir == "" || !isFile(filepath.Join(userDir, name, "SKILL.md")) {
			if byUser[name] {
				unknown = append(unknown, name)
			} else {
				external = append(external, name)
			}
		}
	}
	var leftOut []string
	for _, skill := range d.Catalog.ProjectSkills {
		if !skill.Optional {
			leftOut = append(leftOut, skill.Warnings()...)
		}
	}
	if len(missing) > 0 {
		return Check{id, Fail, strings.Join(append([]string{"dashcrew skills missing: " + strings.Join(missing, ", ") + ", run dashcrew sync"}, leftOut...), "; ")}
	}
	optional := ""
	if len(external) > 0 {
		optional = "optional, not installed: " + strings.Join(external, ", ")
	}
	if len(unknown) > 0 {
		leftOut = append([]string{"not found locally (install, or ignore if a plugin provides them): " + strings.Join(unknown, ", ")}, leftOut...)
	}
	if len(leftOut) > 0 {
		if optional != "" {
			leftOut = append(leftOut, optional)
		}
		return Check{id, Warn, strings.Join(leftOut, "; ")}
	}
	detail := fmt.Sprintf("%d referenced skills found", len(referenced)-len(external)-len(unknown))
	if optional != "" {
		detail += "; " + optional
	}
	return Check{id, OK, detail}
}

// skillIndexNames are the skill names listed under the `### Skill index` heading of AGENTS.md; empty when there
// is none.
func skillIndexNames(agentsMd string) []string {
	start := strings.Index(agentsMd, "### Skill index\n")
	if start < 0 {
		return nil
	}
	section := agentsMd[start:]
	if end := strings.Index(agentsMd[start+1:], "\n#"); end >= 0 {
		section = agentsMd[start : start+1+end]
	}
	var names []string
	for _, match := range indexEntry.FindAllStringSubmatch(section, -1) {
		names = append(names, match[1])
	}
	return names
}

// claudeAlwaysOn adds CLAUDE.md and CLAUDE.local.md with the `@` imports they reach (importDepth hops), and the
// `.claude/rules` files without `paths`; it returns the imports outside the project.
func claudeAlwaysOn(d Doctor, add func(relative string) bool) []string {
	var outside []string
	var follow func(relative string, depth int)
	follow = func(relative string, depth int) {
		if !add(relative) || depth == importDepth {
			return
		}
		text, _ := d.read(relative)
		for _, target := range importsOf(text) {
			resolved, ok := d.inProject(filepath.Join(filepath.Dir(d.path(relative)), filepath.FromSlash(target)), target)
			if !ok {
				outside = append(outside, target)
				continue
			}
			follow(resolved, depth+1)
		}
	}
	follow("CLAUDE.md", 0)
	follow("CLAUDE.local.md", 0)
	for _, rule := range d.ruleFiles(".claude/rules", true) {
		if _, scoped := frontMatterValue(d.readOrEmpty(rule), "paths"); !scoped {
			add(rule)
		}
	}
	return outside
}
