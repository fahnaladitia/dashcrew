package guard

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"runtime"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/guard/shell"
	"github.com/fahnaladitia/dashcrew/internal/knowledge"
	"github.com/fahnaladitia/dashcrew/internal/pathstyle"
	"github.com/fahnaladitia/dashcrew/internal/spec"
)

// The rules below are Go only (K2, P7). Write paths are resolved to their real path, following symbolic links over
// the part of the path that exists, before the write rules run. A write whose real path is inside the knowledge
// clone is allowed only under its inbox, and under specs/ to a file that does not exist yet or whose front matter
// status is draft or review, for every agent and capability; reading is never blocked.

// knowledgeReason is the block reason of a write into the knowledge clone outside its inbox and specs.
const knowledgeReason = "The knowledge clone is read-only for agents except inbox/. " +
	"Write a proposal under .ai/knowledge/inbox/ instead; pages change through review."

// specReason is the block reason of a write under specs/ to a file that exists and is not draft or review.
const specReason = "Under specs/, agents write only new files and files whose status is draft or review. " +
	"An approved or superseded spec changes after a member runs dashcrew spec revise <folder>."

// specReviseReason is the block reason of an agent running `dashcrew spec revise`.
const specReviseReason = "dashcrew spec revise reopens an approved spec; a member runs it. Ask the user."

// maxLinks is how many symbolic links realPath follows before it treats the rest of the path as plain names.
const maxLinks = 40

// knowledgeRoots is where the hook runs and the knowledge folders, as real paths, on this machine. Every path is
// spelled and compared in style, and links are read through fsys, so the rules are the same for a Windows path
// tested on any host.
type knowledgeRoots struct {
	style     pathstyle.Style
	fsys      pathFS
	cwd, home string
	// roots are the clone and the project's `.ai/knowledge` (a link to the clone, or a copy of it).
	roots []string
}

// at returns p answering for a hook that runs in cwd, in project (empty when the policy is not the project's
// `.ai/guard.json`), for a user whose home folder is home. Without cwd, relative paths are left to the text rules.
func (p Policy) at(cwd, project, home string) Policy {
	if cwd == "" {
		return p
	}
	p.knowledge = newKnowledgeRoots(pathstyle.Host, osFS{}, cwd, project, home, p.KnowledgeClone)
	return p
}

// newKnowledgeRoots resolves clone (`~/...` or a path) and the project's `.ai/knowledge` on a machine whose paths
// are spelled in s. Git Bash spellings of cwd, project and home (`/c/Users/me`) are read as Windows paths.
func newKnowledgeRoots(s pathstyle.Style, fsys pathFS, cwd, project, home, clone string) *knowledgeRoots {
	k := &knowledgeRoots{style: s, fsys: fsys, cwd: nativeDir(s, cwd), home: nativeDir(s, home)}
	if clone != "" {
		if !strings.HasPrefix(clone, "~/") || home != "" {
			k.roots = append(k.roots, k.realPath(k.absolute(k.expandHome(clone))))
		}
		if project != "" {
			if link := k.realPath(s.Join(nativeDir(s, project), s.FromSlash(knowledge.LinkPath))); !k.containsPath(k.roots, link) {
				k.roots = append(k.roots, link)
			}
		}
	}
	return k
}

// nativeDir is a folder the hook or the environment named, spelled as s spells it.
func nativeDir(s pathstyle.Style, dir string) string {
	if s.Windows {
		if native, ok := pathstyle.MSYSDrive(dir); ok {
			return native
		}
	}
	return s.Normalize(dir)
}

// expandHome is the policy's clone path with a leading `~/` replaced by the home folder.
func (k *knowledgeRoots) expandHome(clone string) string {
	if rest, ok := strings.CutPrefix(clone, "~/"); ok {
		return k.style.Join(k.home, k.style.FromSlash(rest))
	}
	return k.style.FromSlash(clone)
}

func (k *knowledgeRoots) containsPath(paths []string, path string) bool {
	for _, item := range paths {
		if k.style.Same(item, path) {
			return true
		}
	}
	return false
}

// absolute is path joined to the working folder and cleaned, unless it is absolute already.
func (k *knowledgeRoots) absolute(path string) string {
	if k.style.IsAbs(path) {
		return path
	}
	return k.style.Clean(k.style.Abs(path, k.cwd))
}

// pathFS is what the knowledge rules read from the file system: whether a name exists and is a symbolic link, a
// link's target, and the first bytes of a file.
type pathFS interface {
	lstat(name string) (exists, link bool)
	readlink(name string) (string, error)
	// head is the first n bytes of the file name; an error wrapping fs.ErrNotExist when nothing is there.
	head(name string, n int) ([]byte, error)
}

// osFS is the real file system.
type osFS struct{}

