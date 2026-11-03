package sync

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestARequiredProjectSkillWritesItsFilesAndKeepsScriptsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no execute bits on Windows")
	}
	s, project := newProject(t, "toolkit: 1\nagents: [claude-code, codex]\n")
	write(t, project, ".ai/skills/api-style/meta.yaml", "description: Team API style.\n")
	write(t, project, ".ai/skills/api-style/SKILL.md", "# API style\n")
	write(t, project, ".ai/skills/api-style/references/checklist.md", "- one\n")
	write(t, project, ".ai/skills/api-style/scripts/check.sh", "#!/bin/sh\necho ok\n")
	if err := os.Chmod(filepath.Join(project, ".ai/skills/api-style/scripts/check.sh"), 0o755); err != nil {
		t.Fatal(err)
	}

	run(t, s, false, false)

	locked := map[string]bool{}
	for _, path := range lockedPaths(t, project) {
		locked[path] = true
	}
	for _, dir := range []string{".claude/skills/api-style/", ".agents/skills/api-style/"} {
		if read(t, project, dir+"references/checklist.md") != "- one\n" || !locked[dir+"references/checklist.md"] {
			t.Errorf("%sreferences/checklist.md not written or not locked", dir)
		}
		info, err := os.Stat(filepath.Join(project, filepath.FromSlash(dir+"scripts/check.sh")))
		if err != nil || info.Mode().Perm()&0o111 == 0 {
			t.Errorf("%sscripts/check.sh is not executable: %v %v", dir, info, err)
		}
		if info, err := os.Stat(filepath.Join(project, filepath.FromSlash(dir+"references/checklist.md"))); err != nil || info.Mode().Perm()&0o111 != 0 {
			t.Errorf("%sreferences/checklist.md is executable", dir)
		}
	}
}

func TestASkippedSkillFileIsAWarningAndNotWritten(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\n")
	write(t, project, ".ai/skills/api-style/meta.yaml", "description: Team API style.\n")
	write(t, project, ".ai/skills/api-style/SKILL.md", "# API style\n")
	write(t, project, ".ai/skills/api-style/big.bin", strings.Repeat("x", 1<<20+1))

	result := run(t, s, false, false)

	found := false
	for _, warning := range result.Warnings {
		found = found || strings.Contains(warning, ".ai/skills/api-style/big.bin: over 1 MiB")
	}
	if !found {
		t.Errorf("warnings %v", result.Warnings)
	}
	if _, err := os.Stat(filepath.Join(project, ".claude/skills/api-style/big.bin")); err == nil {
		t.Error("the skipped file was written")
	}
	if read(t, project, ".claude/skills/api-style/SKILL.md") == "" {
		t.Error("the skill itself was not written")
	}
}

func TestSyncDoesNotWarnAboutAnOptionalSkillItNeverWrites(t *testing.T) {
	s, project := newProject(t, "toolkit: 1\nagents: [claude-code]\n")
	write(t, project, ".ai/skills/ui-slop/meta.yaml", "description: d\noptional: true\n")
	write(t, project, ".ai/skills/ui-slop/SKILL.md", "# UI slop\n")
	write(t, project, ".ai/skills/ui-slop/big.bin", strings.Repeat("x", 1<<20+1))

	for _, warning := range run(t, s, false, false).Warnings {
		if strings.Contains(warning, "ui-slop") {
			t.Errorf("warning about an optional skill: %s", warning)
		}
	}
}
