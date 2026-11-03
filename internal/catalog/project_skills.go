package catalog

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// ProjectSkillsDir holds a project's own skills, one folder per skill.
const ProjectSkillsDir = ".ai/skills"

// ProjectSkill is `.ai/skills/<name>/`: a team skill whose SKILL.md body is copied verbatim, never rendered as
// Mustache, so a skill written elsewhere keeps every character.
type ProjectSkill struct {
	Name string
	Meta DocMeta
	// Optional skills are not written by sync; `dashcrew me sync` writes them only into the clone of a member who
	// turns them on.
	Optional bool
	// Body is SKILL.md without its front matter, ending in one newline.
	Body string
	// Files are the skill folder's other files (not SKILL.md or meta.yaml), sorted by path; each agent gets them
	// next to its SKILL.md.
	Files []SkillFile
	// Skipped are the files left out for a limit, each as `<path>: <reason>`; sync and doctor warn with them.
	Skipped []string
}

// SkillFile is a supporting file of a project skill, copied verbatim.
type SkillFile struct {
	// Path is relative to the skill folder, slash-separated.
	Path    string
	Content string
	// Executable is set when the source file has an execute bit; the written file gets one too.
	Executable bool
}

// Supporting file limits: a skill folder carries at most MaxSkillFiles files of at most MaxSkillFileBytes each; a
// file past them is skipped, not fatal.
const (
	MaxSkillFiles     = 50
	MaxSkillFileBytes = 1 << 20
)

var projectSkillName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ValidProjectSkillName is lowercase letters, digits and hyphens, starting with a letter.
func ValidProjectSkillName(name string) bool { return projectSkillName.MatchString(name) }

// ProjectSkill returns the project skill called name.
func (c *Catalog) ProjectSkill(name string) (ProjectSkill, bool) {
	for _, skill := range c.ProjectSkills {
		if skill.Name == name {
			return skill, true
		}
	}
	return ProjectSkill{}, false
}

// TakenSkillNames are the folder names dashcrew writes under an agent's skills folder: catalog skills and
// workflows of every platform, and every pack skill. A project skill may not use one.
func (c *Catalog) TakenSkillNames() map[string]bool {
	taken := map[string]bool{}
	addPacks := func(packs Packs) {
		for _, family := range packs.Keys() {
			options := packs.Value(family)
			for _, option := range options.Keys() {
				for name := range options.Value(option).Skills {
					taken[name] = true
				}
			}
		}
	}
	for name := range c.Skills {
		taken[name] = true
	}
	for name := range c.Workflows {
		taken[name] = true
	}
	addPacks(c.Packs)
	for _, platform := range c.Platforms {
		for name := range platform.Skills {
			taken[name] = true
		}
		for name := range platform.Workflows {
			taken[name] = true
		}
		if platform.Packs != nil {
			addPacks(platform.Packs)
		}
	}
	return taken
}

// projectSkillsIn loads every `<dir>/<name>/` with a meta.yaml and a SKILL.md, sorted by name.
func projectSkillsIn(fsys fs.FS, dir string, taken map[string]bool) ([]ProjectSkill, error) {
	if !isDir(fsys, dir) {
		return nil, nil
	}
	var out []ProjectSkill
	for _, sub := range sortedDirs(fsys, dir) {
		name := path.Base(sub)
		if !projectSkillName.MatchString(name) {
			return nil, fail(sub, "a skill name is lowercase letters, digits and hyphens, starting with a letter")
		}
		if taken[name] {
			return nil, fail(sub, "dashcrew already has a skill or workflow named "+name+"; rename the folder")
		}
		manifest, file := sub+"/meta.yaml", sub+"/SKILL.md"
		if !isFile(fsys, manifest) {
			return nil, fail(manifest, "missing meta.yaml (description, when_to_use, review, optional)")
		}
		if !isFile(fsys, file) {
			return nil, fail(file, "missing SKILL.md")
		}
		meta, err := loadDocMeta(fsys, manifest)
		if err != nil {
			return nil, err
		}
		optional, err := optionalFlag(fsys, manifest)
		if err != nil {
			return nil, err
		}
		if optional && meta.Review {
			return nil, fail(manifest, "a review skill is the team's review checklist and cannot be optional")
		}
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, err
		}
		files, skipped, err := supportingFiles(fsys, sub)
		if err != nil {
			return nil, err
		}
		out = append(out, ProjectSkill{Name: name, Meta: meta, Optional: optional, Body: skillBody(string(data)), Files: files, Skipped: skipped})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func optionalFlag(fsys fs.FS, manifest string) (bool, error) {
	parsed, err := readYaml(fsys, manifest)
	if err != nil {
		return false, err
	}
	yaml, _ := parsed.(*omap.Map[any])
	if yaml == nil || yaml.Value("optional") == nil {
		return false, nil
	}
	optional, ok := yaml.Value("optional").(bool)
	if !ok {
		return false, fail(manifest, "optional: expected true or false")
	}
	return optional, nil
}

// skillBody drops a SKILL.md front matter and the blank lines after it; each adapter writes its own front matter.
func skillBody(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		if _, body, found := strings.Cut(rest, "\n---\n"); found {
			text = body
		}
	}
	return strings.TrimRight(strings.TrimLeft(text, "\n"), " \t\n") + "\n"
}