func (osFS) lstat(name string) (bool, bool) {
	info, err := os.Lstat(name)
	if err != nil {
		return false, false
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return true, true
	}
	// Go reports a Windows junction as irregular, not as a link; it redirects a path all the same.
	if runtime.GOOS == "windows" && info.Mode()&fs.ModeIrregular != 0 {
		if _, err := os.Readlink(name); err == nil {
			return true, true
		}
	}
	return true, false
}

func (osFS) readlink(name string) (string, error) { return os.Readlink(name) }

func (osFS) head(name string, n int) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buffer := make([]byte, n)
	read, err := io.ReadFull(file, buffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buffer[:read], nil
}

// realPath is path with every symbolic link resolved, as the kernel walks it, as far as the path exists; the
// missing rest is joined and cleaned. A `..` after a link climbs from the link's target. path must be absolute.
func (k *knowledgeRoots) realPath(path string) string {
	s := k.style
	path = s.Normalize(path)
	volume := s.VolumeName(path)
	resolved := volume + s.Separator()
	parts := k.splitPath(path[len(volume):])
	links := 0
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if part == ".." {
			resolved = s.Dir(resolved)
			continue
		}
		next := s.Join(resolved, part)
		exists, link := k.fsys.lstat(next)
		if !exists || (link && links >= maxLinks) {
			return s.Clean(s.Join(append([]string{next}, parts[i+1:]...)...))
		}
		if !link {
			resolved = next
			continue
		}
		links++
		target, err := k.fsys.readlink(next)
		if err != nil {
			return s.Clean(s.Join(append([]string{next}, parts[i+1:]...)...))
		}
		target = s.Normalize(target)
		if targetVolume := s.VolumeName(target); targetVolume != "" || strings.HasPrefix(target, s.Separator()) {
			if targetVolume != "" {
				volume = targetVolume
			}
			resolved = volume + s.Separator()
			target = target[len(targetVolume):]
		}
		parts = append(k.splitPath(target), parts[i+1:]...)
		i = -1
	}
	return resolved
}

