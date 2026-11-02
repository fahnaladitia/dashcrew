package sync

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

const knowledgeProfile = "toolkit: 1\nagents: [claude-code]\nknowledge: { repo: 'git@example.com:team/notes.git' }\n"

const (
	shaOne = "1111111111111111111111111111111111111111"
	shaTwo = "2222222222222222222222222222222222222222"
)

// fakeClone writes a clone of the knowledge repo under home: two pages and a git folder whose HEAD is sha.
func fakeClone(t *testing.T, home, sha string) string {
	t.Helper()
	clone := filepath.Join(home, ".dashcrew", "knowledge", "notes")
	write(t, clone, ".git/HEAD", "ref: refs/heads/main\n")
	write(t, clone, ".git/refs/heads/main", sha+"\n")
	write(t, clone, "features/share.md", "---\ntitle: Share\n---\n")
	write(t, clone, "inbox/.gitkeep", "")
	return clone
}

func knowledgeProject(t *testing.T, profile string) (Sync, string, string) {
	t.Helper()
	s, project := newProject(t, profile)
	home, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s.Home = home
	return s, project, home
}

func knowledgeEntry(t *testing.T, project string) *omap.Map[any] {
	t.Helper()
	entry, _ := lock(t, project).Value("knowledge").(*omap.Map[any])
	return entry
}

func TestAProfileWithoutKnowledgeGetsNoLinkLockEntryIgnoreBlockOrGuardField(t *testing.T) {
	s, project, _ := knowledgeProject(t, defaultProfile)
	run(t, s, false, false)
	if exists(project, ".ai/knowledge") || exists(project, ".gitignore") || knowledgeEntry(t, project) != nil {
		t.Error("knowledge output without a knowledge key")
	}
	if strings.Contains(read(t, project, ".ai/guard.json"), "knowledge") {
		t.Error("guard.json names knowledge")
	}
}

func TestAMissingCloneWarnsAndSyncContinues(t *testing.T) {
	s, project, _ := knowledgeProject(t, knowledgeProfile)
	result := run(t, s, false, false)
	want := "knowledge clone notes is missing (~/.dashcrew/knowledge/notes); run dashcrew knowledge pull"
	if !slices.Contains(result.Warnings, want) {
		t.Errorf("warnings %v", result.Warnings)
	}
	if !slices.Contains(result.Written, ".gitignore") || slices.Contains(result.Written, ".ai/knowledge") {
		t.Errorf("written %v", result.Written)
	}
	if got := encode(t, knowledgeEntry(t, project)); got != `{"name":"notes","link":".ai/knowledge","state":"missing"}` {
		t.Errorf("lock entry %s", got)
	}
	if got := read(t, project, ".gitignore"); got != ignoreBegin+"\n/.ai/knowledge\n"+ignoreEnd+"\n" {
		t.Errorf(".gitignore %q", got)
	}
	if !strings.Contains(read(t, project, ".ai/guard.json"), `"knowledge_clone": "~/.dashcrew/knowledge/notes"`) {
		t.Error("guard.json lacks the clone")
	}
	again := run(t, s, false, false)
	if len(again.Written) != 0 || !slices.Contains(again.Warnings, want) {
		t.Errorf("second sync wrote %v, warned %v", again.Written, again.Warnings)
	}

	write(t, project, ".ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")
	if dropped := run(t, s, false, false); !slices.Contains(dropped.Removed, ".gitignore") || slices.Contains(dropped.Written, ".gitignore") {
		t.Errorf("dropping knowledge wrote %v, removed %v", dropped.Written, dropped.Removed)
	}
	if exists(project, ".gitignore") {
		t.Error("a .gitignore holding only the block was kept empty")
	}
}

