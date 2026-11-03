package me

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/filemode"
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

// projectSkillsLoader loads a repo's `.ai/skills` alone; the names dashcrew ships are read once per run, the names
// the repo's own packs and platforms take are read from their folder layout.
type projectSkillsLoader struct {
	shipped map[string]bool
}

// load reads the repo's project skills; a name its own packs or platforms take is refused, as sync refuses it.
func (l *projectSkillsLoader) load(dir string) ([]catalog.ProjectSkill, error) {
	if l.shipped == nil {
		shipped, err := catalog.LoadEmbedded()
		if err != nil {
			return nil, err
		}
		l.shipped = shipped.TakenSkillNames()
	}
	taken := catalog.ProjectSkillNames(dir)
	for name := range l.shipped {
		taken[name] = true
	}
	return catalog.LoadProjectSkills(dir, taken)
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
			for _, warning := range skill.Warnings() {
				notes = append(notes, Change{r.dir, warning, false})
			}
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
			out = append(out, target{path: full, text: text, want: true, skill: true, files: skill.Files})
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

// ---- supporting files of an optional skill: owned through the folder's manifest ----

// skillManifest lists, one per line, the supporting files me sync wrote into a skill folder. A file it does not
// list was not written by me sync and is never overwritten or removed.
const skillManifest = ".dashcrew-me"

// readManifest is the manifest's paths that stay inside the skill folder, sorted; any other line is ignored, so a
// manifest written by hand cannot reach a file outside the folder.
func readManifest(folder string) []string {
	data, err := os.ReadFile(filepath.Join(folder, skillManifest))
	if err != nil {
		return nil
	}
	var out []string
	for _, name := range strings.Fields(string(data)) {
		if filepath.IsLocal(filepath.FromSlash(name)) && name != skillManifest && name != "SKILL.md" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func writeManifest(folder string, files []catalog.SkillFile) error {
	if len(files) == 0 {
		if err := os.Remove(filepath.Join(folder, skillManifest)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	lines := make([]string, len(files))
	for i, file := range files {
		lines[i] = file.Path
	}
	return os.WriteFile(filepath.Join(folder, skillManifest), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// pruneEmpty removes the empty folders between file and folder, deepest first, never folder or anything above it.
func pruneEmpty(folder, file string) {
	for dir := filepath.Dir(file); ; dir = filepath.Dir(dir) {
		relative, err := filepath.Rel(folder, dir)
		if err != nil || relative == "." || !filepath.IsLocal(relative) || os.Remove(dir) != nil {
			return
		}
	}
}

// writeSkill writes a skill target and, unless its SKILL.md was kept as foreign, its supporting files: each one
// the manifest lists or that is absent, executable when the source is, and the files it no longer carries go.
func writeSkill(t target) ([]Change, error) {
	change, err := write(t)
	if err != nil || change.Kept {
		return []Change{change}, err
	}
	changes := []Change{change}
	folder := filepath.Dir(t.path)
	owned := map[string]bool{}
	for _, name := range readManifest(folder) {
		owned[name] = true
	}
	carried := map[string]bool{}
	var ownedNow []catalog.SkillFile
	for _, file := range t.files {
		carried[file.Path] = true
		full := filepath.Join(folder, filepath.FromSlash(file.Path))
		current, readErr := os.ReadFile(full)
		if readErr == nil && !owned[file.Path] {
			changes = append(changes, Change{full, "kept, not written by dashcrew", true})
			continue
		}
		ownedNow = append(ownedNow, file)
		if readErr != nil || string(current) != file.Content {
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return changes, err
			}
			if err := os.WriteFile(full, []byte(file.Content), 0o644); err != nil {
				return changes, err
			}
			what := "written"
			if readErr == nil {
				what = "updated"
			}
			changes = append(changes, Change{full, what, false})
		}
		if err := filemode.SetExecutable(full, file.Executable); err != nil {
			return changes, err
		}
	}
	for _, name := range readManifest(folder) {
		if carried[name] {
			continue
		}
		full := filepath.Join(folder, filepath.FromSlash(name))
		err := os.Remove(full)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return changes, err
		}
		pruneEmpty(folder, full)
		changes = append(changes, Change{full, "removed", false})
	}
	return changes, writeManifest(folder, ownedNow)
}

// removeSkill takes out a skill target's SKILL.md, the supporting files its manifest lists, the manifest, and the
// folders left empty; a file me sync did not write stays.
func removeSkill(t target) ([]Change, error) {
	folder := filepath.Dir(t.path)
	current, err := readOwned(t)
	if err != nil || !current.owned {
		return nil, err
	}
	var changes []Change
	for _, name := range readManifest(folder) {
		full := filepath.Join(folder, filepath.FromSlash(name))
		if err := os.Remove(full); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return changes, err
		}
		pruneEmpty(folder, full)
	}
	if err := os.Remove(filepath.Join(folder, skillManifest)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return changes, err
	}
	change, err := remove(t)
	if err != nil {
		return changes, err
	}
	changes = appendChange(changes, change)
	return changes, dropEmptySkillDir(t.path)
}

// skillFileStatus compares a written skill's supporting files with what me sync would write: content, execute
// bits, and the files its manifest lists that the skill no longer carries.
func skillFileStatus(t target) []Change {
	folder := filepath.Dir(t.path)
	owned, carried := map[string]bool{}, map[string]bool{}
	for _, name := range readManifest(folder) {
		owned[name] = true
	}
	var changes []Change
	for _, file := range t.files {
		carried[file.Path] = true
		full := filepath.Join(folder, filepath.FromSlash(file.Path))
		data, err := os.ReadFile(full)
		info, statErr := os.Stat(full)
		switch {
		case err != nil || statErr != nil:
			changes = append(changes, Change{full, "missing", false})
		case !owned[file.Path]:
			changes = append(changes, Change{full, "not dashcrew's", true})
		case string(data) != file.Content || executableBits(info) != file.Executable:
			changes = append(changes, Change{full, "stale", false})
		}
	}
	for _, name := range readManifest(folder) {
		full := filepath.Join(folder, filepath.FromSlash(name))
		if !carried[name] && fileExists(full) {
			changes = append(changes, Change{full, "left over", false})
		}
	}
	return changes
}

// executableBits is whether a file has execute bits; always false where the file system has none.
func executableBits(info os.FileInfo) bool {
	return runtime.GOOS != "windows" && info.Mode().Perm()&0o111 != 0
}

func fileExists(file string) bool {
	_, err := os.Lstat(file)
	return err == nil
}
