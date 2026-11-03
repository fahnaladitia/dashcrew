// Package projectskill adopts a skill into a project's `.ai/skills/`, from a member's own skills or a local folder.
package projectskill

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
)

// Adopt copies a skill into `<projectDir>/.ai/skills/<name>/` and writes its meta.yaml from the SKILL.md front matter.
// source is the name of a skill in `<home>/.dashcrew/skills/`, or a local folder holding a SKILL.md (the skill is
// named after the folder). An adopted skill is optional unless required. It returns the skill's name and the other
// files of the source folder, which are not carried: only SKILL.md reaches the agents.
func Adopt(projectDir, home, source string, required bool) (string, []string, error) {
	if strings.Contains(source, "://") {
		return "", nil, errors.New(source + ": adopt takes a local folder or a skill in ~/.dashcrew/skills, not a URL")
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".ai", "project.yaml")); err != nil {
		return "", nil, errors.New("no .ai/project.yaml in " + projectDir + "; run from the project root or pass -p")
	}
	folder, name := filepath.Join(home, ".dashcrew", "skills", source), source
	if strings.ContainsAny(source, `/\`) || strings.HasPrefix(source, ".") {
		folder, name = source, filepath.Base(filepath.Clean(source))
	}
	if !catalog.ValidProjectSkillName(name) {
		return "", nil, errors.New(name + ": a skill name is lowercase letters, digits and hyphens, starting with a letter")
	}
	data, err := os.ReadFile(filepath.Join(folder, "SKILL.md"))
	if errors.Is(err, fs.ErrNotExist) {
		if folder == source {
			return "", nil, errors.New("no " + filepath.Join(source, "SKILL.md"))
		}
		return "", nil, errors.New("no ~/.dashcrew/skills/" + name + "/SKILL.md")
	}
	if err != nil {
		return "", nil, err
	}
	values := catalog.SkillFrontMatter(string(data))
	description, whenToUse := values["description"], values["when_to_use"]
	if description == "" {
		return "", nil, errors.New("SKILL.md needs a description in its front matter (description: what the skill is)")
	}

	shipped, err := catalog.LoadEmbedded()
	if err != nil {
		return "", nil, err
	}
	current, err := shipped.WithProjectPacks(projectDir)
	if err != nil {
		return "", nil, err
	}
	if current.TakenSkillNames()[name] {
		return "", nil, errors.New("dashcrew already has a skill or workflow named " + name + "; rename the folder first")
	}
	target := filepath.Join(projectDir, filepath.FromSlash(catalog.ProjectSkillsDir), name)
	if _, err := os.Stat(target); err == nil {
		return "", nil, errors.New(filepath.ToSlash(filepath.Join(catalog.ProjectSkillsDir, name)) + " already exists; edit it there")
	}

	meta := "# Adopted by dashcrew skill adopt. optional: true writes it only for members who list it in .ai/me.yaml skills.\n" +
		"description: " + quoted(description) + "\n"
	if whenToUse != "" {
		meta += "when_to_use: " + quoted(whenToUse) + "\n"
	}
	meta += fmt.Sprintf("optional: %t\n", !required)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(target, "meta.yaml"), []byte(meta), 0o644); err != nil {
		return "", nil, err
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), data, 0o644); err != nil {
		return "", nil, err
	}
	return name, otherFiles(folder), nil
}

// otherFiles are the files of a skill folder besides SKILL.md, slash-separated and sorted.
func otherFiles(folder string) []string {
	var out []string
	_ = filepath.WalkDir(folder, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if relative, err := filepath.Rel(folder, path); err == nil && relative != "SKILL.md" {
			out = append(out, filepath.ToSlash(relative))
		}
		return nil
	})
	return out
}

func quoted(s string) string {
	text, _ := dartjson.Encode(s)
	return text
}
