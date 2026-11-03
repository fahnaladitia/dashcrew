//go:build !windows

package catalog

import (
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestAFifoInASkillFolderIsRefusedNotRead(t *testing.T) {
	root, project := projectPacksRoot(t)
	write(t, project, ".ai/skills/api-style/meta.yaml", "description: d\n")
	write(t, project, ".ai/skills/api-style/SKILL.md", "# API style\n")
	if err := syscall.Mkfifo(filepath.Join(project, ".ai/skills/api-style/pipe"), 0o644); err != nil {
		t.Skip("mkfifo: " + err.Error())
	}

	_, err := load(t, root).WithProjectPacks(project)

	if got := catalogError(t, err); got.Source != ".ai/skills/api-style/pipe" || !strings.Contains(got.Message, "not a regular file") {
		t.Errorf("error %s: %s", got.Source, got.Message)
	}
}
