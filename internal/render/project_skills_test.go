package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
)

// withProjectSkills is the shipped catalog over a temp project holding one required and one optional skill.
func withProjectSkills(t *testing.T) *catalog.Catalog {
	t.Helper()
	project := t.TempDir()
	for path, text := range map[string]string{
		".ai/skills/api-style/meta.yaml": "description: Team API style.\nwhen_to_use: When an endpoint changes.\n",
		".ai/skills/api-style/SKILL.md":  "---\nname: api-style\n---\n# API style\n\nKeep {{braces}}.\n",
		".ai/skills/ui-slop/meta.yaml":   "description: Catch generic UI.\noptional: true\n",
		".ai/skills/ui-slop/SKILL.md":    "# UI slop\n",
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
	return c
}

func TestARequiredProjectSkillRendersVerbatimAndJoinsTheIndex(t *testing.T) {
	targets := renderAll(t, withProjectSkills(t), minimal)

	if got := targetContent(t, targets, "skill/api-style"); got != "# API style\n\nKeep {{braces}}.\n" {
		t.Errorf("api-style %q", got)
	}
	if !strings.Contains(targetContent(t, targets, "agents_md"), "- `api-style`: When an endpoint changes.\n") {
		t.Error("the skill index lacks api-style")
	}
}

func TestAnOptionalProjectSkillStaysOutOfTheTeamFiles(t *testing.T) {
	plain := targetContent(t, renderShipped(t, minimal), "agents_md")
	targets := renderAll(t, withProjectSkills(t), minimal)

	if targetNames(targets)["skill/ui-slop"] {
		t.Error("sync would write the optional ui-slop for everyone")
	}
	agentsMd := targetContent(t, targets, "agents_md")
	if strings.Contains(agentsMd, "ui-slop") {
		t.Error("the skill index lists the optional ui-slop")
	}
	if want := len(plain) + len("- `api-style`: When an endpoint changes.\n"); len(agentsMd) != want {
		t.Errorf("AGENTS.md grew by %d bytes, want only the api-style index line", len(agentsMd)-len(plain))
	}
}
