package catalog

import (
	"io/fs"
	"path"
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
}

// SkillFile is a supporting file of a project skill, copied verbatim.
type SkillFile struct {
	// Path is relative to the skill folder, slash-separated.
	Path    string
	Content string
	// Executable is set when the source file has an execute bit; the written file gets one too.
	Executable bool
}

// Supporting file limits: a skill folder carries at most MaxSkillFiles files of at most MaxSkillFileBytes each.
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
		files, err := supportingFiles(fsys, sub)
		if err != nil {
			return nil, err
		}
		out = append(out, ProjectSkill{Name: name, Meta: meta, Optional: optional, Body: skillBody(string(data)), Files: files})
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

// SkillFolderFiles are the supporting files of the skill folder dir in fsys, under the same rules and limits as a
// project skill; `dashcrew skill adopt` copies them.
func SkillFolderFiles(fsys fs.FS, dir string) ([]SkillFile, error) { return supportingFiles(fsys, dir) }

// supportingFiles reads every file of a skill folder but SKILL.md and meta.yaml. Hidden files and folders are
// skipped; a symlink, a file over MaxSkillFileBytes or more than MaxSkillFiles files is an error.
func supportingFiles(fsys fs.FS, dir string) ([]SkillFile, error) {
	var out []SkillFile
	err := fs.WalkDir(fsys, dir, func(file string, entry fs.DirEntry, err error) error {
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
		if entry.Type()&fs.ModeSymlink != 0 {
			return fail(file, "a symlink; copy the file into the skill folder instead")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fail(file, "not a regular file (a pipe, socket or device); a skill carries plain files")
		}
		relative := strings.TrimPrefix(file, dir+"/")
		if relative == "SKILL.md" || relative == "meta.yaml" {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > MaxSkillFileBytes {
			return fail(file, "over 1 MiB; a skill carries small reference files and scripts")
		}
		if len(out) == MaxSkillFiles {
			return fail(dir, "more than 50 supporting files")
		}
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return err
		}
		out = append(out, SkillFile{Path: relative, Content: string(data), Executable: info.Mode().Perm()&0o111 != 0})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}
