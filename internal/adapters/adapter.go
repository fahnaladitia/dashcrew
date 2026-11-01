// Package adapters writes the per-agent files: Claude Code, Codex and Antigravity.
// AGENTS.md and the guard policy are shared and written by sync.
package adapters

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/projectfiles"
	"github.com/fahnaladitia/dashcrew/internal/render"
)

// MergeResult is what Merge wrote and the MCP servers the adapter owns afterwards.
type MergeResult struct {
	Written  []string
	OwnedMcp map[string]bool
}

// Adapter is what sync writes for one agent named in `project.yaml` `agents`.
type Adapter interface {
	// Agent is the `agents` value this adapter serves.
	Agent() string
	// Files are the generated files (relative path -> content) in the order the Dart adapter writes them,
	// tracked in the lock. lockedFiles tells a file dashcrew wrote earlier apart from one the user owns;
	// warnings may be nil.
	Files(p *profile.Profile, targets []render.RenderedTarget, context map[string]any, lockedFiles map[string]string, warnings *[]string) (*omap.Map[string], error)
	// Merge adds dashcrew entries to files the user edits too, keeping everything else.
	// ownedMcp are the MCP servers this adapter wrote before.
	Merge(ownedMcp map[string]bool, warnings *[]string) (MergeResult, error)
}

// SupportedAdapters are the agents AdapterFor accepts, in message order.
var SupportedAdapters = []string{"claude-code", "codex", "antigravity"}

// AdapterFor returns the adapter for agent on platform.
func AdapterFor(agent string, c *catalog.Catalog, projectDir, platform string) (Adapter, error) {
	switch agent {
	case "claude-code":
		return NewClaudeCode(c, projectDir, platform), nil
	case "codex":
		return NewCodex(c, projectDir, platform), nil
	case "antigravity":
		return NewAntigravity(c, projectDir, platform), nil
	}
	return nil, fmt.Errorf("Agent adapter not available yet: %s. Supported: %s.", agent, strings.Join(SupportedAdapters, ", "))
}

func warn(warnings *[]string, message string) {
	if warnings != nil {
		*warnings = append(*warnings, message)
	}
}

// SkillDoc is a skill or workflow ready to write: its description and rendered body.
type SkillDoc struct {
	Name string
	Meta catalog.DocMeta
	Body string
}

// SkillDocs are the skills and workflows among targets, for every adapter alike.
// A skill no selected pack describes is skipped with a warning; a workflow without a description is an error.
func SkillDocs(c *catalog.Catalog, p *profile.Profile, targets []render.RenderedTarget, warnings *[]string) ([]SkillDoc, error) {
	packSkills := map[string]catalog.DocMeta{}
	for _, pack := range render.New(c).SelectedPacks(p) {
		for name, meta := range pack.Skills {
			packSkills[name] = meta
		}
	}
	skills := c.SkillsFor(p.Platform)
	workflows := c.WorkflowsFor(p.Platform)
	var docs []SkillDoc
	for _, target := range targets {
		kind, name, ok := strings.Cut(target.Target, "/")
		if !ok {
			continue
		}
		switch kind {
		case "skill":
			meta, ok := skills[name]
			if !ok {
				meta, ok = packSkills[name]
			}
			if !ok {
				warn(warnings, "skill "+name+" skipped: no selected pack defines skills/"+name+".yaml")
				continue
			}
			docs = append(docs, SkillDoc{Name: name, Meta: meta, Body: target.Content})
		case "workflow":
			meta, ok := workflows[name]
			if !ok {
				return nil, errors.New("workflow/" + name + " has no meta.yaml")
			}
			docs = append(docs, SkillDoc{Name: name, Meta: meta, Body: target.Content})
		}
	}
	return docs, nil
}

func jsonString(s string) string {
	text, _ := dartjson.Encode(s)
	return text
}

// AgentsSkillFile is `.agents/skills/<name>/SKILL.md`, the format Codex and Antigravity both read: front matter
// `name` and `description` only, so the trigger joins the description and the argument hint opens the body.
func AgentsSkillFile(name string, meta catalog.DocMeta, body string) string {
	description := meta.Description
	if meta.WhenToUse != nil {
		description += " Use when: " + *meta.WhenToUse
	}
	arguments := ""
	if meta.ArgumentHint != nil {
		arguments = "Arguments: `" + *meta.ArgumentHint + "`\n\n"
	}
	return "---\nname: " + name + "\ndescription: " + jsonString(description) + "\n---\n\n" + arguments + body
}

