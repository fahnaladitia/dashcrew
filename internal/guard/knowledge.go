package guard

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/guard/shell"
	"github.com/fahnaladitia/dashcrew/internal/knowledge"
)

// The rules below are Go only (K2). Write paths are resolved to their real path, following symbolic links over
// the part of the path that exists, before the write rules run. A write whose real path is inside the knowledge
// clone is allowed only under its inbox, for every agent and capability; reading is never blocked.

// knowledgeReason is the block reason of a write into the knowledge clone outside its inbox.
const knowledgeReason = "The knowledge clone is read-only for agents except inbox/. " +
	"Write a proposal under .ai/knowledge/inbox/ instead; pages change through review."

// maxLinks is how many symbolic links realPath follows before it treats the rest of the path as plain names.
const maxLinks = 40

// foldCase is true where file names are usually case-insensitive, so a differently cased path still matches.
var foldCase = runtime.GOOS == "darwin" || runtime.GOOS == "windows"

// knowledgeRoots is where the hook runs and the knowledge folders, as real paths, on this machine.
type knowledgeRoots struct {
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
	k := &knowledgeRoots{cwd: cwd, home: home}
	if p.KnowledgeClone != "" {
		if clone := p.KnowledgeClone; !strings.HasPrefix(clone, "~/") || home != "" {
			k.roots = append(k.roots, realPath(absolutePath(knowledge.ExpandHome(clone, home), cwd)))
		}
		if project != "" {
			if link := realPath(filepath.Join(project, filepath.FromSlash(knowledge.LinkPath))); !containsPath(k.roots, link) {
				k.roots = append(k.roots, link)
			}
		}
	}
	p.knowledge = k
	return p
}

func containsPath(paths []string, path string) bool {
	for _, item := range paths {
		if samePath(item, path) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	if foldCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func absolutePath(path, cwd string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(cwd, path)
}

// realPath is path with every symbolic link resolved, as the kernel walks it, as far as the path exists; the
// missing rest is joined and cleaned. path must be absolute.
func realPath(path string) string {
	volume := filepath.VolumeName(path)
	resolved := volume + string(filepath.Separator)
	parts := splitPath(path[len(volume):])
	links := 0
	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if part == ".." {
			resolved = filepath.Dir(resolved)
			continue
		}
		next := filepath.Join(resolved, part)
		info, err := os.Lstat(next)
		if err != nil || (info.Mode()&fs.ModeSymlink != 0 && links >= maxLinks) {
			return filepath.Clean(filepath.Join(append([]string{next}, parts[i+1:]...)...))
		}
		if info.Mode()&fs.ModeSymlink == 0 {
			resolved = next
			continue
		}
		links++
		target, err := os.Readlink(next)
		if err != nil {
			return filepath.Clean(filepath.Join(append([]string{next}, parts[i+1:]...)...))
		}
		if filepath.IsAbs(target) {
			volume = filepath.VolumeName(target)
			resolved = volume + string(filepath.Separator)
			target = target[len(volume):]
		}
		parts = append(splitPath(target), parts[i+1:]...)
		i = -1
	}
	return resolved
}

func splitPath(path string) []string {
	var parts []string
	for _, part := range strings.FieldsFunc(filepath.FromSlash(path), func(r rune) bool { return r == filepath.Separator }) {
		if part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// within is the slash-separated path of child inside parent, "" for parent itself; false when child is outside.
func within(parent, child string) (string, bool) {
	p, c := parent, child
	if foldCase {
		p, c = strings.ToLower(p), strings.ToLower(c)
	}
	if p == c {
		return "", true
	}
	prefix := strings.TrimSuffix(p, string(filepath.Separator)) + string(filepath.Separator)
	if !strings.HasPrefix(c, prefix) {
		return "", false
	}
	return filepath.ToSlash(child[len(prefix):]), true
}

// expand is a path the agent wrote: `~/`, `$HOME/` and `${HOME}/` become the home folder, a relative path is
// joined to dir.
func (k *knowledgeRoots) expand(path, dir string) string {
	if k.home != "" {
		for _, prefix := range []string{"~/", "$HOME/", "${HOME}/"} {
			if rest, ok := strings.CutPrefix(path, prefix); ok {
				path = filepath.Join(k.home, rest)
				break
			}
		}
	}
	return absolutePath(filepath.FromSlash(path), dir)
}

// blocks is true when path, resolved from dir as the kernel would or after cleaning `..` first as some tools do,
// is inside a knowledge root but is neither its inbox folder nor inside it.
func (k *knowledgeRoots) blocks(path, dir string) bool {
	if len(k.roots) == 0 || path == "" {
		return false
	}
	full := k.expand(path, dir)
	for _, candidate := range []string{realPath(full), realPath(filepath.Clean(full))} {
		for _, root := range k.roots {
			relative, inside := within(root, candidate)
			if inside && relative != knowledge.InboxDir && !strings.HasPrefix(relative, knowledge.InboxDir+"/") {
				return true
			}
		}
	}
	return false
}

// appPath is the write path as the text rules read it, after resolving links: relative to the real working folder
// when it is inside it; false when it says nothing new.
func (k *knowledgeRoots) appPath(path string) (string, bool) {
	real := realPath(k.expand(path, k.cwd))
	relative, inside := within(realPath(k.cwd), real)
	if !inside || relative == "" || relative == filepath.ToSlash(path) {
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
		for _, dir := range dirs {
			if k.blocks(target, dir) {
				return Block(knowledgeReason)
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
				*dirs = append(*dirs, k.expand(dir[0], (*dirs)[len(*dirs)-1]))
			}
		}
		*targets = append(*targets, commandWrites(program, inner.argv[1:])...)
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
