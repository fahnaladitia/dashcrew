package me

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
	// Home holds `.dashcrew/me.yaml`, `.dashcrew/me-repos` and `.dashcrew/skills/`.
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

// ---- registry of repos with a `.ai/me.yaml` ----

func registryPath(places Places) string { return filepath.Join(places.Home, ".dashcrew", "me-repos") }

// Repos are the repo folders me sync writes personal files into, in the order they were registered.
func Repos(places Places) ([]string, error) {
	data, err := os.ReadFile(registryPath(places))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !contains(out, line) {
			out = append(out, line)
		}
	}
	return out, nil
}

func saveRepos(places Places, repos []string) error {
	if len(repos) == 0 {
		err := os.Remove(registryPath(places))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(registryPath(places)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(registryPath(places), []byte(strings.Join(repos, "\n")+"\n"), 0o644)
}

// Register adds a repo folder (absolute) to the repos me sync writes into.
func Register(places Places, dir string) error {
	repos, err := Repos(places)
	if err != nil || contains(repos, dir) {
		return err
	}
	return saveRepos(places, append(repos, dir))
}

// Unregister drops a repo folder from the repos me sync writes into.
func Unregister(places Places, dir string) error {
	repos, err := Repos(places)
	if err != nil {
		return err
	}
	var kept []string
	for _, repo := range repos {
		if repo != dir {
			kept = append(kept, repo)
		}
	}
	return saveRepos(places, kept)
}

// repo is one registered repo; profile is nil when its `.ai/me.yaml` (or the folder) is gone.
type repo struct {
	dir     string
	profile *RepoProfile
}

func loadRepos(places Places) ([]repo, error) {
	dirs, err := Repos(places)
	if err != nil {
		return nil, err
	}
	var out []repo
	for _, dir := range dirs {
		profile, err := LoadRepo(dir)
		if err != nil {
			return nil, err
		}
		out = append(out, repo{dir, profile})
	}
	return out, nil
}

// ---- rendering ----

// Instructions renders the home personal instructions for one agent; linked names the personal skills it can follow.
func Instructions(p *Profile, personal fs.FS, agent string, linked map[string]bool) (string, error) {
	return renderHome(p, personal, agent, linked, false)
}

func baseContext(p *Profile, linked map[string]bool) map[string]any {
	stacks := make([]string, len(p.Stacks))
	for i, stack := range p.Stacks {
		stacks[i] = "`" + stack + "`"
	}
	var chat any
	if p.ChatLanguage != "" {
		chat = p.ChatLanguage
	}
	return map[string]any{
		"antigravity": false, "marker": true, "home": false, "repo": false, "repos": false,
		"h": "##", "h1": "#", "title": "", "where": "", "there": "",
		"role": p.Role, "level": p.Level, "stacks": strings.Join(stacks, ", "), "chat_language": chat,
		"new": p.Level == "new", "lead": p.Role == "lead",
		"lead_off": false, "new_off": false,
		"onboarding_skill": linked["dashcrew-onboarding"], "lead_skill": linked["dashcrew-lead"],
	}
}

func render(personal fs.FS, context map[string]any) (string, error) {
	source, err := fs.ReadFile(personal, "rules.md")
	if err != nil {
		return "", err
	}
	return mustache.Render(string(source), context)
}

func renderHome(p *Profile, personal fs.FS, agent string, linked map[string]bool, repos bool) (string, error) {
	context := baseContext(p, linked)
	context["home"], context["repos"], context["antigravity"] = true, repos, agent == "antigravity"
	return render(personal, context)
}

// renderRepo renders a repo's lines: as its own file, or for codex as a section of the user block.
func renderRepo(home *Profile, r *RepoProfile, dir string, personal fs.FS, agent string, linked map[string]bool) (string, error) {
	effective := EffectiveIn(home, r)
	context := baseContext(&effective.Profile, linked)
	homeNew, homeLead := home.Level == "new", home.Role == "lead"
	context["repo"] = true
	context["new"], context["new_off"] = effective.Level == "new" && !homeNew, homeNew && effective.Level != "new"
	context["lead"], context["lead_off"] = effective.Role == "lead" && !homeLead, homeLead && effective.Role != "lead"
	if agent == "codex" {
		where := "the repository at `" + filepath.ToSlash(dir) + "`"
		context["marker"] = false
		context["h1"], context["h"] = "##", "###"
		context["title"] = "In the repository at `" + filepath.ToSlash(dir) + "`"
		context["where"], context["there"] = where, "there"
	} else {
		context["antigravity"] = agent == "antigravity"
		context["title"] = "Personal profile in this repository (dashcrew)"
		context["where"], context["there"] = "this repository", "here"
	}
	return render(personal, context)
}

// ---- targets: every file or block me sync manages, and what it should hold ----

type target struct {
	path  string
	block bool
	// text is what the target should hold when want; otherwise it should be absent.
	text string
	want bool
}

func homeInstructionPath(places Places, agent string) string {
	switch agent {
	case "codex":
		return filepath.Join(places.CodexDir, "AGENTS.md")
	case "antigravity":
		return filepath.Join(places.GeminiDir, "config", "rules", ruleFile)
	default:
		return filepath.Join(places.ClaudeDir, "rules", ruleFile)
	}
}

func repoInstructionPath(dir, agent string) string {
	if agent == "antigravity" {
		return filepath.Join(dir, ".agents", "rules", ruleFile)
	}
	return filepath.Join(dir, ".claude", "rules", ruleFile)
}

func selectedSkills(home *Profile, repos []repo) []string {
	out := append([]string(nil), home.SelectedSkills()...)
	for _, r := range repos {
		if r.profile == nil {
			continue
		}
		effective := EffectiveIn(home, r.profile)
		for _, name := range effective.SelectedSkills() {
			if !contains(out, name) {
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out
}

func targets(home *Profile, repos []repo, places Places, personal fs.FS, linked map[string]bool) ([]target, error) {
	live := false
	for _, r := range repos {
		live = live || r.profile != nil
	}
	var out []target
	for _, agent := range Agents {
		t := target{path: homeInstructionPath(places, agent), block: agent == "codex", want: contains(home.Agents, agent)}
		if t.want {
			text, err := renderHome(home, personal, agent, linked, live)
			if err != nil {
				return nil, err
			}
			if agent == "codex" {
				for _, r := range repos {
					if r.profile == nil {
						continue
					}
					section, err := renderRepo(home, r.profile, r.dir, personal, agent, linked)
					if err != nil {
						return nil, err
					}
					text += "\n" + section
				}
			}
			t.text = text
		}
		out = append(out, t)
	}
	for _, r := range repos {
		for _, agent := range []string{"claude-code", "antigravity"} {
			t := target{path: repoInstructionPath(r.dir, agent), want: r.profile != nil && contains(home.Agents, agent)}
			if t.want {
				text, err := renderRepo(home, r.profile, r.dir, personal, agent, linked)
				if err != nil {
					return nil, err
				}
				t.text = text
			}
			out = append(out, t)
		}
	}
	return out, nil
}

// ---- sync, status, remove ----

// Sync writes the personal instructions for each agent in the home profile and for each registered repo, and links
// the personal skills any of them selects for Claude Code; it takes out what it wrote earlier and no longer needs.
func Sync(home *Profile, places Places, personal fs.FS) ([]Change, error) {
	repos, err := loadRepos(places)
	if err != nil {
		return nil, err
	}
	var changes []Change
	claude := contains(home.Agents, "claude-code")
	selected := selectedSkills(home, repos)
	linked := map[string]bool{}
	for _, name := range Skills {
		var change Change
		if claude && contains(selected, name) {
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
	all, err := targets(home, repos, places, personal, linked)
	if err != nil {
		return changes, err
	}
	for _, t := range all {
		var change Change
		if t.want {
			change, err = write(t)
		} else {
			change, err = remove(t)
		}
		if err != nil {
			return changes, err
		}
		changes = appendChange(changes, change)
	}
	for _, r := range repos {
		if r.profile == nil {
			changes = append(changes, Change{r.dir, "no .ai/me.yaml there; dashcrew me sync --prune forgets it", false})
		}
	}
	return changes, nil
}

// Status compares what me sync would write with what is there, without writing; a stale or kept path is a change.
func Status(home *Profile, places Places, personal fs.FS) ([]Change, error) {
	repos, err := loadRepos(places)
	if err != nil {
		return nil, err
	}
	var changes []Change
	claude := contains(home.Agents, "claude-code")
	selected := selectedSkills(home, repos)
	linked := map[string]bool{}
	for _, name := range Skills {
		ours, present := linkState(places, name)
		want := claude && contains(selected, name)
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
	all, err := targets(home, repos, places, personal, linked)
	if err != nil {
		return nil, err
	}
	for _, t := range all {
		current, err := readOwned(t)
		if err != nil {
			return nil, err
		}
		switch {
		case !t.want:
			if current.owned {
				changes = append(changes, Change{t.path, "left over", false})
			}
		case current.foreign:
			changes = append(changes, Change{t.path, "not dashcrew's", true})
		case !current.owned:
			changes = append(changes, Change{t.path, "missing", false})
		case current.text != t.text:
			changes = append(changes, Change{t.path, "stale", false})
		}
	}
	for _, r := range repos {
		if r.profile == nil {
			changes = append(changes, Change{r.dir, "no .ai/me.yaml there", false})
		}
	}
	return changes, nil
}

// Remove takes out every file, block and link me sync wrote, in home and in every registered repo, and forgets the
// repos; me.yaml and each repo's .ai/me.yaml stay.
func Remove(places Places) ([]Change, error) {
	repos, err := Repos(places)
	if err != nil {
		return nil, err
	}
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
	var all []target
	for _, agent := range Agents {
		all = append(all, target{path: homeInstructionPath(places, agent), block: agent == "codex"})
	}
	for _, dir := range repos {
		for _, agent := range []string{"claude-code", "antigravity"} {
			all = append(all, target{path: repoInstructionPath(dir, agent)})
		}
	}
	for _, t := range all {
		change, err := remove(t)
		if err != nil {
			return changes, err
		}
		changes = appendChange(changes, change)
	}
	return changes, saveRepos(places, nil)
}

// RemoveRepo takes one repo's personal files out and forgets it, then syncs the rest, so its Codex section and the
// skills only it selected go too; its .ai/me.yaml stays.
func RemoveRepo(home *Profile, places Places, personal fs.FS, dir string) ([]Change, error) {
	var changes []Change
	for _, agent := range []string{"claude-code", "antigravity"} {
		change, err := remove(target{path: repoInstructionPath(dir, agent)})
		if err != nil {
			return changes, err
		}
		changes = appendChange(changes, change)
	}
	if err := Unregister(places, dir); err != nil {
		return changes, err
	}
	rest, err := Sync(home, places, personal)
	return append(changes, rest...), err
}

// Prune forgets every registered repo whose .ai/me.yaml is gone and returns their folders.
func Prune(places Places) ([]string, error) {
	repos, err := loadRepos(places)
	if err != nil {
		return nil, err
	}
	var kept, gone []string
	for _, r := range repos {
		if r.profile == nil {
			gone = append(gone, r.dir)
		} else {
			kept = append(kept, r.dir)
		}
	}
	return gone, saveRepos(places, kept)
}

func appendChange(changes []Change, change Change) []Change {
	if change.What == "" {
		return changes
	}
	return append(changes, change)
}

// owned is what a target path holds: dashcrew's text, someone else's file, or nothing.
type owned struct {
	text    string
	owned   bool
	foreign bool
	// file is the whole Codex AGENTS.md; begin and end bound the block in it.
	file       string
	begin, end int
}

func readOwned(t target) (owned, error) {
	data, err := os.ReadFile(t.path)
	if errors.Is(err, fs.ErrNotExist) {
		return owned{}, nil
	}
	if err != nil {
		return owned{}, err
	}
	file := string(data)
	if !t.block {
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

func write(t target) (Change, error) {
	current, err := readOwned(t)
	if err != nil {
		return Change{}, err
	}
	if current.foreign {
		what := "kept, not written by dashcrew"
		if t.block {
			what = "kept, " + codexBegin + " has no " + codexEnd
		}
		return Change{t.path, what, true}, nil
	}
	if current.owned && current.text == t.text {
		return Change{t.path, "unchanged", false}, nil
	}
	out := t.text
	if t.block {
		block := codexBegin + "\n" + t.text + codexEnd
		switch {
		case current.owned:
			out = current.file[:current.begin] + block + current.file[current.end:]
		case strings.TrimSpace(current.file) == "":
			out = block + "\n"
		default:
			out = strings.TrimRight(current.file, "\n") + "\n\n" + block + "\n"
		}
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return Change{}, err
	}
	if err := os.WriteFile(t.path, []byte(out), 0o644); err != nil {
		return Change{}, err
	}
	if current.owned {
		return Change{t.path, "updated", false}, nil
	}
	return Change{t.path, "written", false}, nil
}

func remove(t target) (Change, error) {
	current, err := readOwned(t)
	if err != nil || !current.owned {
		return Change{}, err
	}
	if !t.block {
		return Change{t.path, "removed", false}, os.Remove(t.path)
	}
	rest := strings.TrimRight(current.file[:current.begin], "\n") + strings.TrimRight(current.file[current.end:], "\n")
	if strings.TrimSpace(rest) == "" {
		return Change{t.path, "removed", false}, os.Remove(t.path)
	}
	return Change{t.path, "block removed", false}, os.WriteFile(t.path, []byte(rest+"\n"), 0o644)
}

// ---- personal skills ----

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
