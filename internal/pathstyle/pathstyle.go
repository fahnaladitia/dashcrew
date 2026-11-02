// Package pathstyle spells and compares file paths as one operating system does, with pure string functions, so
// Windows paths are handled, and tested, the same way on every host.
package pathstyle

import (
	"path"
	"runtime"
	"strings"
)

// Style is how an operating system spells file paths.
type Style struct {
	// Windows: `\` and `/` both separate names, and an absolute path starts with a volume, `C:\` or
	// `\\server\share`.
	Windows bool
	// FoldCase compares names without regard to case.
	FoldCase bool
}

var (
	// POSIX is Linux and the other Unix systems: `/` only, case-sensitive.
	POSIX = Style{}
	// Darwin is POSIX with names compared case-insensitively, as on the default macOS file system.
	Darwin = Style{FoldCase: true}
	// Windows separates with `\` or `/`, starts absolute paths with a volume and compares case-insensitively.
	Windows = Style{Windows: true, FoldCase: true}
	// Host is the style of the operating system dashcrew runs on.
	Host = ForGOOS(runtime.GOOS)
)

// ForGOOS is the style of a GOOS value.
func ForGOOS(goos string) Style {
	switch goos {
	case "windows":
		return Windows
	case "darwin", "ios":
		return Darwin
	}
	return POSIX
}

