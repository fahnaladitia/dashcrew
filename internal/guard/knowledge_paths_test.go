package guard

import (
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/pathstyle"
)

// fakeFS is a file system held in a map, keyed by paths spelled in style, so the knowledge rules run on Windows
// paths on any host.
type fakeFS struct {
	style   pathstyle.Style
	entries map[string]fakeEntry
}

type fakeEntry struct {
	dir     bool
	link    string
	content string
}

func newFakeFS(style pathstyle.Style) *fakeFS {
	return &fakeFS{style: style, entries: map[string]fakeEntry{}}
}

func (f *fakeFS) key(name string) string {
	name = f.style.Clean(name)
	if f.style.FoldCase {
		name = strings.ToLower(name)
	}
	return name
}

// add puts entry at name and a folder at every parent of name.
func (f *fakeFS) add(name string, entry fakeEntry) {
	f.entries[f.key(name)] = entry
	for dir := f.style.Dir(name); ; dir = f.style.Dir(dir) {
		if _, ok := f.entries[f.key(dir)]; !ok {
			f.entries[f.key(dir)] = fakeEntry{dir: true}
		}
		if f.style.Dir(dir) == dir {
			return
		}
	}
}

func (f *fakeFS) lstat(name string) (bool, bool) {
	entry, ok := f.entries[f.key(name)]
	return ok, ok && entry.link != ""
}

func (f *fakeFS) readlink(name string) (string, error) {
	if entry, ok := f.entries[f.key(name)]; ok && entry.link != "" {
		return entry.link, nil
	}
	return "", errors.New("not a link")
}

func (f *fakeFS) head(name string, n int) ([]byte, error) {
	entry, ok := f.entries[f.key(name)]
	switch {
	case !ok:
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	case entry.dir:
		return nil, errors.New("is a directory")
	}
	return []byte(entry.content)[:min(n, len(entry.content))], nil
}

// pathWorld is one machine: a project in /work/app whose .ai/knowledge links to the clone under /home/me, written
// in POSIX shape and translated to a style by spell.
type pathWorld struct {
	style pathstyle.Style
	spell func(posix string) string
}

var (
	posixWorld   = pathWorld{pathstyle.POSIX, func(p string) string { return p }}
	darwinWorld  = pathWorld{pathstyle.Darwin, func(p string) string { return p }}
	windowsWorld = pathWorld{pathstyle.Windows, windowsSpelling}
)