func TestSyncLinksTheCloneRecordsItsHeadAndRepairsTheLink(t *testing.T) {
	s, project, home := knowledgeProject(t, knowledgeProfile)
	clone := fakeClone(t, home, shaOne)
	result := run(t, s, false, false)
	if !slices.Contains(result.Written, ".ai/knowledge") || len(result.Warnings) != 0 {
		t.Errorf("written %v, warnings %v", result.Written, result.Warnings)
	}
	if target, err := os.Readlink(filepath.Join(project, ".ai", "knowledge")); err != nil || target != clone {
		t.Errorf("link %q, %v", target, err)
	}
	want := `{"name":"notes","link":".ai/knowledge","state":"linked","target":` + encode(t, clone) + `,"head":"` + shaOne + `"}`
	if got := encode(t, knowledgeEntry(t, project)); got != want {
		t.Errorf("lock entry %s", got)
	}
	if again := run(t, s, false, false); len(again.Written) != 0 {
		t.Errorf("second sync wrote %v", again.Written)
	}

	write(t, clone, ".git/refs/heads/main", shaTwo+"\n")
	if again := run(t, s, false, false); len(again.Written) != 0 {
		t.Errorf("a pull rewrote %v", again.Written)
	}
	if got := knowledgeEntry(t, project).Value("head"); got != shaTwo {
		t.Errorf("head after a pull %v", got)
	}

	link := filepath.Join(project, ".ai", "knowledge")
	os.Remove(link)
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	if repaired := run(t, s, false, false); !slices.Contains(repaired.Written, ".ai/knowledge") {
		t.Errorf("repair wrote %v", repaired.Written)
	}
	if target, _ := os.Readlink(link); target != clone {
		t.Errorf("repaired link points to %s", target)
	}
}

func TestSyncCopiesTheCloneWhenSymlinksFail(t *testing.T) {
	s, project, home := knowledgeProject(t, knowledgeProfile)
	clone := fakeClone(t, home, shaOne)
	original := symlink
	symlink = func(string, string) error { return errors.New("symlinks are not allowed") }
	t.Cleanup(func() { symlink = original })

	result := run(t, s, false, false)
	if !slices.Contains(result.Written, ".ai/knowledge") || isLink(project, ".ai/knowledge") {
		t.Errorf("written %v", result.Written)
	}
	if read(t, project, ".ai/knowledge/features/share.md") != "---\ntitle: Share\n---\n" || exists(project, ".ai/knowledge/.git") {
		t.Error("copy is not the clone without .git")
	}
	if got := knowledgeEntry(t, project).Value("state"); got != "copied" {
		t.Errorf("state %v", got)
	}
	if again := run(t, s, false, false); len(again.Written) != 0 {
		t.Errorf("an unchanged clone was copied again: %v", again.Written)
	}

	write(t, clone, ".git/refs/heads/main", shaTwo+"\n")
	write(t, clone, "features/tags.md", "tags\n")
	os.Remove(filepath.Join(clone, "features", "share.md"))
	if again := run(t, s, false, false); !slices.Contains(again.Written, ".ai/knowledge") {
		t.Errorf("a pulled clone was not copied: %v", again.Written)
	}
	if !exists(project, ".ai/knowledge/features/tags.md") || exists(project, ".ai/knowledge/features/share.md") {
		t.Error("the copy does not follow the clone")
	}

	symlink = original
	if relinked := run(t, s, false, false); !slices.Contains(relinked.Written, ".ai/knowledge") || !isLink(project, ".ai/knowledge") {
		t.Errorf("a copy was not replaced by the link once symlinks work: %v", relinked.Written)
	}
}

func TestOnWindowsSyncCopiesWithoutTryingALink(t *testing.T) {
	s, project, home := knowledgeProject(t, knowledgeProfile)
	fakeClone(t, home, shaOne)
	originalOS, originalLink := goos, symlink
	goos = "windows"
	symlink = func(string, string) error { t.Error("symlink called on windows"); return nil }
	t.Cleanup(func() { goos, symlink = originalOS, originalLink })
	run(t, s, false, false)
	if isLink(project, ".ai/knowledge") || knowledgeEntry(t, project).Value("state") != "copied" {
		t.Error("not copied")
	}
}

func TestAForeignKnowledgeFolderIsLeftAlone(t *testing.T) {
	s, project, home := knowledgeProject(t, knowledgeProfile)
	fakeClone(t, home, shaOne)
	write(t, project, ".ai/knowledge/mine.md", "mine\n")
	result := run(t, s, false, false)
	want := ".ai/knowledge exists and was not made by dashcrew; left as is. Move it away and run dashcrew sync to link the knowledge clone."
	if !slices.Contains(result.Warnings, want) || read(t, project, ".ai/knowledge/mine.md") != "mine\n" {
		t.Errorf("warnings %v", result.Warnings)
	}
	if got := knowledgeEntry(t, project).Value("state"); got != "unlinked" {
		t.Errorf("state %v", got)
	}
}

