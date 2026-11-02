package pathstyle

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Users\me`:              `C:\Users\me`,
		`c:\Users\me`:              `C:\Users\me`,
		`c:/Users/me`:              `C:\Users\me`,
		`C:\Users/me\x/y`:          `C:\Users\me\x\y`,
		`\\?\C:\Users\me`:          `C:\Users\me`,
		`\\?\c:\Users\me`:          `C:\Users\me`,
		`\??\C:\Users\me`:          `C:\Users\me`,
		`//?/C:/Users/me`:          `C:\Users\me`,
		`\\?\UNC\server\share\x`:   `\\server\share\x`,
		`\\?\Volume{1234}\x`:       `\\?\Volume{1234}\x`,
		`\\server\share\x`:         `\\server\share\x`,
		`relative/dir`:             `relative\dir`,
		`/repo/lib`:                `\repo\lib`,
		`~/.dashcrew/knowledge/kb`: `~\.dashcrew\knowledge\kb`,
		``:                         ``,
	} {
		if got := Windows.Normalize(in); got != want {
			t.Errorf("Windows.Normalize(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{`/a/b`, `C:\x`, `\\?\C:\x`, `a\b/c`} {
		if got := POSIX.Normalize(in); got != in {
			t.Errorf("POSIX.Normalize(%q) = %q", in, got)
		}
	}
}

func TestVolumeAndIsAbs(t *testing.T) {
	cases := []struct {
		path, volume string
		abs          bool
	}{
		{`C:\Users`, `C:`, true},
		{`c:/Users`, `c:`, true},
		{`C:`, `C:`, false},
		{`C:x`, `C:`, false},
		{`\\server\share\x`, `\\server\share`, true},
		{`//server/share/x`, `//server/share`, true},
		{`\\server\share`, `\\server\share`, true},
		{`\repo\lib`, ``, false},
		{`/repo/lib`, ``, false},
		{`lib\a.dart`, ``, false},
		{``, ``, false},
	}
	for _, c := range cases {
		if got := Windows.VolumeName(c.path); got != c.volume {
			t.Errorf("Windows.VolumeName(%q) = %q, want %q", c.path, got, c.volume)
		}
		if got := Windows.IsAbs(c.path); got != c.abs {
			t.Errorf("Windows.IsAbs(%q) = %v, want %v", c.path, got, c.abs)
		}
		if runtime.GOOS == "windows" {
			if got := filepath.VolumeName(c.path); got != c.volume {
				t.Errorf("filepath.VolumeName(%q) = %q, the table says %q", c.path, got, c.volume)
			}
			if got := filepath.IsAbs(c.path); got != c.abs {
				t.Errorf("filepath.IsAbs(%q) = %v, the table says %v", c.path, got, c.abs)
			}
		}
	}
	for path, abs := range map[string]bool{"/a": true, "a": false, `C:\a`: false, "": false} {
		if got := POSIX.IsAbs(path); got != abs || POSIX.VolumeName(path) != "" {
			t.Errorf("POSIX.IsAbs(%q) = %v", path, got)
		}
	}
}

func TestAbs(t *testing.T) {
	for _, c := range []struct{ path, dir, want string }{
		{`C:\kb\x`, `D:\work\app`, `C:\kb\x`},
		{`\repo\.ai\plans\..\..\lib\a.dart`, `D:\work\app`, `D:\repo\.ai\plans\..\..\lib\a.dart`},
		{`/repo/lib`, `D:\work\app`, `D:/repo/lib`},
		{`lib\a.dart`, `D:\work\app`, `D:\work\app\lib\a.dart`},
		{`D:lib\a.dart`, `d:\work\app`, `d:\work\app\lib\a.dart`},
		{`C:lib\a.dart`, `D:\work\app`, `C:\lib\a.dart`},
		{`\\server\share\x`, `D:\work`, `\\server\share\x`},
		{`x`, `\\server\share\work`, `\\server\share\work\x`},
		{`\x`, `\\server\share\work`, `\\server\share\x`},
	} {
		if got := Windows.Abs(c.path, c.dir); got != c.want {
			t.Errorf("Windows.Abs(%q, %q) = %q, want %q", c.path, c.dir, got, c.want)
		}
	}
	for _, c := range []struct{ path, dir, want string }{
		{"/repo/lib", "/work/app", "/repo/lib"},
		{"lib/../a", "/work/app", "/work/app/lib/../a"},
	} {
		if got := POSIX.Abs(c.path, c.dir); got != c.want {
			t.Errorf("POSIX.Abs(%q, %q) = %q, want %q", c.path, c.dir, got, c.want)
		}
	}
}

func TestCleanJoinDir(t *testing.T) {
	for in, want := range map[string]string{
		`C:\a\b\..\c`:              `C:\a\c`,
		`c:/a/./b/`:                `C:\a\b`,
		`C:\..\a`:                  `C:\a`,
		`C:\`:                      `C:\`,
		`C:`:                       `C:.`,
		`C:a\..\..\b`:              `C:..\b`,
		`\repo\.ai\plans\..\..\x`:  `\repo\x`,
		`a\..\..\b`:                `..\b`,
		``:                         `.`,
		`\\server\share`:           `\\server\share`,
		`\\server\share\..\x`:      `\\server\share\x`,
		`\\?\C:\a\b\..`:            `C:\a`,
		`C:\Users\me\\.dashcrew\x`: `C:\Users\me\.dashcrew\x`,
	} {
		if got := Windows.Clean(in); got != want {
			t.Errorf("Windows.Clean(%q) = %q, want %q", in, got, want)
		}
		if runtime.GOOS == "windows" && !hasExtendedPrefix(in) {
			if got := filepath.Clean(in); got != want && got != Windows.Normalize(want) && Windows.Normalize(got) != want {
				t.Errorf("filepath.Clean(%q) = %q, the table says %q", in, got, want)
			}
		}
	}
	for _, c := range []struct {
		elem []string
		want string
	}{
		{[]string{`C:\`, "x"}, `C:\x`},
		{[]string{`C:\a`, "..", "b"}, `C:\b`},
		{[]string{`\\server\share\`, "x"}, `\\server\share\x`},
		{[]string{"", ""}, ""},
		{[]string{`C:\a`, "", `b/c`}, `C:\a\b\c`},
	} {
		if got := Windows.Join(c.elem...); got != c.want {
			t.Errorf("Windows.Join(%q) = %q, want %q", c.elem, got, c.want)
		}
	}
	for in, want := range map[string]string{
		`C:\a\b`:              `C:\a`,
		`C:\a`:                `C:\`,
		`C:\`:                 `C:\`,
		`c:/a/b/`:             `C:\a\b`,
		`\\server\share\x`:    `\\server\share\`,
		`\\server\share\x\y`:  `\\server\share\x`,
		`relative`:            `.`,
		`\repo\lib\a.dart`:    `\repo\lib`,
		`C:\Users\me\..\x\y`:  `C:\Users\x`,
		`\\?\C:\Users\me\kb`:  `C:\Users\me`,
		`C:\Users\me\kb\.\..`: `C:\Users\me\kb`,
	} {
		if got := Windows.Dir(in); got != want {
			t.Errorf("Windows.Dir(%q) = %q, want %q", in, got, want)
		}
	}
	for _, in := range []string{"/a/b/../c", "a//b/", "/", "", "../../x", "/../x"} {
		if got, want := POSIX.Clean(in), filepath.ToSlash(filepath.Clean(filepath.FromSlash(in))); runtime.GOOS != "windows" && got != want {
			t.Errorf("POSIX.Clean(%q) = %q, filepath %q", in, got, want)
		}
		if got, want := POSIX.Dir(in), filepath.Dir(in); runtime.GOOS != "windows" && got != want {
			t.Errorf("POSIX.Dir(%q) = %q, filepath %q", in, got, want)
		}
	}
}

func hasExtendedPrefix(p string) bool {
	return len(p) >= 4 && (p[:4] == `\\?\` || p[:4] == `\??\`)
}

func TestWithinAndSame(t *testing.T) {
	for _, c := range []struct {
		parent, child, relative string
		inside                  bool
	}{
		{`C:\Users\me\kb`, `C:\Users\me\kb\features\x.md`, "features/x.md", true},
		{`C:\Users\me\kb`, `c:/users/ME/KB/inbox/x.md`, "inbox/x.md", true},
		{`C:\Users\me\kb`, `\\?\C:\Users\me\kb\specs\1.0`, "specs/1.0", true},
		{`C:\Users\me\kb`, `C:\Users\me\kb`, "", true},
		{`C:\Users\me\kb`, `C:\Users\me\kb-old\x`, "", false},
		{`C:\Users\me\kb`, `D:\Users\me\kb\x`, "", false},
		{`C:\`, `C:\x\y`, "x/y", true},
		{`\\server\share\kb`, `\\SERVER\share\kb\x`, "x", true},
	} {
		relative, inside := Windows.Within(c.parent, c.child)
		if relative != c.relative || inside != c.inside {
			t.Errorf("Windows.Within(%q, %q) = %q, %v; want %q, %v", c.parent, c.child, relative, inside, c.relative, c.inside)
		}
	}
	if relative, inside := POSIX.Within("/home/me/kb", "/home/me/KB/x"); inside || relative != "" {
		t.Error("POSIX folds case")
	}
	if relative, inside := Darwin.Within("/home/me/kb", "/home/me/KB/x"); !inside || relative != "x" {
		t.Error("Darwin does not fold case")
	}
	if relative, inside := POSIX.Within("/", "/a/b"); !inside || relative != "a/b" {
		t.Errorf("POSIX.Within root: %q %v", relative, inside)
	}

	for _, pair := range [][2]string{
		{`C:/Users/me/app.worktrees/feat-a`, `C:\Users\me\app.worktrees\feat-a`},
		{`c:\users\me\app`, `C:\Users\me\app`},
		{`\\?\C:\Users\me\app`, `C:/Users/me/app`},
	} {
		if !Windows.Same(pair[0], pair[1]) {
			t.Errorf("Windows.Same(%q, %q) is false", pair[0], pair[1])
		}
	}
	if Windows.Same(`C:\Users\me\app`, `C:\Users\me\app2`) || POSIX.Same("/a/B", "/a/b") || !Darwin.Same("/a/B", "/a/b") {
		t.Error("Same matches different paths")
	}
}

func TestMSYSDrive(t *testing.T) {
	for in, want := range map[string]string{
		"/c/Users/me/x": `C:\Users\me\x`,
		"/d":            `D:\`,
		"/d/":           `D:\`,
	} {
		if got, ok := MSYSDrive(in); !ok || got != want {
			t.Errorf("MSYSDrive(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"/repo/lib", "/", "c/Users", `C:\x`, "/cd/x", ""} {
		if got, ok := MSYSDrive(in); ok {
			t.Errorf("MSYSDrive(%q) = %q", in, got)
		}
	}
}

func TestForGOOS(t *testing.T) {
	if ForGOOS("windows") != Windows || ForGOOS("darwin") != Darwin || ForGOOS("linux") != POSIX {
		t.Error("ForGOOS")
	}
}