// windowsSpelling is a POSIX path as Windows writes it: an absolute path on drive C:, backslashes throughout.
func windowsSpelling(posix string) string {
	if strings.HasPrefix(posix, "/") {
		return "C:" + strings.ReplaceAll(posix, "/", `\`)
	}
	return strings.ReplaceAll(posix, "/", `\`)
}

const (
	worldHome    = "/home/me"
	worldClone   = "/home/me/.dashcrew/knowledge/notes"
	worldProject = "/work/app"
)

func (w pathWorld) fs() *fakeFS {
	f := newFakeFS(w.style)
	for name, content := range map[string]string{
		worldClone + "/features/share.md":       "share\n",
		worldClone + "/specs/1.3.0/todo/prd.md": "---\ntitle: x\nstatus: approved\nrev: 1\n---\n",
		worldClone + "/specs/1.4.0/sync/prd.md": "---\ntitle: x\nstatus: draft\nrev: 1\n---\n",
		worldProject + "/lib/a.dart":            "void a() {}\n",
		worldProject + "/docs/notes.md":         "notes\n",
	} {
		f.add(w.spell(name), fakeEntry{content: content})
	}
	for _, dir := range []string{worldClone + "/inbox", worldClone + "/repos", worldProject + "/.ai/plans"} {
		f.add(w.spell(dir), fakeEntry{dir: true})
	}
	for link, target := range map[string]string{
		worldProject + "/.ai/knowledge": w.spell(worldClone),
		worldProject + "/kb":            w.spell(".ai/knowledge"),
		worldProject + "/docs/alias":    w.spell(worldClone + "/features"),
		worldProject + "/docs/up":       w.spell("../.ai/knowledge/inbox"),
	} {
		f.add(w.spell(link), fakeEntry{link: target})
	}
	return f
}

// policy is the knowledge policy answering a hook that runs in cwd for a user whose home is home.
func (w pathWorld) policy(t *testing.T, cwd, home string) Policy {
	t.Helper()
	p, err := PolicyFromJSON(knowledgePolicy)
	if err != nil {
		t.Fatal(err)
	}
	p.knowledge = newKnowledgeRoots(w.style, w.fs(), cwd, w.spell(worldProject), home, p.KnowledgeClone)
	return p
}

func evaluateIn(t *testing.T, p Policy, event Event) Decision {
	t.Helper()
	decision, err := Evaluate(event, p)
	if err != nil {
		t.Fatalf("%+v: %v", event, err)
	}
	return decision
}

func worldWrite(path, role string) Event {
	event := Event{Kind: Write, FilePath: path, Content: "x"}
	if role != "" {
		event.Role = &role
	}
	return event
}

var worldWrites = []struct{ path, role, reason string }{
	{".ai/knowledge/features/share.md", "", knowledgeReason},
	{".ai/knowledge/inbox/x.md", "", ""},
	{worldClone + "/features/share.md", "", knowledgeReason},
	{worldClone + "/inbox/x.md", "", ""},
	{worldProject + "/.ai/knowledge/repos/x.md", "", knowledgeReason},
	{".ai/knowledge/inbox/../features/share.md", "", knowledgeReason},
	{"lib/../.ai/knowledge/features/x.md", "", knowledgeReason},
	{"kb/features/x.md", "", knowledgeReason},
	{"kb/inbox/x.md", "", ""},
	{"docs/alias/x.md", "", knowledgeReason},
	{"docs/alias/../inbox/x.md", "", ""},
	{"docs/alias/../repos/x.md", "", knowledgeReason},
	{"docs/up/x.md", "", ""},
	{"docs/up/../features/x.md", "", knowledgeReason},
	{"~/.dashcrew/knowledge/notes/features/x.md", "", knowledgeReason},
	{".ai/knowledge/specs/1.3.0/todo/prd.md", "", specReason},
	{".ai/knowledge/specs/1.3.0/todo", "", specReason},
	{".ai/knowledge/specs/1.4.0/sync/prd.md", "", ""},
	{".ai/knowledge/specs/1.5.0/new/prd.md", "", ""},
	{".ai/knowledge/specs/1.4.0/sync/../../1.3.0/todo/prd.md", "", specReason},
	{"lib/a.dart", "", ""},
	{"docs/notes.md", "", ""},
	{"/repo/.ai/plans/../../lib/a.dart", "planner", ""},
	{".ai/plans/../../lib/a.dart", "planner", "may only write under .ai/plans/"},
	{".ai/plans/x.md", "planner", ""},
	{".ai/knowledge/inbox/x.md", "planner", "may only write under .ai/plans/"},
}

var worldScripts = []struct{ script, reason string }{
	{"rm -rf " + worldClone + "/features", knowledgeReason},
	{"echo x > " + worldClone + "/inbox/x.md", ""},
	{"cd " + worldProject + "/.ai/knowledge && rm features/share.md", knowledgeReason},
	{"cd " + worldProject + "/.ai/knowledge/inbox && echo x > p.md", ""},
	{"cp a.md " + worldProject + "/kb/repos/", knowledgeReason},
	{"echo x > .ai/knowledge/features/x.md", knowledgeReason},
	{"echo x > $HOME/.dashcrew/knowledge/notes/repos/x.md", knowledgeReason},
	{"tee ~/.dashcrew/knowledge/notes/specs/1.3.0/todo/prd.md < x", specReason},
	{"echo x > /tmp/x", ""},
	{"cat " + worldClone + "/features/share.md", ""},
}

// TestKnowledgeRulesOnPOSIXPaths pins the decisions the Windows-shaped tests below must repeat.
func TestKnowledgeRulesOnPOSIXPaths(t *testing.T) {
	for _, w := range []pathWorld{posixWorld, darwinWorld} {
		p := w.policy(t, worldProject, worldHome)
		for _, c := range worldWrites {
			if got := evaluateIn(t, p, worldWrite(c.path, c.role)); got.Blocked != (c.reason != "") || !strings.Contains(got.Reason, c.reason) {
				t.Errorf("%v write %s as %q: %+v, want %q", w.style, c.path, c.role, got, c.reason)
			}
		}
		for _, c := range worldScripts {
			if got := evaluateIn(t, p, Event{Kind: Shell, Command: c.script}); got.Blocked != (c.reason != "") || !strings.Contains(got.Reason, c.reason) {
				t.Errorf("%v %q: %+v, want %q", w.style, c.script, got, c.reason)
			}
		}
	}
}

// windowsWrites are the ways a Windows agent may spell a POSIX-shaped write path of the world.
func windowsWrites(posix string) []string {
	if !strings.HasPrefix(posix, "/") {
		return []string{posix, windowsSpelling(posix)}
	}
	upper := strings.NewReplacer(worldHome, strings.ToUpper(worldHome), worldProject, strings.ToUpper(worldProject)).Replace(posix)
	native := windowsSpelling(posix)
	return []string{
		native,
		strings.ReplaceAll(native, `\`, "/"),
		"c" + native[1:],
		`\\?\` + native,
		windowsSpelling(upper),
		posix,
		native[2:],
	}
}

var absoluteWord = regexp.MustCompile(`(^|[ >])/`)

// windowsScripts are a POSIX-shaped script with its absolute paths written as Git Bash accepts them.
func windowsScripts(script string) []string {
	return []string{absoluteWord.ReplaceAllString(script, "${1}C:/"), absoluteWord.ReplaceAllString(script, "${1}/c/")}
}

// TestKnowledgeRulesOnWindowsPathsMatchPOSIX runs every case of TestKnowledgeRulesOnPOSIXPaths on a Windows machine
// spelled every way an agent, Git Bash or the hook may spell it: each decision must equal the POSIX one.
func TestKnowledgeRulesOnWindowsPathsMatchPOSIX(t *testing.T) {
	want := posixWorld.policy(t, worldProject, worldHome)
	for _, env := range []struct{ cwd, home string }{
		{`C:\work\app`, `C:\home\me`},
		{`c:/work/app`, `c:\home\me`},
		{`/c/work/app`, `/c/home/me`},
		{`C:\WORK\APP`, `C:\HOME\ME`},
		{`\\?\C:\work\app`, `C:/home/me`},
	} {
		p := windowsWorld.policy(t, env.cwd, env.home)
		for _, c := range worldWrites {
			expected := evaluateIn(t, want, worldWrite(c.path, c.role))
			for _, path := range windowsWrites(c.path) {
				for _, kind := range []Kind{Write, Edit} {
					event := worldWrite(path, c.role)
					event.Kind = kind
					expected := expected
					if kind == Edit {
						posix := worldWrite(c.path, c.role)
						posix.Kind = Edit
						expected = evaluateIn(t, want, posix)
					}
					if got := evaluateIn(t, p, event); got != expected {
						t.Errorf("cwd %s: %v %s as %q: %+v, POSIX %+v", env.cwd, kind, path, c.role, got, expected)
					}
				}
			}
		}
		for _, c := range worldScripts {
			expected := evaluateIn(t, want, Event{Kind: Shell, Command: c.script})
			for _, script := range windowsScripts(c.script) {
				if got := evaluateIn(t, p, Event{Kind: Shell, Command: script}); got != expected {
					t.Errorf("cwd %s: %q: %+v, POSIX %+v", env.cwd, script, got, expected)
				}
			}
		}
	}
}

func TestRealPathOnWindowsPaths(t *testing.T) {
	f := newFakeFS(pathstyle.Windows)
	f.add(`C:\kb\features\share.md`, fakeEntry{content: "x"})
	f.add(`C:\kb\inbox`, fakeEntry{dir: true})
	f.add(`D:\app\abs`, fakeEntry{link: `C:\kb`})
	f.add(`D:\app\extended`, fakeEntry{link: `\\?\C:\kb\features`})
	f.add(`D:\app\rooted`, fakeEntry{link: `\other`})
	f.add(`D:\app\relative`, fakeEntry{link: `..\app\abs\inbox`})
	f.add(`D:\app\slashes`, fakeEntry{link: `C:/kb/features`})
	f.add(`D:\app\loop`, fakeEntry{link: `loop`})
	k := &knowledgeRoots{style: pathstyle.Windows, fsys: f}
	for path, want := range map[string]string{
		`D:\app\abs\features\share.md`: `C:\kb\features\share.md`,
		`d:/app/ABS/inbox/x`:           `C:\kb\inbox\x`,
		`D:\app\extended\..\inbox\x`:   `C:\kb\inbox\x`,
		`D:\app\rooted\x`:              `D:\other\x`,
		`D:\app\relative\x`:            `C:\kb\inbox\x`,
		`D:\app\slashes\..\inbox`:      `C:\kb\inbox`,
		`\\?\D:\app\abs`:               `C:\kb`,
		`D:\app\missing\..\abs\x`:      `D:\app\abs\x`,
		`C:\kb\features\share.md\x`:    `C:\kb\features\share.md\x`,
	} {
		if got := k.realPath(path); got != want {
			t.Errorf("realPath(%q) = %q, want %q", path, got, want)
		}
	}
	if got := k.realPath(`D:\app\loop\x`); got == "" {
		t.Error("a link loop gave no path")
	}
}

func TestNewKnowledgeRootsOnWindows(t *testing.T) {
	f := windowsWorld.fs()
	for _, home := range []string{`C:\home\me`, `/c/home/me`, `c:/home/me`} {
		k := newKnowledgeRoots(pathstyle.Windows, f, `C:\work\app`, `C:\work\app`, home, "~/.dashcrew/knowledge/notes")
		if want := []string{`C:\home\me\.dashcrew\knowledge\notes`}; fmt.Sprint(k.roots) != fmt.Sprint(want) {
			t.Errorf("home %s: roots %q, want %q (the project link is the clone)", home, k.roots, want)
		}
	}
	k := newKnowledgeRoots(pathstyle.Windows, f, `C:\work\app`, `C:\work\app`, "", "~/.dashcrew/knowledge/notes")
	if want := []string{`C:\home\me\.dashcrew\knowledge\notes`}; fmt.Sprint(k.roots) != fmt.Sprint(want) {
		t.Errorf("without home: roots %q, want the project link %q", k.roots, want)
	}
}
