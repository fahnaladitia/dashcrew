// Package me is the personal profile (P10b): `~/.dashcrew/me.yaml`, one per machine, never committed. It changes
// how agents work with this member, through personal instruction files outside every repo; it never changes what a
// project allows, and the guard never reads it.
package me

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Roles a member may declare.
var Roles = []string{"be", "fe", "mobile", "devops", "fullstack", "lead"}

// Levels a member may declare.
var Levels = []string{"new", "regular"}

// Agents me sync writes personal instructions for.
var Agents = []string{"claude-code", "codex", "antigravity"}

// Profile is me.yaml.
type Profile struct {
	Role         string
	Stacks       []string
	Level        string
	ChatLanguage string
	Agents       []string
}

var (
	keys         = []string{"role", "stacks", "level", "chat_language", "agents"}
	stackName    = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	languageCode = regexp.MustCompile(`^[a-z]{2}$`)
)

// Path is me.yaml under home.
func Path(home string) string { return filepath.Join(home, ".dashcrew", "me.yaml") }

// Load reads me.yaml under home; os.ErrNotExist when there is none.
func Load(home string) (*Profile, error) {
	data, err := os.ReadFile(Path(home))
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

// Parse reads me.yaml text; the error names the key at fault.
func Parse(text string) (*Profile, error) {
	value, err := yamlutil.Parse(text)
	if err != nil {
		return nil, fmt.Errorf("me.yaml is not valid YAML: %w", err)
	}
	root, ok := value.(*omap.Map[any])
	if !ok {
		return nil, errors.New("me.yaml: expected a map")
	}
	for _, key := range root.Keys() {
		if !contains(keys, key) {
			return nil, fmt.Errorf("me.yaml %s: unknown key", key)
		}
	}
	p := &Profile{Level: "regular"}
	if p.Role, ok = root.Value("role").(string); !ok || !contains(Roles, p.Role) {
		return nil, fmt.Errorf("me.yaml role: expected one of %s", strings.Join(Roles, ", "))
	}
	if p.Stacks, err = names(root.Value("stacks"), "stacks", func(s string) bool { return stackName.MatchString(s) }); err != nil {
		return nil, err
	}
	if len(p.Stacks) == 0 {
		return nil, errors.New("me.yaml stacks: required, the platforms you work on, such as [flutter]")
	}
	if root.Has("level") {
		if p.Level, ok = root.Value("level").(string); !ok || !contains(Levels, p.Level) {
			return nil, fmt.Errorf("me.yaml level: expected one of %s", strings.Join(Levels, ", "))
		}
	}
	switch value := root.Value("chat_language").(type) {
	case nil:
	case string:
		if value != "" && !languageCode.MatchString(value) {
			return nil, errors.New("me.yaml chat_language: expected a two-letter ISO 639-1 code such as id, or empty")
		}
		p.ChatLanguage = value
	default:
		return nil, errors.New("me.yaml chat_language: expected a two-letter ISO 639-1 code such as id, or empty")
	}
	if p.Agents, err = names(root.Value("agents"), "agents", func(s string) bool { return contains(Agents, s) }); err != nil {
		return nil, fmt.Errorf("%w (one of %s)", err, strings.Join(Agents, ", "))
	}
	if len(p.Agents) == 0 {
		p.Agents = []string{"claude-code"}
	}
	return p, nil
}

func names(value any, key string, valid func(string) bool) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("me.yaml %s: expected a list", key)
	}
	var out []string
	for _, item := range list {
		name, ok := item.(string)
		if !ok || !valid(name) {
			return nil, fmt.Errorf("me.yaml %s: %v is not valid", key, item)
		}
		if !contains(out, name) {
			out = append(out, name)
		}
	}
	return out, nil
}

// Text is the profile as me.yaml, with a comment above each key.
func (p *Profile) Text() string {
	return "# Personal profile for dashcrew: how agents work with you. Never committed; the guard never reads it.\n" +
		"# Run dashcrew me sync after editing.\n\n" +
		"# " + strings.Join(Roles, " | ") + "\n" +
		"role: " + p.Role + "\n" +
		"# the platforms you work on\n" +
		"stacks: [" + strings.Join(p.Stacks, ", ") + "]\n" +
		"# " + strings.Join(Levels, " | ") + "\n" +
		"level: " + p.Level + "\n" +
		"# ISO 639-1 such as id; empty keeps each project's language.chat\n" +
		"chat_language: " + p.ChatLanguage + "\n" +
		"# " + strings.Join(Agents, ", ") + "\n" +
		"agents: [" + strings.Join(p.Agents, ", ") + "]\n"
}

// Save writes me.yaml under home.
func (p *Profile) Save(home string) error {
	if err := os.MkdirAll(filepath.Dir(Path(home)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(Path(home), []byte(p.Text()), 0o644)
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}
