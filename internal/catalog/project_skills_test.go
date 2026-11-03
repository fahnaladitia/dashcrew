package catalog

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestProjectSkillsLoadFromDotAiWithTheirBodyVerbatim(t *testing.T) {
	root, project := projectPacksRoot(t)
	write(t, project, ".ai/skills/ui-slop/meta.yaml", "description: Catch generic UI.\nwhen_to_use: When a screen is built.\noptional: true\n")
	write(t, project, ".ai/skills/ui-slop/SKILL.md", "---\nname: ui-slop\ndescription: outside copy\n---\n\n# UI slop\n\nKeep {{braces}} as they are.\n")
	write(t, project, ".ai/skills/api-style/meta.yaml", "description: Team API style.\nreview: true\n")
	write(t, project, ".ai/skills/api-style/SKILL.md", "# API style\n")
	builtin := load(t, root)

	catalog, err := builtin.WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}

	if len(catalog.ProjectSkills) != 2 {
		t.Fatalf("project skills %+v", catalog.ProjectSkills)
	}
	api, slop := catalog.ProjectSkills[0], catalog.ProjectSkills[1]
	if api.Name != "api-style" || api.Optional || !api.Meta.Review || api.Body != "# API style\n" {
		t.Errorf("api-style %+v", api)
	}
	if slop.Name != "ui-slop" || !slop.Optional || slop.Meta.Description != "Catch generic UI." || *slop.Meta.WhenToUse != "When a screen is built." {
		t.Errorf("ui-slop %+v", slop)
	}
	if slop.Body != "# UI slop\n\nKeep {{braces}} as they are.\n" {
		t.Errorf("ui-slop body %q", slop.Body)
	}
	if got, ok := catalog.ProjectSkill("ui-slop"); !ok || got.Name != "ui-slop" {
		t.Errorf("ProjectSkill lookup %+v %v", got, ok)
	}
	if resolved := catalog.ResolvedFor("flutter"); len(resolved.ProjectSkills) != 2 {
		t.Error("ResolvedFor drops the project skills")
	}
	if len(builtin.ProjectSkills) != 0 {
		t.Error("built-in catalog changed")
	}
}

func TestProjectSkillErrorsNameTheirPath(t *testing.T) {
	for _, c := range []struct {
		name, folder, meta, skill, source, message string
	}{
		{"review cannot be optional", "checks", "description: d\nreview: true\noptional: true\n", "x\n", ".ai/skills/checks/meta.yaml", "cannot be optional"},
		{"optional must be a boolean", "checks", "description: d\noptional: yes please\n", "x\n", ".ai/skills/checks/meta.yaml", "optional: expected true or false"},
		{"needs SKILL.md", "checks", "description: d\n", "", ".ai/skills/checks/SKILL.md", "missing"},
		{"needs meta.yaml", "checks", "", "x\n", ".ai/skills/checks/meta.yaml", "missing"},
		{"name format", "Checks_1", "description: d\n", "x\n", ".ai/skills/Checks_1", "lowercase"},
		{"clashes with a catalog skill", "branching", "description: d\n", "x\n", ".ai/skills/branching", "dashcrew already has"},
		{"clashes with a catalog workflow", "verify", "description: d\n", "x\n", ".ai/skills/verify", "dashcrew already has"},
		{"clashes with a pack skill", "bloc-patterns", "description: d\n", "x\n", ".ai/skills/bloc-patterns", "dashcrew already has"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, project := projectPacksRoot(t)
			write(t, root, "core/skills/branching/meta.yaml", "description: b\n")
			write(t, root, "core/workflows/verify/meta.yaml", "description: v\n")
			write(t, root, "packs/state/bloc/skills/bloc-patterns.yaml", "description: p\n")
			if c.meta != "" {
				write(t, project, ".ai/skills/"+c.folder+"/meta.yaml", c.meta)
			}
			if c.skill != "" {
				write(t, project, ".ai/skills/"+c.folder+"/SKILL.md", c.skill)
			}

			_, err := load(t, root).WithProjectPacks(project)

			got := catalogError(t, err)
			if got.Source != c.source || !strings.Contains(got.Message, c.message) {
				t.Errorf("error %s: %s", got.Source, got.Message)
			}
		})
	}
}

func TestSkillFrontMatterReadsTopLevelYaml(t *testing.T) {
	for text, want := range map[string]string{
		"---\ndescription: Rules for the 'api' layer's 'dto'\n---\nx":     "Rules for the 'api' layer's 'dto'",
		"---\ndescription: \"Quoted: yes\"\n---\nx":                       "Quoted: yes",
		"---\ndescription: >-\n  Folded\n  text.\n---\nx":                 "Folded text.",
		"---\nmetadata:\n  description: nested\ndescription: top\n---\nx": "top",
		"no front matter": "",
		"---\ndescription: [not, a, string]\n---\nx": "",
	} {
		if got := SkillFrontMatter(text)["description"]; got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestProjectSkillsCarryTheirSupportingFiles(t *testing.T) {
	root, project := projectPacksRoot(t)
	write(t, project, ".ai/skills/api-style/meta.yaml", "description: d\n")
	write(t, project, ".ai/skills/api-style/SKILL.md", "# API style\n")
	write(t, project, ".ai/skills/api-style/references/checklist.md", "- one\n")
	write(t, project, ".ai/skills/api-style/scripts/check.sh", "#!/bin/sh\necho ok\n")
	write(t, project, ".ai/skills/api-style/.DS_Store", "junk")
	write(t, project, ".ai/skills/api-style/.git/config", "junk")
	if err := os.Chmod(filepath.Join(project, ".ai/skills/api-style/scripts/check.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	catalog, err := load(t, root).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}

	skill, _ := catalog.ProjectSkill("api-style")
	want := []SkillFile{
		{Path: "references/checklist.md", Content: "- one\n"},
		{Path: "scripts/check.sh", Content: "#!/bin/sh\necho ok\n", Executable: runtime.GOOS != "windows"},
	}
	if !reflect.DeepEqual(skill.Files, want) {
		t.Errorf("files %+v", skill.Files)
	}
}

func TestProjectSkillFileLimits(t *testing.T) {
	for _, c := range []struct {
		name    string
		prepare func(t *testing.T, folder string)
		source  string
		message string
	}{
		{"a symlink", func(t *testing.T, folder string) {
			if err := os.Symlink("/etc/hosts", filepath.Join(folder, "hosts")); err != nil {
				t.Skip("no symlinks here")
			}
		}, ".ai/skills/api-style/hosts", "a symlink"},
		{"a file over 1 MiB", func(t *testing.T, folder string) {
			write(t, folder, "big.bin", strings.Repeat("x", 1<<20+1))
		}, ".ai/skills/api-style/big.bin", "over 1 MiB"},
		{"more than 50 files", func(t *testing.T, folder string) {
			for i := 0; i < 51; i++ {
				write(t, folder, fmt.Sprintf("refs/%02d.md", i), "x\n")
			}
		}, ".ai/skills/api-style", "more than 50 supporting files"},
	} {
		t.Run(c.name, func(t *testing.T) {
			root, project := projectPacksRoot(t)
			write(t, project, ".ai/skills/api-style/meta.yaml", "description: d\n")
			write(t, project, ".ai/skills/api-style/SKILL.md", "# API style\n")
			c.prepare(t, filepath.Join(project, ".ai", "skills", "api-style"))

			_, err := load(t, root).WithProjectPacks(project)

			got := catalogError(t, err)
			if got.Source != c.source || !strings.Contains(got.Message, c.message) {
				t.Errorf("error %s: %s", got.Source, got.Message)
			}
		})
	}
}
