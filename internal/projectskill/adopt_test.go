package projectskill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
)

func write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// project is a temp project root with its .ai/project.yaml.
func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".ai", "project.yaml"), "toolkit: 1\n")
	return dir
}

const outside = "---\nname: ui-slop\ndescription: \"Catch generic UI: gradients, emoji.\"\nwhen_to_use: When a screen is built.\n---\n# UI slop\n\nKeep {{braces}}.\n"

func TestAdoptAPersonalSkillAsOptional(t *testing.T) {
	home, project := t.TempDir(), project(t)
	write(t, filepath.Join(home, ".dashcrew", "skills", "ui-slop", "SKILL.md"), outside)

	name, skipped, err := Adopt(project, home, "ui-slop", false)
	if err != nil {
		t.Fatal(err)
	}

	if name != "ui-slop" || len(skipped) != 0 {
		t.Errorf("name %q skipped %v", name, skipped)
	}
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	overlaid, err := c.WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}
	skill, ok := overlaid.ProjectSkill("ui-slop")
	if !ok || !skill.Optional || skill.Meta.Description != "Catch generic UI: gradients, emoji." || *skill.Meta.WhenToUse != "When a screen is built." {
		t.Fatalf("adopted %+v %v", skill, ok)
	}
	if skill.Body != "# UI slop\n\nKeep {{braces}}.\n" {
		t.Errorf("body %q", skill.Body)
	}
}

func TestAdoptAFolderAsRequired(t *testing.T) {
	home, project, folder := t.TempDir(), project(t), filepath.Join(t.TempDir(), "api-style")
	write(t, filepath.Join(folder, "SKILL.md"), "---\ndescription: Team API style.\n---\n# API style\n")
	write(t, filepath.Join(folder, "references", "checklist.md"), "x\n")

	_, skipped, err := Adopt(project, home, folder, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(skipped) != 1 || skipped[0] != "references/checklist.md" {
		t.Errorf("skipped %v, want the supporting file reported", skipped)
	}

	meta, err := os.ReadFile(filepath.Join(project, ".ai", "skills", "api-style", "meta.yaml"))
	if err != nil || !strings.Contains(string(meta), "optional: false\n") || strings.Contains(string(meta), "when_to_use") {
		t.Errorf("meta.yaml %q %v", meta, err)
	}
}

func TestAdoptRefuses(t *testing.T) {
	for _, c := range []struct {
		name, source, skill, want string
		existing                  bool
	}{
		{"missing personal skill", "ghost", "", "no ~/.dashcrew/skills/ghost/SKILL.md", false},
		{"no description", "plain", "# no front matter\n", "SKILL.md needs a description", false},
		{"a dashcrew skill name", "branching", outside, "dashcrew already has a skill or workflow named branching", false},
		{"an existing project skill", "ui-slop", outside, ".ai/skills/ui-slop already exists", true},
		{"a bad name", "Bad_Name", outside, "a skill name is lowercase", false},
		{"a URL", "https://example.com/skill", "", "a local folder or a skill in ~/.dashcrew/skills", false},
		{"an empty block description", "quotes", "---\ndescription: >-\n\n---\nx\n", "SKILL.md needs a description", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			home, project := t.TempDir(), project(t)
			if c.skill != "" {
				write(t, filepath.Join(home, ".dashcrew", "skills", c.source, "SKILL.md"), c.skill)
			}
			if c.existing {
				write(t, filepath.Join(project, ".ai", "skills", c.source, "meta.yaml"), "description: x\n")
				write(t, filepath.Join(project, ".ai", "skills", c.source, "SKILL.md"), "x\n")
			}

			_, _, err := Adopt(project, home, c.source, false)

			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want %q", err, c.want)
			}
		})
	}
}

func TestAdoptNeedsTheProjectRoot(t *testing.T) {
	home, notRoot := t.TempDir(), t.TempDir()
	write(t, filepath.Join(home, ".dashcrew", "skills", "ui-slop", "SKILL.md"), outside)

	_, _, err := Adopt(notRoot, home, "ui-slop", false)

	if err == nil || !strings.Contains(err.Error(), "no .ai/project.yaml") {
		t.Errorf("got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(notRoot, ".ai")); statErr == nil {
		t.Error("adopt wrote .ai/ outside a project root")
	}
}