// SkillFrontMatter is the top-level string values of a SKILL.md front matter, read as YAML; empty without one or
// when it does not parse.
func SkillFrontMatter(text string) map[string]string {
	out := map[string]string{}
	rest, ok := strings.CutPrefix(strings.ReplaceAll(text, "\r\n", "\n"), "---\n")
	if !ok {
		return out
	}
	head, _, found := strings.Cut(rest, "\n---\n")
	if !found {
		return out
	}
	parsed, err := yamlutil.Parse(head + "\n")
	if err != nil {
		return out
	}
	values, ok := parsed.(*omap.Map[any])
	if !ok {
		return out
	}
	for _, key := range values.Keys() {
		if value, ok := values.Value(key).(string); ok {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

// LoadProjectSkills reads `<projectDir>/.ai/skills` alone, without the project's packs and platforms; taken are the
// names a project skill may not use. `dashcrew me sync` uses it; sync and doctor load the whole `.ai/` instead.
func LoadProjectSkills(projectDir string, taken map[string]bool) ([]ProjectSkill, error) {
	return projectSkillsIn(os.DirFS(projectDir), ProjectSkillsDir, taken)
}

// SkillFolderFiles are the supporting files of the skill folder dir in fsys and the ones skipped for a limit, under
// the same rules as a project skill; `dashcrew skill adopt` copies them.
func SkillFolderFiles(fsys fs.FS, dir string) ([]SkillFile, []string, error) {
	return supportingFiles(fsys, dir)
}

// supportingFiles reads the files of a skill folder but SKILL.md and meta.yaml. Hidden files and folders are left
// out silently. A symlink, a file that is not regular and a file over MaxSkillFileBytes are left out and listed in
// skipped as `<path>: <reason>`; of the rest, the first MaxSkillFiles by full path are read and the others listed
// in one entry. A symlinked SKILL.md or meta.yaml is an error: dashcrew never follows one.
func supportingFiles(fsys fs.FS, dir string) (files []SkillFile, skipped []string, err error) {
	type candidate struct {
		file, relative string
		executable     bool
	}
	var candidates []candidate
	err = fs.WalkDir(fsys, dir, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if file == dir {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		relative := strings.TrimPrefix(file, dir+"/")
		symlink := entry.Type()&fs.ModeSymlink != 0
		if relative == "SKILL.md" || relative == "meta.yaml" {
			if symlink {
				return fail(file, "a symlink, which dashcrew never follows; put the file itself in the skill folder")
			}
			return nil
		}
		switch {
		case symlink:
			skipped = append(skipped, file+": a symlink, copy the file into the skill folder")
			return nil
		case entry.IsDir():
			return nil
		case !entry.Type().IsRegular():
			skipped = append(skipped, file+": not a regular file (a pipe, socket or device)")
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxSkillFileBytes {
			skipped = append(skipped, file+": over 1 MiB")
			return nil
		}
		candidates = append(candidates, candidate{file, relative, info.Mode().Perm()&0o111 != 0})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].relative < candidates[j].relative })
	if len(candidates) > MaxSkillFiles {
		var past []string
		for _, c := range candidates[MaxSkillFiles:] {
			past = append(past, c.relative)
		}
		if len(past) > 5 {
			past = append(past[:5], "...")
		}
		skipped = append(skipped, fmt.Sprintf("%s: %d files past the first %d (%s)", dir, len(candidates)-MaxSkillFiles, MaxSkillFiles, strings.Join(past, ", ")))
		candidates = candidates[:MaxSkillFiles]
	}
	for _, c := range candidates {
		data, err := fs.ReadFile(fsys, c.file)
		if err != nil {
			return nil, nil, err
		}
		files = append(files, SkillFile{Path: c.relative, Content: string(data), Executable: c.executable})
	}
	return files, skipped, nil
}

// Warnings are the files the skill left out, one line each, as sync, doctor and me sync print them.
func (s ProjectSkill) Warnings() []string {
	out := make([]string, len(s.Skipped))
	for i, skipped := range s.Skipped {
		out[i] = "project skill " + s.Name + " leaves out " + skipped
	}
	return out
}

// ProjectSkillNames are the skill and workflow names a project's own packs and platforms define, read from the
// folder layout alone (`.ai/packs/<family>/<option>/skills/<name>.yaml`, `.ai/platforms/<p>/skills|workflows/<name>/`).
// `dashcrew me sync` adds them to the taken names without parsing those packs and platforms.
func ProjectSkillNames(projectDir string) map[string]bool {
	out := map[string]bool{}
	for _, pattern := range []string{".ai/packs/*/*/skills/*.yaml", ".ai/platforms/*/skills/*", ".ai/platforms/*/workflows/*"} {
		matches, _ := filepath.Glob(filepath.Join(projectDir, filepath.FromSlash(pattern)))
		for _, match := range matches {
			out[strings.TrimSuffix(filepath.Base(match), ".yaml")] = true
		}
	}
	return out
}
