package doctor

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/fahnaladitia/dashcrew/internal/agents"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// AlwaysOnBudget is the most an agent should load from the project before the first answer: AGENTS.md, the memory
// index and the rules that are always on, about 10k tokens.
const AlwaysOnBudget = 40 * 1024

// MemoryIndexLineMax is the longest line the memory index should hold: a title and a short hook.
const MemoryIndexLineMax = 150

// importDepth is how many `@path` hops Claude Code follows from CLAUDE.md.
const importDepth = 5

// alwaysOnPart is one file an agent loads at session start.
type alwaysOnPart struct {
	path string
	size int
}

// claudeImport is `@path` at the start of a line or after a space; code is skipped by the caller.
var claudeImport = regexp.MustCompile(`(?:^|\s)@([^\s` + "`" + `]+)`)

// alwaysOnParts lists what agent loads from the project at session start, and the imports outside the project it
// does not count.
func (d Doctor) alwaysOnParts(agent string) ([]alwaysOnPart, []string) {
	var parts []alwaysOnPart
	var outside []string
	seen := map[string]bool{}
	add := func(relative string) bool {
		relative = filepath.ToSlash(filepath.Clean(relative))
		if seen[relative] {
			return false
		}
		info, err := os.Stat(d.path(relative))
		if err != nil || info.IsDir() {
			return false
		}
		seen[relative] = true
		parts = append(parts, alwaysOnPart{relative, int(info.Size())})
		return true
	}
	switch agent {
	case "claude-code":
		var follow func(relative string, depth int)
		follow = func(relative string, depth int) {
			if !add(relative) || depth == importDepth {
				return
			}
			text, _ := d.read(relative)
			for _, target := range importsOf(text) {
				resolved, ok := d.inProject(filepath.Join(filepath.Dir(d.path(relative)), filepath.FromSlash(target)), target)
				if !ok {
					outside = append(outside, target)
					continue
				}
				follow(resolved, depth+1)
			}
		}
		follow("CLAUDE.md", 0)
		follow("CLAUDE.local.md", 0)
		for _, rule := range d.ruleFiles(".claude/rules", true) {
			if _, scoped := frontMatterValue(d.readOrEmpty(rule), "paths"); !scoped {
				add(rule)
			}
		}
	case "codex":
		if !add("AGENTS.override.md") {
			add("AGENTS.md")
		}
	case "antigravity":
		add("AGENTS.md")
		for _, rule := range d.ruleFiles(".agents/rules", false) {
			if trigger, _ := frontMatterValue(d.readOrEmpty(rule), "trigger"); trigger == "always_on" {
				add(rule)
			}
		}
	}
	if memory, ok := d.memoryIndexPath(); ok {
		add(memory)
	}
	return parts, outside
}

// inProject is target, an import, relative to the project; false for `~/…` and for a path outside the project.
func (d Doctor) inProject(joined, target string) (string, bool) {
	if strings.HasPrefix(target, "~") {
		return "", false
	}
	path := joined
	if filepath.IsAbs(filepath.FromSlash(target)) {
		path = filepath.FromSlash(target)
	}
	relative, err := filepath.Rel(d.ProjectDir, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return relative, true
}

// importsOf lists the `@path` imports of text, outside fenced code blocks and code spans.
func importsOf(text string) []string {
	var out []string
	fenced := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		parts := strings.Split(line, "`")
		for i := 0; i < len(parts); i += 2 {
			for _, m := range claudeImport.FindAllStringSubmatch(parts[i], -1) {
				out = append(out, m[1])
			}
		}
	}
	return out
}

// ruleFiles are the `.md` files in dir, below it too when deep, as project-relative slash paths in order.
func (d Doctor) ruleFiles(dir string, deep bool) []string {
	var out []string
	root := d.path(dir)
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && !deep {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".md" {
			if relative, err := filepath.Rel(d.ProjectDir, path); err == nil {
				out = append(out, filepath.ToSlash(relative))
			}
		}
		return nil
	})
	return out
}

func (d Doctor) readOrEmpty(relative string) string {
	text, _ := d.read(relative)
	return text
}

// frontMatterValue is the value of a top-level key in the `---` front matter of text, unquoted; false when the key
// is absent.
func frontMatterValue(text, key string) (string, bool) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", false
	}
	for _, line := range strings.Split(text[4:4+end], "\n") {
		if value, ok := strings.CutPrefix(line, key+":"); ok {
			return strings.Trim(strings.TrimSpace(value), `"'`), true
		}
	}
	return "", false
}

