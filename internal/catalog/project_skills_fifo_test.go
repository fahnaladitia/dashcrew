//go:build !windows

package catalog

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestAFifoInASkillFolderIsSkippedNotRead(t *testing.T) {
	root, project := projectPacksRoot(t)
	write(t, project, ".ai/skills/api-style/meta.yaml", "description: d\n")
	write(t, project, ".ai/skills/api-style/SKILL.md", "# API style\n")
	if err := syscall.Mkfifo(filepath.Join(project, ".ai/skills/api-style/pipe"), 0o644); err != nil {
		t.Skip("mkfifo: " + err.Error())
	}

	catalog, err := load(t, root).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}

	if skill, _ := catalog.ProjectSkill("api-style"); len(skill.Skipped) != 1 || !strings.HasPrefix(skill.Skipped[0], ".ai/skills/api-style/pipe: not a regular file") {
		t.Errorf("skipped %v", skill.Skipped)
	}
}
