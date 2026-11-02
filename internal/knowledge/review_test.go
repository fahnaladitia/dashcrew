package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func proposalText(kind, target, body string) string {
	return "---\nkind: " + kind + "\ntarget: " + target + "\nrepo: notes-api\nbranch: feat/x\ncommit: 1a2b3c4d\n---\n\n" + body
}

// answers feeds Ask from a list and records the prompts; it reports the end of input after the last answer.
type answers struct {
	left    []string
	prompts int
}

func (a *answers) ask(prompt string) (string, bool) {
	a.prompts++
	if len(a.left) == 0 {
		return "", false
	}
	answer := a.left[0]
	a.left = a.left[1:]
	return answer, true
}

func reviewFixture(t *testing.T) string {
	t.Helper()
	clone := t.TempDir()
	writeTree(t, clone, map[string]string{
		"README.md":                  "# Knowledge\n\n## Pages\n\nNone reviewed yet.\n",
		"templates/page.md":          pageTemplate,
		"features/share.md":          "---\ntitle: Note sharing\nrepos: [notes-api]\ntags: [sharing]\n---\n\n# Note sharing\n\n## Gotchas\n\nOld. <!-- verified notes-api@abcdef1 2026-01-01 -->\n",
		"inbox/.gitkeep":             "",
		"inbox/2026-02-01-expiry.md": proposalText("feature", "share", "## Gotchas\n\nLinks expire after 7 days. `notes-api:src/share/token.go:42`\n"),
		"inbox/2026-02-02-tags.md":   proposalText("feature", "tags", "## Purpose\n\nTags group notes.\n"),
		"inbox/2026-02-03-bad.md":    strings.Replace(proposalText("feature", "x", "x\n"), "feature", "flow", 1),
		"inbox/2026-02-04-later.md":  proposalText("repo", "notes-api", "Later.\n"),
		"inbox/2026-02-05-noise.md":  proposalText("repo", "notes-api", "Noise.\n"),
		"inbox/2026-02-06-typo.md":   proposalText("repo", "notes-api", "Run make devv.\n"),
	})
	return clone
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestReviewWalksTheInboxOldestFirst(t *testing.T) {
	clone := reviewFixture(t)
	in := &answers{left: []string{"a", "x", "accept", "s", "d", "e", "a"}}
	var out []string
	edited := 0
	r := Review{
		Clone: clone,
		Now:   func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) },
		Ask:   in.ask,
		Edit: func(path string) error {
			edited++
			return os.WriteFile(path, []byte(strings.Replace(read(t, path), "devv", "dev", 1)), 0o644)
		},
		Out: func(line string) { out = append(out, line) },
	}
	result, err := r.Run()
	if err != nil {
		t.Fatal(err)
	}
	if result != (ReviewResult{Accepted: 3, Skipped: 1, Deleted: 1, Malformed: 1}) {
		t.Errorf("result %+v", result)
	}
	if edited != 1 || in.prompts != 7 {
		t.Errorf("edited %d, prompts %d", edited, in.prompts)
	}
	share := read(t, filepath.Join(clone, "features/share.md"))
	if !strings.HasSuffix(share, "Old. <!-- verified notes-api@abcdef1 2026-01-01 -->\n\nLinks expire after 7 days. `notes-api:src/share/token.go:42` <!-- verified notes-api@1a2b3c4 2026-03-01 -->\n") {
		t.Errorf("share.md\n%s", share)
	}
	tags := Parse(read(t, filepath.Join(clone, "features/tags.md")))
	// A page made in this run keeps only the sections a proposal filled.
	if tags.Title != "Tags" || !strings.HasSuffix(tags.Body, "# Tags\n\n## Purpose\n\nTags group notes. <!-- verified notes-api@1a2b3c4 2026-03-01 -->\n") ||
		strings.Contains(tags.Body, "## Entry points") {
		t.Errorf("tags.md\n%s", tags.Body)
	}
	repo := read(t, filepath.Join(clone, "repos/notes-api.md"))
	if !strings.HasSuffix(repo, "## Notes\n\nRun make dev. <!-- verified notes-api@1a2b3c4 2026-03-01 -->\n") {
		t.Errorf("notes-api.md\n%s", repo)
	}
	for name, want := range map[string]bool{"2026-02-01-expiry.md": false, "2026-02-02-tags.md": false, "2026-02-03-bad.md": true,
		"2026-02-04-later.md": true, "2026-02-05-noise.md": false, "2026-02-06-typo.md": false, ".gitkeep": true} {
		if got := exists(filepath.Join(clone, "inbox", name)); got != want {
			t.Errorf("inbox/%s exists %v", name, got)
		}
	}
	readme := read(t, filepath.Join(clone, "README.md"))
	if !strings.Contains(readme, "- [Note sharing](features/share.md): Old. Verified 2026-03-01.") ||
		!strings.Contains(readme, "- [Tags](features/tags.md): Tags group notes. Verified 2026-03-01.") {
		t.Errorf("README.md\n%s", readme)
	}
	text := strings.Join(out, "\n")
	for _, want := range []string{
		"[1/6] inbox/2026-02-01-expiry.md",
		"inbox/2026-02-03-bad.md is malformed, left in inbox/: kind: expected feature or repo",
		"answer a, e, s or d",
		"created features/tags.md from templates/page.md",
		"git -C " + clone + " checkout -b knowledge-review-2026-03-01",
		"git -C " + clone + " add -A",
		"accepted 3, skipped 1, deleted 1, malformed 1",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("output lacks %q\n%s", want, text)
		}
	}
}

func TestReviewStopsAtTheEndOfInputAndKeepsAProposalBrokenByAnEdit(t *testing.T) {
	clone := reviewFixture(t)
	in := &answers{left: []string{"e"}}
	var out []string
	r := Review{Clone: clone, Now: time.Now, Ask: in.ask,
		Edit: func(path string) error { return os.WriteFile(path, []byte("broken\n"), 0o644) },
		Out:  func(line string) { out = append(out, line) }}
	result, err := r.Run()
	if err != nil {
		t.Fatal(err)
	}
	if result.Malformed != 1 || result.Accepted != 0 || in.prompts != 2 {
		t.Errorf("result %+v, prompts %d", result, in.prompts)
	}
	if !exists(filepath.Join(clone, "inbox/2026-02-01-expiry.md")) {
		t.Error("a proposal broken by an edit was removed")
	}
	if text := strings.Join(out, "\n"); !strings.Contains(text, "no more answers; 5 proposals left in inbox/") ||
		!strings.Contains(text, "nothing to commit") && !strings.Contains(text, "README.md") {
		t.Errorf("output\n%s", text)
	}
}

func TestReviewEditFailureReprompts(t *testing.T) {
	clone := reviewFixture(t)
	in := &answers{left: []string{"e", "s"}}
	var out []string
	r := Review{Clone: clone, Now: time.Now, Ask: in.ask,
		Edit: func(string) error { return errors.New("EDITOR is not set") },
		Out:  func(line string) { out = append(out, line) }}
	if _, err := r.Run(); err != nil {
		t.Fatal(err)
	}
	if text := strings.Join(out, "\n"); !strings.Contains(text, "edit failed: EDITOR is not set") {
		t.Errorf("output\n%s", text)
	}
}

func TestReviewRefusesAFolderThatIsNotAClone(t *testing.T) {
	r := Review{Clone: filepath.Join(t.TempDir(), "missing"), Now: time.Now, Ask: (&answers{}).ask, Out: func(string) {}}
	if _, err := r.Run(); err == nil || !strings.Contains(err.Error(), "no inbox/") {
		t.Errorf("err %v", err)
	}
}
