package me

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/mustache"
)

const (
	// marker is in every file and block me sync writes; a file without it is never overwritten.
	marker     = "<!-- dashcrew:me -"
	codexBegin = "<!-- dashcrew:me:begin -->"
	codexEnd   = "<!-- dashcrew:me:end -->"
	ruleFile   = "dashcrew-me.md"
	// copyMarker sits in a skill folder me sync copied because a symlink could not be made.
	copyMarker = ".dashcrew-me"
)

// Skills are the personal skills dashcrew ships, in personal/skills/.
var Skills = []string{"dashcrew-onboarding", "dashcrew-lead"}

// Places are the folders me sync reads and writes.
type Places struct {
	// Home holds `.dashcrew/me.yaml` and `.dashcrew/skills/`.
	Home string
	// ClaudeDir is `$CLAUDE_CONFIG_DIR`, else `~/.claude`.
	ClaudeDir string
	// CodexDir is `$CODEX_HOME`, else `~/.codex`.
	CodexDir string
	// GeminiDir is `~/.gemini`.
	GeminiDir string
}

// PlacesFrom resolves Places from the environment: HOME, else USERPROFILE.
func PlacesFrom(env map[string]string) (Places, error) {
	home := env["HOME"]
	if home == "" {
		home = env["USERPROFILE"]
	}
	if home == "" {
		return Places{}, errors.New("no home folder: set HOME")
	}
	p := Places{
		Home:      home,
		ClaudeDir: filepath.Join(home, ".claude"),
		CodexDir:  filepath.Join(home, ".codex"),
		GeminiDir: filepath.Join(home, ".gemini"),
	}
	if dir := env["CLAUDE_CONFIG_DIR"]; dir != "" {
		p.ClaudeDir = dir
	}
	if dir := env["CODEX_HOME"]; dir != "" {
		p.CodexDir = dir
	}
	return p, nil
}

// Change is one path me sync looked at. Kept means it was left alone because dashcrew did not write it.
type Change struct {
	Path string
	What string
	Kept bool
}

func (c Change) String() string { return c.What + "  " + c.Path }

// SelectedSkills are the personal skills the profile turns on.
func (p *Profile) SelectedSkills() []string {
	var out []string
	if p.Level == "new" {
		out = append(out, "dashcrew-onboarding")
	}
	if p.Role == "lead" {
		out = append(out, "dashcrew-lead")
	}
	return out
}

// Instructions renders the personal instructions for one agent; linked names the personal skills it can follow.
func Instructions(p *Profile, personal fs.FS, agent string, linked map[string]bool) (string, error) {
	source, err := fs.ReadFile(personal, "rules.md")
	if err != nil {
		return "", err
	}
	stacks := make([]string, len(p.Stacks))
	for i, stack := range p.Stacks {
		stacks[i] = "`" + stack + "`"
	}
	var chat any
	if p.ChatLanguage != "" {
		chat = p.ChatLanguage
	}
	return mustache.Render(string(source), map[string]any{
		"antigravity":      agent == "antigravity",
		"role":             p.Role,
		"stacks":           strings.Join(stacks, ", "),
		"new":              p.Level == "new",
		"lead":             p.Role == "lead",
		"chat_language":    chat,
		"onboarding_skill": linked["dashcrew-onboarding"],
		"lead_skill":       linked["dashcrew-lead"],
	})
}

// Sync writes the personal instructions for each agent in the profile and links its selected skills for Claude
// Code; it takes out what it wrote earlier for agents and skills the profile no longer has.
func Sync(p *Profile, places Places, personal fs.FS) ([]Change, error) {
	var changes []Change
	claude := contains(p.Agents, "claude-code")
	linked := map[string]bool{}
	for _, name := range Skills {
		var change Change
		var err error
		if claude && contains(p.SelectedSkills(), name) {
			change, err = linkSkill(places, personal, name)
			linked[name] = err == nil && !change.Kept
		} else {
			change, err = unlinkSkill(places, name)
		}
		if err != nil {
			return changes, err
		}
		changes = appendChange(changes, change)
	}
	for _, agent := range Agents {
		var change Change
		var err error
		if contains(p.Agents, agent) {
			var text string
			if text, err = Instructions(p, personal, agent, linked); err != nil {
				return changes, err
			}
			change, err = write(places, agent, text)
		} else {
			change, err = remove(places, agent)
		}
		if err != nil {
			return changes, err
		}
		changes = appendChange(changes, change)
	}
	return changes, nil
}

