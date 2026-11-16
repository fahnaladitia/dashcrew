package doctor

import (
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/sync"
)

const threeAgents = "toolkit: 1\nagents: [claude-code, codex, antigravity]\n"

func (f *fixture) parts(agent string) (names []string, outside []string) {
	f.t.Helper()
	parts, outside := Doctor{Catalog: shippedCatalog, ProjectDir: f.project}.alwaysOnParts(agent)
	for _, p := range parts {
		names = append(names, p.path)
	}
	return names, outside
}

func has(list []string, item string) bool {
	for _, x := range list {
		if x == item {
			return true
		}
	}
	return false
}

// indexLines is a memory index of n lines, each width characters long.
func indexLines(n, width int) string {
	var b strings.Builder
	b.WriteString("# Memory Index\n\n")
	for i := 0; i < n; i++ {
		line := "- [Fact](fact-" + string(rune('a'+i%26)) + ".md) - "
		b.WriteString(line + strings.Repeat("x", width-len(line)) + "\n")
	}
	return b.String()
}

func TestAlwaysOn(t *testing.T) {
	t.Run("a fresh sync is under the budget for every agent", func(t *testing.T) {
		f := setup(t, threeAgents)
		f.sync()

		expect(t, f.doctor(), "always-on", OK, "claude-code ", "codex ", "antigravity ", "of 40.0 KiB")
		if _, ok := f.doctor()["memory-index"]; ok {
			t.Error("memory-index without an index")
		}
	})

	t.Run("a large memory index puts every agent over, largest part first", func(t *testing.T) {
		f := setup(t, threeAgents)
		f.sync()
		f.write(".ai/memory/MEMORY.md", indexLines(250, 120))

		check := f.doctor()["always-on"]
		expect(t, map[string]Check{"always-on": check}, "always-on", Warn, "claude-code ", "over 40.0 KiB: .ai/memory/MEMORY.md 29.", ", AGENTS.md ")
		if strings.Count(check.Detail, "over 40.0 KiB") != 3 {
			t.Errorf("want three agents over: %s", check.Detail)
		}
	})

	t.Run("claude imports are followed five deep, once each, inside the project", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\n")
		f.sync()
		f.write("CLAUDE.md", "@AGENTS.md\n@docs/d1.md\n@~/personal.md\nmail me at a@b.md\n```\n@docs/code.md\n```\nsee `@docs/code.md`\n")
		for i := 1; i <= 7; i++ {
			// An import resolves from the importing file's folder.
			next := "d" + string(rune('0'+i+1)) + ".md"
			f.write("docs/d"+string(rune('0'+i))+".md", "@"+next+" and back to @d1.md\n")
		}
		f.write("docs/code.md", "code\n")

		names, outside := f.parts("claude-code")
		for _, want := range []string{"CLAUDE.md", "AGENTS.md", "docs/d1.md", "docs/d5.md"} {
			if !has(names, want) {
				t.Errorf("missing %s in %v", want, names)
			}
		}
		for _, unwanted := range []string{"docs/d6.md", "docs/code.md", "b.md"} {
			if has(names, unwanted) {
				t.Errorf("%s counted in %v", unwanted, names)
			}
		}
		if len(names) != len(unique(names)) {
			t.Errorf("a file counted twice: %v", names)
		}
		if strings.Join(outside, ",") != "~/personal.md" {
			t.Errorf("outside %v", outside)
		}
	})

	t.Run("claude rules count unless scoped by paths", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\n")
		f.sync()
		f.write(".claude/rules/always.md", "# Always\n")
		f.write(".claude/rules/flutter/state.md", "---\npaths:\n  - \"lib/**/bloc/**\"\n---\n# State\n")

		names, _ := f.parts("claude-code")
		if !has(names, ".claude/rules/always.md") || has(names, ".claude/rules/flutter/state.md") {
			t.Errorf("%v", names)
		}
	})

	t.Run("antigravity rules count when always on", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [antigravity]\n")
		f.sync()
		f.write(".agents/rules/on.md", "---\ntrigger: always_on\n---\n# On\n")
		f.write(".agents/rules/maybe.md", "---\ntrigger: model_decision\n---\n# Maybe\n")

		names, _ := f.parts("antigravity")
		if !has(names, ".agents/rules/on.md") || has(names, ".agents/rules/maybe.md") || !has(names, "AGENTS.md") {
			t.Errorf("%v", names)
		}
	})

	t.Run("an imported memory index is counted once", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\n")
		f.sync()
		f.write(".ai/memory/MEMORY.md", indexLines(3, 60))
		f.write("CLAUDE.md", "@AGENTS.md\n@.ai/memory/MEMORY.md\n")

		names, _ := f.parts("claude-code")
		count := 0
		for _, n := range names {
			if n == ".ai/memory/MEMORY.md" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("%v", names)
		}
	})

	t.Run("project headings that repeat a generated heading are named", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\n")
		f.sync()
		expect(t, f.doctor(), "always-on", OK)
		if strings.Contains(f.doctor()["always-on"].Detail, "repeats") {
			t.Error("a fresh sync repeats nothing")
		}
		agents := f.read("AGENTS.md")
		f.write("AGENTS.md", strings.Replace(agents, sync.ProjectBegin, sync.ProjectBegin+"\n## git\n\n- ours\n\n## Git notes\n\n## Testing\n", 1))

		expect(t, f.doctor(), "always-on", OK, "; the AGENTS.md project section repeats generated headings: git, Testing")
	})

	t.Run("codex reads AGENTS.override.md instead", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [codex]\n")
		f.sync()
		f.write("AGENTS.override.md", "# Override\n")

		names, _ := f.parts("codex")
		if !has(names, "AGENTS.override.md") || has(names, "AGENTS.md") {
			t.Errorf("%v", names)
		}
	})
}

func unique(list []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range list {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func TestMemoryIndex(t *testing.T) {
	t.Run("a line over 150 characters warns with its slug", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\n")
		f.sync()
		f.write(".ai/memory/MEMORY.md", indexLines(2, 150)+"- [Long](project-long-one.md) - "+strings.Repeat("y", 120)+"\n")

		expect(t, f.doctor(), "memory-index", Warn, "3 lines, 1 over 150 characters: project-long-one")
	})

	t.Run("150 characters pass", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\n")
		f.sync()
		f.write(".ai/memory/MEMORY.md", indexLines(4, 150))

		expect(t, f.doctor(), "memory-index", OK, "4 lines, each at most 150 characters")
	})

	t.Run("characters, not bytes", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\n")
		f.sync()
		f.write(".ai/memory/MEMORY.md", "- [Catatan](catatan.md) - "+strings.Repeat("é", 124)+"\n")

		expect(t, f.doctor(), "memory-index", OK)
	})

	t.Run("paths.memory is followed", func(t *testing.T) {
		f := setup(t, "toolkit: 1\nagents: [claude-code]\npaths:\n  memory: docs/ai/memory\n")
		f.sync()
		f.write("docs/ai/memory/MEMORY.md", "- [Long](long.md) - "+strings.Repeat("z", 200)+"\n")

		expect(t, f.doctor(), "memory-index", Warn, "docs/ai/memory/MEMORY.md", "long")
		names, _ := f.parts("claude-code")
		if !has(names, "docs/ai/memory/MEMORY.md") {
			t.Errorf("%v", names)
		}
	})
}
