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
	"github.com/fahnaladitia/dashcrew/internal/filemode"
)

// Adopt copies a skill into `<projectDir>/.ai/skills/<name>/` and writes its meta.yaml from the SKILL.md front matter.
// source is the name of a skill in `<home>/.dashcrew/skills/`, or a local folder holding a SKILL.md (the skill is
// named after the folder). Its supporting files come along under the project skill limits. An adopted skill is
// optional unless required. It returns the skill's name and the hidden files it left out.
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
	files, err := catalog.SkillFolderFiles(os.DirFS(folder), ".")
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", folder, err)
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
	if err := writeSkillFolder(target, meta, data, files); err != nil {
		_ = os.RemoveAll(target)
		return "", nil, err
	}
	return name, hiddenFiles(folder), nil
}

// writeSkillFolder creates the adopted skill folder: meta.yaml, SKILL.md and the supporting files with their execute bits.
func writeSkillFolder(target, meta string, skill []byte, files []catalog.SkillFile) error {
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "meta.yaml"), []byte(meta), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), skill, 0o644); err != nil {
		return err
	}
	for _, file := range files {
		full := filepath.Join(target, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(file.Content), 0o644); err != nil {
			return err
		}
		if err := filemode.SetExecutable(full, file.Executable); err != nil {
			return err
		}
	}
	return nil
}

// hiddenFiles are the hidden files and folders anywhere in a skill folder, slash-separated, which adopt leaves
// out; a hidden folder is named once, not file by file.
func hiddenFiles(folder string) []string {
	var out []string
	_ = filepath.WalkDir(folder, func(file string, entry fs.DirEntry, err error) error {
		if err != nil || file == folder || !strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
		if relative, err := filepath.Rel(folder, file); err == nil {
			out = append(out, filepath.ToSlash(relative))
		}
		if entry.IsDir() {
			return fs.SkipDir
		}
		return nil
	})
	return out
}

func quoted(s string) string {
	text, _ := dartjson.Encode(s)
	return text
}