// Remove takes out every file, block and link me sync wrote; me.yaml stays.
func Remove(places Places) ([]Change, error) {
	var changes []Change
	for _, name := range Skills {
		change, err := unlinkSkill(places, name)
		if err != nil {
			return changes, err
		}
		changes = appendChange(changes, change)
		if err := os.RemoveAll(skillSource(places, name)); err != nil {
			return changes, err
		}
	}
	for _, agent := range Agents {
		change, err := remove(places, agent)
		if err != nil {
			return changes, err
		}
		changes = appendChange(changes, change)
	}
	return changes, nil
}

// Status compares what me sync would write with what is there, without writing; a stale or kept path is a change.
func Status(p *Profile, places Places, personal fs.FS) ([]Change, error) {
	var changes []Change
	claude := contains(p.Agents, "claude-code")
	linked := map[string]bool{}
	for _, name := range Skills {
		ours, present := linkState(places, name)
		want := claude && contains(p.SelectedSkills(), name)
		link := filepath.Join(places.ClaudeDir, "skills", name)
		switch {
		case want && present && !ours:
			changes = append(changes, Change{link, "not dashcrew's", true})
		case want && !present:
			changes = append(changes, Change{link, "missing", false})
		case !want && ours:
			changes = append(changes, Change{link, "left over", false})
		}
		linked[name] = want && ours
	}
	for _, agent := range Agents {
		path := instructionPath(places, agent)
		current, err := readOwned(places, agent)
		if err != nil {
			return nil, err
		}
		if !contains(p.Agents, agent) {
			if current.owned {
				changes = append(changes, Change{path, "left over", false})
			}
			continue
		}
		text, err := Instructions(p, personal, agent, linked)
		if err != nil {
			return nil, err
		}
		switch {
		case current.foreign:
			changes = append(changes, Change{path, "not dashcrew's", true})
		case !current.owned:
			changes = append(changes, Change{path, "missing", false})
		case current.text != text:
			changes = append(changes, Change{path, "stale", false})
		}
	}
	return changes, nil
}

func appendChange(changes []Change, change Change) []Change {
	if change.What == "" {
		return changes
	}
	return append(changes, change)
}

func instructionPath(places Places, agent string) string {
	switch agent {
	case "codex":
		return filepath.Join(places.CodexDir, "AGENTS.md")
	case "antigravity":
		return filepath.Join(places.GeminiDir, "config", "rules", ruleFile)
	default:
		return filepath.Join(places.ClaudeDir, "rules", ruleFile)
	}
}

// owned is what an instruction path holds: dashcrew's text, someone else's file, or nothing.
type owned struct {
	text    string
	owned   bool
	foreign bool
	// file is the whole Codex AGENTS.md; begin and end bound the block in it, end -1 when the block is not closed.
	file       string
	begin, end int
}

func readOwned(places Places, agent string) (owned, error) {
	data, err := os.ReadFile(instructionPath(places, agent))
	if errors.Is(err, fs.ErrNotExist) {
		return owned{}, nil
	}
	if err != nil {
		return owned{}, err
	}
	file := string(data)
	if agent != "codex" {
		if !strings.Contains(file, marker) {
			return owned{foreign: true}, nil
		}
		return owned{text: file, owned: true}, nil
	}
	begin := strings.Index(file, codexBegin)
	if begin < 0 {
		return owned{file: file, begin: -1, end: -1}, nil
	}
	end := strings.Index(file[begin:], codexEnd)
	if end < 0 {
		return owned{file: file, begin: begin, end: -1, foreign: true}, nil
	}
	end += begin + len(codexEnd)
	text := strings.TrimPrefix(file[begin+len(codexBegin):end-len(codexEnd)], "\n")
	return owned{text: text, owned: true, file: file, begin: begin, end: end}, nil
}