func (k *knowledgeRoots) splitPath(path string) []string {
	var parts []string
	separator := k.style.Separator()[0]
	for _, part := range strings.FieldsFunc(k.style.FromSlash(path), func(r rune) bool { return r == rune(separator) }) {
		if part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// expand is a path the agent wrote: `~/`, `$HOME/` and `${HOME}/` become the home folder, a relative path is
// joined to dir, and on Windows a path rooted without a volume takes dir's. Nothing is cleaned, so realPath still
// walks a `..` after a link as the kernel does.
func (k *knowledgeRoots) expand(path, dir string) string {
	s := k.style
	path = s.Normalize(path)
	if k.home != "" {
		for _, prefix := range []string{"~/", "$HOME/", "${HOME}/"} {
			if rest, ok := strings.CutPrefix(path, s.FromSlash(prefix)); ok {
				return k.home + s.Separator() + rest
			}
		}
	}
	return s.Abs(path, dir)
}

// spellings are the absolute paths a written path may name from dir: expand's, and on Windows also the Git Bash
// reading of `/c/...` as `C:\...`, so a shell target is checked both ways.
func (k *knowledgeRoots) spellings(path, dir string) []string {
	full := []string{k.expand(path, dir)}
	if k.style.Windows {
		if native, ok := pathstyle.MSYSDrive(path); ok {
			full = append(full, native)
		}
	}
	return full
}

// blocks is the reason a write to path is refused, empty when it is allowed. path is resolved from dir as the
// kernel would, and after cleaning `..` first as some tools do; inside a knowledge root it is allowed only in the
// inbox, and under specs/ where specWritable says so.
func (k *knowledgeRoots) blocks(path, dir string) string {
	if len(k.roots) == 0 || path == "" {
		return ""
	}
	for _, full := range k.spellings(path, dir) {
		for _, candidate := range []string{k.realPath(full), k.realPath(k.style.Clean(full))} {
			for _, root := range k.roots {
				relative, inside := k.style.Within(root, candidate)
				switch {
				case !inside || relative == knowledge.InboxDir || strings.HasPrefix(relative, knowledge.InboxDir+"/"):
				case relative == spec.Dir || strings.HasPrefix(relative, spec.Dir+"/"):
					if !specWritable(k.fsys, candidate) {
						return specReason
					}
				default:
					return knowledgeReason
				}
			}
		}
	}
	return ""
}

// specWritable is true for a path that does not exist yet, or a file whose front matter status, read from its first
// 2 KB, is draft or review. A folder that exists is not writable: a command on it reaches every file inside.
func specWritable(fsys pathFS, path string) bool {
	data, err := fsys.head(path, 2048)
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	return spec.Writable(spec.StatusOf(data))
}

// appPath is the write path as the text rules read it, after resolving links: relative to the real working folder
// when it is inside it; false when it says nothing new.
func (k *knowledgeRoots) appPath(path string) (string, bool) {
	real := k.realPath(k.expand(path, k.cwd))
	relative, inside := k.style.Within(k.realPath(k.cwd), real)
	if !inside || relative == "" || relative == k.style.ToSlash(path) {
		return "", false
	}
	return relative, true
}

// shellWrites blocks a script, or an argv list, that writes into the knowledge clone outside its inbox: output
// redirections and the targets of tee, cp, mv, rm, touch, mkdir, ln, sed -i, dd, find -delete and the like,
// through nested shells and wrappers. Relative targets are tried from the hook's folder and from every folder the
// script changes to with cd or pushd.
func (k *knowledgeRoots) shellWrites(script string, argv []string) Decision {
	var targets []string
	dirs := []string{k.cwd}
	collectScript(script, 0, &targets, &dirs, k)
	if argv != nil {
		collectArgv(argv, 0, &targets, &dirs, k)
	}
	for _, target := range targets {
		if target == specReviseReason {
			return Block(specReviseReason)
		}
		for _, dir := range dirs {
			if reason := k.blocks(target, dir); reason != "" {
				return Block(reason)
			}
		}
	}
	return Allow()
}

func collectScript(script string, depth int, targets, dirs *[]string, k *knowledgeRoots) {
	if depth > maxNesting {
		return
	}
	commands, writes := shell.Scan(script)
	*targets = append(*targets, writes...)
	for _, argv := range commands {
		collectArgv(argv, depth, targets, dirs, k)
	}
}

func collectArgv(argv []string, depth int, targets, dirs *[]string, k *knowledgeRoots) {
	if depth > maxNesting {
		return
	}
	inner := unwrap(argv)
	if len(inner.argv) > 0 {
		program := baseName(inner.argv[0])
		if program == "cd" || program == "pushd" {
			if dir := operands(inner.argv[1:]); len(dir) > 0 {
				*dirs = append(*dirs, k.spellings(dir[0], (*dirs)[len(*dirs)-1])...)
			}
		}
		*targets = append(*targets, commandWrites(program, inner.argv[1:])...)
		if program == "dashcrew" && len(inner.argv) > 2 && inner.argv[1] == "spec" && inner.argv[2] == "revise" {
			*targets = append(*targets, specReviseReason)
		}
	}
	for _, script := range inner.scripts {
		collectScript(script, depth+1, targets, dirs, k)
	}
	for _, command := range inner.commands {
		collectArgv(command, depth+1, targets, dirs, k)
	}
}

// operands are the words that are not options; every word after `--` is one. A lone `-` is an operand.
func operands(args []string) []string {
	out := []string{}
	for i, word := range args {
		if word == "--" {
			return append(out, args[i+1:]...)
		}
		if len(word) > 1 && word[0] == '-' {
			continue
		}
		out = append(out, word)
	}
	return out
}

// targetDirectory is the value of `-t DIR`, `-tDIR`, `--target-directory DIR` or `--target-directory=DIR`.
func targetDirectory(args []string) (string, bool) {
	for i, word := range args {
		switch {
		case word == "--":
			return "", false
		case word == "-t" || word == "--target-directory":
			if i+1 < len(args) {
				return args[i+1], true
			}
		case strings.HasPrefix(word, "--target-directory="):
			return strings.TrimPrefix(word, "--target-directory="), true
		case strings.HasPrefix(word, "-t") && len(word) > 2 && !strings.HasPrefix(word, "--"):
			return word[2:], true
		}
	}
	return "", false
}

// commandWrites is every path a file command writes or removes. Operands are over-counted (an option's value may
// be read as a path), never under-counted, so a target is not missed.
func commandWrites(program string, args []string) []string {
	switch program {
	case "tee", "rm", "rmdir", "unlink", "touch", "mkdir", "truncate", "shred", "mv":
		files := operands(args)
		if dir, ok := targetDirectory(args); ok && program == "mv" {
			files = append(files, dir)
		}
		return files
	case "cp", "install", "ln", "rsync":
		if dir, ok := targetDirectory(args); ok {
			return []string{dir}
		}
		if files := operands(args); len(files) > 1 {
			return files[len(files)-1:]
		}
	case "dd":
		var files []string
		for _, word := range args {
			if file, ok := strings.CutPrefix(word, "of="); ok {
				files = append(files, file)
			}
		}
		return files
	case "sed":
		inPlace := false
		for _, word := range args {
			if word == "--" {
				break
			}
			if strings.HasPrefix(word, "--in-place") || (strings.HasPrefix(word, "-") && !strings.HasPrefix(word, "--") && strings.Contains(word, "i")) {
				inPlace = true
			}
		}
		if inPlace {
			return operands(args)
		}
	case "find":
		for _, word := range args {
			if word == "-delete" {
				var starts []string
				for _, start := range args {
					if strings.HasPrefix(start, "-") || start == "(" || start == "!" {
						break
					}
					starts = append(starts, start)
				}
				return starts
			}
		}
	}
	return nil
}
