package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryAdapterWritesARequiredProjectSkillAndNoOptionalOne(t *testing.T) {
	project := t.TempDir()
	for path, text := range map[string]string{
		".ai/skills/api-style/meta.yaml":               "description: Team API style.\nwhen_to_use: When an endpoint changes.\n",
		".ai/skills/api-style/SKILL.md":                "# API style\n",
		".ai/skills/api-style/references/checklist.md": "- one\n",
		".ai/skills/ui-slop/meta.yaml":                 "description: Catch generic UI.\noptional: true\n",
		".ai/skills/ui-slop/SKILL.md":                  "# UI slop\n",
	} {
		full := filepath.Join(project, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := shipped(t).WithProjectPacks(project)
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range SupportedAdapters {
		adapter, err := AdapterFor(agent, c, project, "flutter", nil)
		if err != nil {
			t.Fatal(err)
		}
		written := files(t, c, adapter, "toolkit: 1\nagents: ["+agent+"]\n")
		dir := ".agents/skills/"
		if agent == "claude-code" {
			dir = ".claude/skills/"
		}
		skill, ok := written[dir+"api-style/SKILL.md"]
		if !ok || !strings.Contains(skill, "Team API style.") || !strings.HasSuffix(skill, "# API style\n") {
			t.Errorf("%s api-style %q", agent, skill)
		}
		if got := written[dir+"api-style/references/checklist.md"]; got != "- one\n" {
			t.Errorf("%s api-style/references/checklist.md %q", agent, got)
		}
		if _, ok := written[dir+"ui-slop/SKILL.md"]; ok {
			t.Errorf("%s writes the optional ui-slop for everyone", agent)
		}
	}
}
