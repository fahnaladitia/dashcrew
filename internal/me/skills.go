package me

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/worktree"
)

func validSkillName(name string) bool { return catalog.ValidProjectSkillName(name) }

// validOwnSkill keeps the shipped personal skills out of `skills`: the role and level select them.
func validOwnSkill(name string) bool { return validSkillName(name) && !contains(Skills, name) }

// skillMarker opens the body of an optional project skill me sync writes into a member's clone.
const skillMarker = marker + " an optional project skill this member turned on in .ai/me.yaml; written by dashcrew me sync, never committed -->"

// ---- the member's own skills: ~/.dashcrew/skills/<name>/, linked for Claude Code ----

// ownSkill is one of the member's own skills, with the description its SKILL.md front matter gives.
type ownSkill struct {
	name, description string
}

func ownSkillFile(places Places, name string) string {
	return filepath.Join(skillSource(places, name), "SKILL.md")
}

// ownSkills are the profile's own skills whose SKILL.md exists, and a kept change for each that does not.
func ownSkills(home *Profile, places Places) ([]ownSkill, []Change) {
	var found []ownSkill
	var notes []Change
	for _, name := range home.Skills {
		data, err := os.ReadFile(ownSkillFile(places, name))
		if err != nil {
			notes = append(notes, Change{ownSkillFile(places, name), "missing; skill " + name + " not linked", true})
			continue
		}
		description := catalog.SkillFrontMatter(string(data))["description"]
		if description == "" {
			description = "the member's own skill " + name
		}
		found = append(found, ownSkill{name, description})
	}
	return found, notes
}

// ownLinkedPath records the own skills me sync linked, so it never unlinks one the member linked by hand.
func ownLinkedPath(places Places) string {
	return filepath.Join(places.Home, ".dashcrew", "me-linked-skills")
}

func readOwnLinked(places Places) []string {
	data, err := os.ReadFile(ownLinkedPath(places))
	if err != nil {
		return nil
	}
	return strings.Fields(string(data))
}