// rolesIn yields each role target whose role exists and matches context.
func rolesIn(c *catalog.Catalog, targets []render.RenderedTarget, context map[string]any, each func(role catalog.RoleMeta, body string)) {
	for _, target := range targets {
		name, ok := strings.CutPrefix(target.Target, "role/")
		if !ok {
			continue
		}
		role, ok := c.Roles[name]
		if !ok || !render.WhenMatches(role.When, context) {
			continue
		}
		each(role, target.Content)
	}
}

// readJSONObject reads a JSON object file; a missing file is an empty object.
func readJSONObject(project projectfiles.Project, path string) (*omap.Map[any], error) {
	if !project.Exists(path) {
		return omap.New[any](), nil
	}
	text, err := project.Read(path)
	if err != nil {
		return nil, err
	}
	value, err := dartjson.Decode(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	object, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fmt.Errorf("%s: expected a JSON object", path)
	}
	return object, nil
}

func writeJSON(project projectfiles.Project, path string, value any) (bool, error) {
	text, err := dartjson.EncodeIndent(value, "  ")
	if err != nil {
		return false, err
	}
	return project.WriteIfChanged(path, text+"\n")
}

// objectAt returns the object under key, or a new empty one when the key is absent or null.
func objectAt(parent *omap.Map[any], key, path string) (*omap.Map[any], error) {
	value := parent.Value(key)
	if value == nil {
		return omap.New[any](), nil
	}
	object, ok := value.(*omap.Map[any])
	if !ok {
		return nil, fmt.Errorf("%s: %s is not an object", path, key)
	}
	return object, nil
}

// MergeJSONMcpServers adds servers to the `mcpServers` map of the JSON file at path, keeping every other entry.
// A server is replaced only when dashcrew owned it before; one the user defined differently stays, with a warning.
func MergeJSONMcpServers(project projectfiles.Project, path string, servers *omap.Map[any], owned map[string]bool, warnings *[]string) (bool, map[string]bool, error) {
	config, err := readJSONObject(project, path)
	if err != nil {
		return false, nil, err
	}
	current, err := objectAt(config, "mcpServers", path)
	if err != nil {
		return false, nil, err
	}
	nowOwned := map[string]bool{}
	for name := range owned {
		nowOwned[name] = true
	}
	for _, name := range servers.Keys() {
		server := servers.Value(name)
		existing := current.Value(name)
		if existing == nil || owned[name] {
			current.Set(name, server)
			nowOwned[name] = true
			continue
		}
		left, err := dartjson.Encode(existing)
		if err != nil {
			return false, nil, err
		}
		right, err := dartjson.Encode(server)
		if err != nil {
			return false, nil, err
		}
		if left != right {
			warn(warnings, path+` already defines "`+name+`" differently; left as is.`)
		}
	}
	config.Set("mcpServers", current)
	changed, err := writeJSON(project, path, config)
	return changed, nowOwned, err
}

// replaceGuardEntry puts hook in the PreToolUse list under config's `hooks` object: in place of the first entry
// that calls `.ai/bin/guard`, or last.
func replaceGuardEntry(config *omap.Map[any], hook *omap.Map[any], path string) error {
	hooks, err := objectAt(config, "hooks", path)
	if err != nil {
		return err
	}
	var preToolUse []any
	if value := hooks.Value("PreToolUse"); value != nil {
		list, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s: hooks.PreToolUse is not a list", path)
		}
		preToolUse = append(preToolUse, list...)
	}
	index := -1
	for i, entry := range preToolUse {
		text, err := dartjson.Encode(entry)
		if err != nil {
			return err
		}
		if strings.Contains(text, ".ai/bin/guard") {
			index = i
			break
		}
	}
	if index >= 0 {
		preToolUse[index] = hook
	} else {
		preToolUse = append(preToolUse, hook)
	}
	if preToolUse == nil {
		preToolUse = []any{}
	}
	hooks.Set("PreToolUse", preToolUse)
	config.Set("hooks", hooks)
	return nil
}

func stringList(items []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

func platformServers(c *catalog.Catalog, platform string) *omap.Map[catalog.McpServer] {
	if spec := c.Platforms[platform]; spec != nil && spec.McpServers != nil {
		return spec.McpServers
	}
	return omap.New[catalog.McpServer]()
}