// Separator is the separator s writes.
func (s Style) Separator() string {
	if s.Windows {
		return `\`
	}
	return "/"
}

func (s Style) isSeparator(c byte) bool { return c == '/' || (s.Windows && c == '\\') }

func isLetter(c byte) bool { return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') }

// FromSlash is p with every `/` written as s's separator.
func (s Style) FromSlash(p string) string {
	if s.Windows {
		return strings.ReplaceAll(p, "/", `\`)
	}
	return p
}

// ToSlash is p with every separator written as `/`.
func (s Style) ToSlash(p string) string {
	if s.Windows {
		return strings.ReplaceAll(p, `\`, "/")
	}
	return p
}

// Normalize is p spelled one way: on Windows with `\` only, without the `\\?\` and `\??\` prefixes of extended
// paths (`\\?\UNC\server` becomes `\\server`) and with an upper-case drive letter. POSIX paths are returned as is.
// Nothing is cleaned.
func (s Style) Normalize(p string) string {
	if !s.Windows {
		return p
	}
	p = strings.ReplaceAll(p, "/", `\`)
	switch {
	case len(p) >= 8 && strings.EqualFold(p[:8], `\\?\UNC\`):
		p = `\\` + p[8:]
	case (strings.HasPrefix(p, `\\?\`) || strings.HasPrefix(p, `\??\`)) && len(p) >= 6 && isLetter(p[4]) && p[5] == ':':
		p = p[4:]
	}
	if len(p) >= 2 && p[1] == ':' && isLetter(p[0]) {
		p = strings.ToUpper(p[:1]) + p[1:]
	}
	return p
}

// VolumeName is the leading volume of p on Windows, `C:` or `\\server\share`; empty on POSIX.
func (s Style) VolumeName(p string) string {
	if !s.Windows {
		return ""
	}
	return p[:s.volumeLen(p)]
}

func (s Style) volumeLen(p string) int {
	if len(p) >= 2 && p[1] == ':' && isLetter(p[0]) {
		return 2
	}
	if len(p) < 3 || !s.isSeparator(p[0]) || !s.isSeparator(p[1]) || s.isSeparator(p[2]) {
		return 0
	}
	n := 2
	for n < len(p) && !s.isSeparator(p[n]) {
		n++
	}
	if n == len(p) {
		return n
	}
	share := n + 1
	end := share
	for end < len(p) && !s.isSeparator(p[end]) {
		end++
	}
	if end == share {
		return n
	}
	return end
}

// IsAbs tells whether p names one place whatever the working folder: `/x` on POSIX, `C:\x` or `\\server\share`
// on Windows, where `\x` alone still depends on the current drive.
func (s Style) IsAbs(p string) bool {
	if !s.Windows {
		return strings.HasPrefix(p, "/")
	}
	n := s.volumeLen(p)
	switch {
	case n == 0:
		return false
	case n > 2:
		return true
	}
	return len(p) > 2 && s.isSeparator(p[2])
}

// IsRooted tells whether p starts from a root: IsAbs, or on Windows a path such as `\x` that starts from the
// current drive's root.
func (s Style) IsRooted(p string) bool {
	return s.IsAbs(p) || (s.Windows && p != "" && s.isSeparator(p[0]))
}

// Abs is p made absolute against dir, which must be absolute: p itself when it is absolute, a Windows path rooted
// without a volume (`\x`) on dir's volume, a drive-relative path (`C:x`) from dir when dir is on that drive and from
// the drive's root otherwise, anything else joined to dir. Nothing is cleaned, so a later walk still meets `..`.
func (s Style) Abs(p, dir string) string {
	if s.IsAbs(p) {
		return p
	}
	if s.Windows {
		if len(p) > 0 && s.isSeparator(p[0]) {
			return s.VolumeName(dir) + p
		}
		if volume := s.VolumeName(p); volume != "" {
			if strings.EqualFold(volume, s.VolumeName(dir)) {
				return dir + s.Separator() + p[len(volume):]
			}
			return volume + s.Separator() + p[len(volume):]
		}
	}
	return dir + s.Separator() + p
}

// Clean is the shortest path naming the same place as p by lexical processing, as path/filepath.Clean does on the
// operating system of s. On Windows the result is also Normalized.
func (s Style) Clean(p string) string {
	if !s.Windows {
		return path.Clean(p)
	}
	p = s.Normalize(p)
	volume := s.VolumeName(p)
	rest := p[len(volume):]
	if rest == "" && len(volume) > 2 {
		return volume
	}
	rooted := rest != "" && rest[0] == '\\'
	var names []string
	for _, name := range strings.Split(rest, `\`) {
		switch name {
		case "", ".":
		case "..":
			switch {
			case len(names) > 0 && names[len(names)-1] != "..":
				names = names[:len(names)-1]
			case !rooted:
				names = append(names, "..")
			}
		default:
			names = append(names, name)
		}
	}
	body := strings.Join(names, `\`)
	switch {
	case rooted:
		return volume + `\` + body
	case body == "":
		return volume + "."
	}
	return volume + body
}

// Join is the non-empty elements joined with s's separator and cleaned; empty when every element is.
func (s Style) Join(elem ...string) string {
	if !s.Windows {
		return path.Join(elem...)
	}
	var parts []string
	for _, e := range elem {
		if e != "" {
			parts = append(parts, e)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return s.Clean(strings.Join(parts, `\`))
}

// Dir is p without its last name, cleaned.
func (s Style) Dir(p string) string {
	if !s.Windows {
		return path.Dir(p)
	}
	p = s.Normalize(p)
	volume := s.VolumeName(p)
	i := len(p) - 1
	for i >= len(volume) && p[i] != '\\' {
		i--
	}
	dir := s.Clean(p[len(volume) : i+1])
	if dir == "." && len(volume) > 2 {
		return volume
	}
	return volume + dir
}

// Within is the slash-separated path of child inside parent, "" for parent itself; false when child is outside.
// Both must be clean and absolute; names are compared as s compares them.
func (s Style) Within(parent, child string) (string, bool) {
	parent, child = s.Normalize(parent), s.Normalize(child)
	p, c := parent, child
	if s.FoldCase {
		p, c = strings.ToLower(p), strings.ToLower(c)
	}
	if p == c {
		return "", true
	}
	prefix := strings.TrimSuffix(p, s.Separator()) + s.Separator()
	if !strings.HasPrefix(c, prefix) || len(prefix) > len(child) {
		return "", false
	}
	return s.ToSlash(child[len(prefix):]), true
}

// Same tells whether a and b are spelled alike once Normalized, ignoring case where s does. Neither is cleaned.
func (s Style) Same(a, b string) bool {
	a, b = s.Normalize(a), s.Normalize(b)
	if s.FoldCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// MSYSDrive is a Git Bash (MSYS) drive path such as `/c/Users/me` spelled as Windows does, `C:\Users\me`; false for
// any other path.
func MSYSDrive(p string) (string, bool) {
	if len(p) < 2 || p[0] != '/' || !isLetter(p[1]) || (len(p) > 2 && p[2] != '/') {
		return "", false
	}
	return strings.ToUpper(p[1:2]) + `:\` + strings.ReplaceAll(strings.TrimPrefix(p[2:], "/"), "/", `\`), true
}