func saveOwnLinked(places Places, names []string) error {
	if len(names) == 0 {
		if err := os.Remove(ownLinkedPath(places)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	sort.Strings(names)
	if err := os.MkdirAll(filepath.Dir(ownLinkedPath(places)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(ownLinkedPath(places), []byte(strings.Join(names, "\n")+"\n"), 0o644)
}

// linkOwn links an own skill folder into the Claude skills folder, or copies it where a symlink cannot be made.
func linkOwn(places Places, name string) (Change, error) {
	data, err := os.ReadFile(ownSkillFile(places, name))
	if err != nil {
		return Change{}, err
	}
	return linkOrCopy(places, name, data)
}

// ownLinksLeftOver are the own skills me sync linked earlier that the profile no longer lists (wanted holds every
// listed name, readable or not), and whose link is still dashcrew's.
func ownLinksLeftOver(places Places, wanted map[string]bool) []string {
	var out []string
	for _, name := range readOwnLinked(places) {
		if wanted[name] || contains(Skills, name) {
			continue
		}
		if ours, _ := linkState(places, name); ours {
			out = append(out, name)
		}
	}
	return out
}

// ---- optional project skills: written into the clone of a member who turns them on ----

// projectSkillsLoader loads a repo's project skills; the shipped catalog is read once.
type projectSkillsLoader struct {
	shipped *catalog.Catalog
}

func (l *projectSkillsLoader) load(dir string) ([]catalog.ProjectSkill, error) {
	if l.shipped == nil {
		shipped, err := catalog.LoadEmbedded()
		if err != nil {
			return nil, err
		}
		l.shipped = shipped
	}
	overlaid, err := l.shipped.WithProjectPacks(dir)
	if err != nil {
		return nil, err
	}
	return overlaid.ProjectSkills, nil
}

// repoSkills are the optional project skills a repo profile turns on, and a change for each name it cannot turn
// on. failed means the repo's .ai/ did not load: me sync then leaves that repo's skill files as they are.
func repoSkills(r repo, loader *projectSkillsLoader) (on []catalog.ProjectSkill, notes []Change, failed bool) {
	if r.profile == nil || len(r.profile.Skills) == 0 {
		return nil, nil, false
	}
	skills, err := loader.load(r.dir)
	if err != nil {
		return nil, []Change{{r.dir, "skills not read, left as they are: " + err.Error(), true}}, true
	}
	byName := map[string]catalog.ProjectSkill{}
	for _, skill := range skills {
		byName[skill.Name] = skill
	}
	for _, name := range r.profile.Skills {
		skill, ok := byName[name]
		switch {
		case !ok:
			notes = append(notes, Change{r.dir, "skill " + name + ": no optional skill of that name in .ai/skills", true})
		case !skill.Optional:
			notes = append(notes, Change{r.dir, "skill " + name + ": required in this repo, on for everyone", false})
		default:
			on = append(on, skill)
		}
	}
	return on, notes, false
}

// skillFiles are the repo-relative skill files the member's agents read for name, one per folder.
func skillFiles(home *Profile, name string) map[string]string {
	out := map[string]string{}
	for _, agent := range adapters.SupportedAdapters {
		if contains(home.Agents, agent) {
			out[adapters.SkillFile(agent, name)] = agent
		}
	}
	return out
}

// skillRoots are the agent skill folders me sync may have written into, repo-relative.
func skillRoots() []string {
	seen := map[string]bool{}
	var out []string
	for _, agent := range adapters.SupportedAdapters {
		root := path.Dir(path.Dir(adapters.SkillFile(agent, "x")))
		if !seen[root] {
			seen[root] = true
			out = append(out, root)
		}
	}
	return out
}

// repoSkillTargets are the files of each turned-on optional skill, plus every skill file me sync wrote earlier
// in this repo that is no longer wanted.
func repoSkillTargets(home *Profile, dir string, on []catalog.ProjectSkill) []target {
	var out []target
	wanted := map[string]bool{}
	for _, skill := range on {
		body := skillMarker + "\n\n" + skill.Body
		files := skillFiles(home, skill.Name)
		relatives := make([]string, 0, len(files))
		for relative := range files {
			relatives = append(relatives, relative)
		}
		sort.Strings(relatives)
		for _, relative := range relatives {
			text := adapters.AgentsSkillFile(skill.Name, skill.Meta, body)
			if files[relative] == "claude-code" {
				text = adapters.ClaudeSkillFile(skill.Name, skill.Meta, body)
			}
			full := filepath.Join(dir, filepath.FromSlash(relative))
			wanted[full] = true
			out = append(out, target{path: full, text: text, want: true, skill: true})
		}
	}
	for _, root := range skillRoots() {
		entries, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(root)))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			full := filepath.Join(dir, filepath.FromSlash(root), entry.Name(), "SKILL.md")
			if wanted[full] {
				continue
			}
			if data, err := os.ReadFile(full); err == nil && strings.Contains(string(data), skillMarker) {
				out = append(out, target{path: full, skill: true})
			}
		}
	}
	return out
}

// excludeEntry is the git exclude line for the folder of a skill file, repo-relative with a trailing slash.
func excludeEntry(dir, file string) string {
	relative, err := filepath.Rel(dir, filepath.Dir(file))
	if err != nil {
		return ""
	}
	return filepath.ToSlash(relative) + "/"
}

func isGitCheckout(dir string) bool {
	return exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run() == nil
}

// excludeSkills lists each written skill folder in the repo's git exclude file; nothing outside a git checkout.
func excludeSkills(dir string, written []target) error {
	var entries []string
	for _, t := range written {
		entries = append(entries, excludeEntry(dir, t.path))
	}
	if len(entries) == 0 || !isGitCheckout(dir) {
		return nil
	}
	return worktree.Exclude(dir, entries)
}

// unexcludeSkills drops the exclude lines of skill folders me sync removed, so a skill that becomes required, and
// is then written by sync, shows up in git status again.
func unexcludeSkills(dir string, removed []target) error {
	if len(removed) == 0 || !isGitCheckout(dir) {
		return nil
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--path-format=absolute", "--git-path", "info/exclude").Output()
	if err != nil {
		return nil
	}
	excludePath := strings.TrimSpace(string(out))
	data, err := os.ReadFile(excludePath)
	if err != nil {
		return nil
	}
	drop := map[string]bool{}
	for _, t := range removed {
		drop["/"+excludeEntry(dir, t.path)] = true
	}
	lines := strings.Split(string(data), "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !drop[line] {
			kept = append(kept, line)
		}
	}
	if len(kept) == len(lines) {
		return nil
	}
	return os.WriteFile(excludePath, []byte(strings.Join(kept, "\n")), 0o644)
}

// dropEmptySkillDir removes the folder of a skill file me sync just removed, when nothing else is in it.
func dropEmptySkillDir(file string) error {
	err := os.Remove(filepath.Dir(file))
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if entries, readErr := os.ReadDir(filepath.Dir(file)); readErr == nil && len(entries) > 0 {
		return nil
	}
	return err
}

// ownSkillLines and repoSkillLines are the template lines for a list of skills: name and when to use it.
func ownSkillLines(skills []ownSkill) []any {
	out := []any{}
	for _, skill := range skills {
		out = append(out, map[string]any{"name": skill.name, "when": skill.description})
	}
	return out
}

func repoSkillLines(skills []catalog.ProjectSkill) []any {
	out := []any{}
	for _, skill := range skills {
		when := skill.Meta.Description
		if skill.Meta.WhenToUse != nil {
			when = *skill.Meta.WhenToUse
		}
		out = append(out, map[string]any{"name": skill.Name, "when": when})
	}
	return out
}