func TestTheIgnoreBlockKeepsOtherLinesAndGoesWithTheKnowledgeKey(t *testing.T) {
	s, project, home := knowledgeProject(t, knowledgeProfile)
	fakeClone(t, home, shaOne)
	write(t, project, ".gitignore", "build/\n*.log")
	run(t, s, false, false)
	block := ignoreBegin + "\n/.ai/knowledge\n" + ignoreEnd + "\n"
	if got := read(t, project, ".gitignore"); got != "build/\n*.log\n"+block {
		t.Errorf(".gitignore %q", got)
	}
	write(t, project, ".gitignore", "build/\n"+ignoreBegin+"\n/old\n"+ignoreEnd+"\ndist/\n")
	if result := run(t, s, false, false); !slices.Contains(result.Written, ".gitignore") {
		t.Errorf("a stale block was kept: %v", result.Written)
	}
	if got := read(t, project, ".gitignore"); got != "build/\n"+block+"dist/\n" {
		t.Errorf("block not replaced in place: %q", got)
	}

	write(t, project, ".ai/project.yaml", "toolkit: 1\nagents: [claude-code]\n")
	result := run(t, s, false, false)
	if got := read(t, project, ".gitignore"); got != "build/\ndist/\n" {
		t.Errorf(".gitignore after knowledge was dropped %q", got)
	}
	if !slices.Contains(result.Removed, ".ai/knowledge") || exists(project, ".ai/knowledge") || isLink(project, ".ai/knowledge") {
		t.Errorf("removed %v", result.Removed)
	}
	if knowledgeEntry(t, project) != nil {
		t.Error("lock keeps the knowledge entry")
	}
}

func TestIgnoreBlockEdgeCases(t *testing.T) {
	for _, c := range []struct{ current, want string }{
		{"", ""},
		{"a\n", "a\n"},
		{ignoreBegin + "\n/x\n" + ignoreEnd + "\n", ""},
		{"a\n" + ignoreBegin + "\n/x\n" + ignoreEnd, "a\n"},
		{"a\n" + ignoreBegin + "\n/x\n", "a\n" + ignoreBegin + "\n/x\n"},
	} {
		if got := withIgnoreBlock(c.current, nil); got != c.want {
			t.Errorf("%q: %q, want %q", c.current, got, c.want)
		}
	}
	if got := withIgnoreBlock("", []string{"/a", "/b"}); got != ignoreBegin+"\n/a\n/b\n"+ignoreEnd+"\n" {
		t.Errorf("%q", got)
	}
}

func TestKnowledgeRegistersTheMcpServerForEveryAgentAndDroppingItRemovesIt(t *testing.T) {
	const agents = "toolkit: 1\nagents: [claude-code, codex, antigravity]\n"
	s, project, _ := knowledgeProject(t, agents+"knowledge: { repo: 'git@example.com:team/notes.git' }\n")
	run(t, s, false, false)
	const args = `["knowledge","serve","--clone","~/.dashcrew/knowledge/notes"]`
	if got := encode(t, object(t, read(t, project, ".mcp.json")).Value("mcpServers")); !strings.Contains(got, `"dashcrew-knowledge":{"type":"stdio","command":"dashcrew","args":`+args+`}`) {
		t.Errorf(".mcp.json %s", got)
	}
	if got := encode(t, object(t, read(t, project, ".agents/mcp_config.json")).Value("mcpServers")); !strings.Contains(got, `"dashcrew-knowledge":{"command":"dashcrew","args":`+args+`}`) {
		t.Errorf("mcp_config.json %s", got)
	}
	if got := read(t, project, ".codex/config.toml"); !strings.Contains(got, "[mcp_servers.dashcrew-knowledge]\ncommand = \"dashcrew\"\nargs = "+args+"\n") {
		t.Errorf("config.toml %q", got)
	}
	want := `{"antigravity":["dart-mcp-server","dashcrew-knowledge"],"claude-code":["dart-mcp-server","dashcrew-knowledge"],"codex":["dart-mcp-server","dashcrew-knowledge"]}`
	if got := encode(t, lock(t, project).Value("mcp_servers")); got != want {
		t.Errorf("lock mcp_servers %s", got)
	}

	write(t, project, ".ai/project.yaml", agents)
	run(t, s, false, false)
	for _, file := range []string{".mcp.json", ".agents/mcp_config.json", ".codex/config.toml", LockPath} {
		if strings.Contains(read(t, project, file), "dashcrew-knowledge") {
			t.Errorf("%s still names dashcrew-knowledge", file)
		}
	}
}