// memoryIndexPath is `{{paths.memory}}/MEMORY.md` relative to the project, or absolute when the memory lives
// outside it; false when there is no index.
func (d Doctor) memoryIndexPath() (string, bool) {
	dir := ".ai/memory"
	if p, err := d.parseProfile(); err == nil && p.Paths != nil {
		if value, ok := p.Paths.Value("memory").(string); ok && value != "" {
			dir = value
		}
	}
	path := filepath.ToSlash(filepath.Join(filepath.FromSlash(dir), "MEMORY.md"))
	full := path
	if !filepath.IsAbs(filepath.FromSlash(dir)) {
		full = d.path(path)
	}
	if !isFile(full) {
		return "", false
	}
	return path, true
}

func kib(n int) string { return fmt.Sprintf("%.1f", float64(n)/1024) }

// alwaysOn sums, per selected agent, what it loads from the project at session start, against AlwaysOnBudget.
func (d Doctor) alwaysOn(selected map[string]bool) Check {
	const id = "always-on"
	status := OK
	var lines []string
	for _, agent := range agents.All() {
		if !selected[agent] {
			continue
		}
		parts, outside := d.alwaysOnParts(agent)
		total := 0
		for _, p := range parts {
			total += p.size
		}
		line := fmt.Sprintf("%s %s KiB of %s KiB", agent, kib(total), kib(AlwaysOnBudget))
		if total > AlwaysOnBudget {
			status = Warn
			sort.SliceStable(parts, func(i, j int) bool { return parts[i].size > parts[j].size })
			names := make([]string, len(parts))
			for i, p := range parts {
				names[i] = p.path + " " + kib(p.size)
			}
			line = fmt.Sprintf("%s %s KiB over %s KiB: %s", agent, kib(total), kib(AlwaysOnBudget), strings.Join(names, ", "))
		}
		if len(outside) > 0 {
			line += " (outside the project, not counted: " + strings.Join(outside, ", ") + ")"
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return Check{id, Skip, "no agent selected"}
	}
	if repeated := d.repeatedHeadings(); len(repeated) > 0 {
		lines = append(lines, "the AGENTS.md project section repeats generated headings: "+strings.Join(repeated, ", "))
	}
	return Check{id, status, strings.Join(lines, "; ")}
}

var markdownHeading = regexp.MustCompile(`(?m)^#{1,6}[ \t]+(.+?)[ \t]*$`)

// repeatedHeadings are the headings of the AGENTS.md project section, as written there, that a generated heading
// already has, compared without case: the same topic loaded twice at every session start.
func (d Doctor) repeatedHeadings() []string {
	text, err := d.read("AGENTS.md")
	if err != nil {
		return nil
	}
	begin := strings.Index(text, sync.ProjectBegin)
	if begin < 0 {
		return nil
	}
	project := text[begin+len(sync.ProjectBegin):]
	if end := strings.Index(project, sync.ProjectEnd); end >= 0 {
		project = project[:end]
	}
	generated := map[string]bool{}
	for _, m := range markdownHeading.FindAllStringSubmatch(text[:begin], -1) {
		generated[strings.ToLower(m[1])] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, m := range markdownHeading.FindAllStringSubmatch(project, -1) {
		key := strings.ToLower(m[1])
		if generated[key] && !seen[key] {
			seen[key] = true
			out = append(out, m[1])
		}
	}
	return out
}

// memoryIndexLink is the slug a memory index line points at.
var memoryIndexLink = regexp.MustCompile(`\]\(([^)\s]+?)(?:\.md)?\)`)

// memoryIndex warns about memory index lines longer than MemoryIndexLineMax characters; false without an index.
func (d Doctor) memoryIndex() (Check, bool) {
	const id = "memory-index"
	path, ok := d.memoryIndexPath()
	if !ok {
		return Check{}, false
	}
	full := path
	if !filepath.IsAbs(filepath.FromSlash(path)) {
		full = d.path(path)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return Check{id, Warn, path + ": " + err.Error()}, true
	}
	entries := 0
	var long []string
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "- ") {
			entries++
		}
		if utf8.RuneCountInString(line) <= MemoryIndexLineMax {
			continue
		}
		name := line
		if m := memoryIndexLink.FindStringSubmatch(line); m != nil {
			name = filepath.Base(m[1])
		} else if utf8.RuneCountInString(name) > 40 {
			name = string([]rune(name)[:40]) + "..."
		}
		long = append(long, name)
	}
	if len(long) == 0 {
		return Check{id, OK, fmt.Sprintf("%s: %d lines, each at most %d characters", path, entries, MemoryIndexLineMax)}, true
	}
	shown := long
	if len(shown) > 3 {
		shown = shown[:3]
	}
	return Check{id, Warn, fmt.Sprintf("%s: %d lines, %d over %d characters: %s; keep the title and a short hook, the fact lives in its file",
		path, entries, len(long), MemoryIndexLineMax, strings.Join(shown, ", "))}, true
}