func write(places Places, agent, text string) (Change, error) {
	path := instructionPath(places, agent)
	current, err := readOwned(places, agent)
	if err != nil {
		return Change{}, err
	}
	if current.foreign {
		what := "kept, not written by dashcrew"
		if agent == "codex" {
			what = "kept, " + codexBegin + " has no " + codexEnd
		}
		return Change{path, what, true}, nil
	}
	if current.owned && current.text == text {
		return Change{path, "unchanged", false}, nil
	}
	out := text
	if agent == "codex" {
		block := codexBegin + "\n" + text + codexEnd
		switch {
		case current.owned:
			out = current.file[:current.begin] + block + current.file[current.end:]
		case strings.TrimSpace(current.file) == "":
			out = block + "\n"
		default:
			out = strings.TrimRight(current.file, "\n") + "\n\n" + block + "\n"
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Change{}, err
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return Change{}, err
	}
	if current.owned {
		return Change{path, "updated", false}, nil
	}
	return Change{path, "written", false}, nil
}

func remove(places Places, agent string) (Change, error) {
	path := instructionPath(places, agent)
	current, err := readOwned(places, agent)
	if err != nil || !current.owned {
		return Change{}, err
	}
	if agent != "codex" {
		return Change{path, "removed", false}, os.Remove(path)
	}
	rest := strings.TrimRight(current.file[:current.begin], "\n") + strings.TrimRight(current.file[current.end:], "\n")
	if strings.TrimSpace(rest) == "" {
		return Change{path, "removed", false}, os.Remove(path)
	}
	return Change{path, "block removed", false}, os.WriteFile(path, []byte(rest+"\n"), 0o644)
}

func skillSource(places Places, name string) string {
	return filepath.Join(places.Home, ".dashcrew", "skills", name)
}

// linkState says whether the Claude skill path is dashcrew's link (or copy) and whether anything is there.
func linkState(places Places, name string) (ours, present bool) {
	link := filepath.Join(places.ClaudeDir, "skills", name)
	info, err := os.Lstat(link)
	if err != nil {
		return false, false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(link)
		return err == nil && target == skillSource(places, name), true
	}
	_, err = os.Stat(filepath.Join(link, copyMarker))
	return err == nil, true
}

func linkSkill(places Places, personal fs.FS, name string) (Change, error) {
	source := skillSource(places, name)
	data, err := fs.ReadFile(personal, "skills/"+name+"/SKILL.md")
	if err != nil {
		return Change{}, err
	}
	if err := os.MkdirAll(source, 0o755); err != nil {
		return Change{}, err
	}
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), data, 0o644); err != nil {
		return Change{}, err
	}
	link := filepath.Join(places.ClaudeDir, "skills", name)
	ours, present := linkState(places, name)
	switch {
	case present && !ours:
		return Change{link, "kept, not linked by dashcrew", true}, nil
	case present:
		if info, err := os.Lstat(link); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return Change{link, "unchanged", false}, nil
		}
		if copied, err := os.ReadFile(filepath.Join(link, "SKILL.md")); err == nil && string(copied) == string(data) {
			return Change{link, "unchanged", false}, nil
		}
		return Change{link, "copied", false}, copySkill(data, link)
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return Change{}, err
	}
	if os.Symlink(source, link) == nil {
		return Change{link, "linked", false}, nil
	}
	return Change{link, "copied", false}, copySkill(data, link)
}

func copySkill(data []byte, link string) error {
	if err := os.MkdirAll(link, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(link, "SKILL.md"), data, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(link, copyMarker), []byte("copied by dashcrew me sync\n"), 0o644)
}

func unlinkSkill(places Places, name string) (Change, error) {
	ours, _ := linkState(places, name)
	if !ours {
		return Change{}, nil
	}
	link := filepath.Join(places.ClaudeDir, "skills", name)
	return Change{link, "unlinked", false}, os.RemoveAll(link)
}
