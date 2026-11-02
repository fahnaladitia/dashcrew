package sync

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/knowledge"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// The `.gitignore` block dashcrew owns: written only while it has entries, replaced in place, never touched
// outside its markers.
const (
	ignoreBegin = "# dashcrew:begin - managed by dashcrew sync, do not edit"
	ignoreEnd   = "# dashcrew:end"
)

// Lock states of the knowledge link.
const (
	knowledgeLinked   = "linked"
	knowledgeCopied   = "copied"
	knowledgeMissing  = "missing"
	knowledgeUnlinked = "unlinked"
)

// Seams for tests: Windows copies the clone instead of linking it, as does a failed symlink.
var (
	goos    = runtime.GOOS
	symlink = os.Symlink
)

// home is the folder holding `.dashcrew/knowledge`: Sync.Home, else the user's home folder.
func (s Sync) home() (string, error) {
	if s.Home != "" {
		return s.Home, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", &Error{Message: "No home folder for the knowledge clone: " + err.Error()}
	}
	return home, nil
}

// syncKnowledge creates, repairs, copies or removes `.ai/knowledge` and returns the lock entry (nil without
// knowledge). It never touches the network: a missing clone is a warning naming `dashcrew knowledge pull`.
func (s Sync) syncKnowledge(k *profile.Knowledge, previous *omap.Map[any], result *Result) (*omap.Map[any], error) {
	link := s.project().Path(knowledge.LinkPath)
	previousState := ""
	if previous != nil {
		previousState, _ = previous.Value("state").(string)
	}
	if k == nil {
		removed, err := removeOwnedLink(link, previousState)
		if removed {
			result.Removed = append(result.Removed, knowledge.LinkPath)
		}
		return nil, err
	}

	entry := omap.Of[any]("name", k.Name, "link", knowledge.LinkPath)
	home, err := s.home()
	if err != nil {
		return nil, err
	}
	clone := knowledge.CloneDir(home, k.Name)
	if info, err := os.Stat(clone); err != nil || !info.IsDir() {
		result.Warnings = append(result.Warnings, "knowledge clone "+k.Name+" is missing ("+knowledge.PolicyClonePath(k.Name)+
			"); run dashcrew knowledge pull")
		entry.Set("state", knowledgeMissing)
		return entry, nil
	}
	var head any
	if sha, err := knowledge.HeadSHA(clone); err == nil && sha != "" {
		head = sha
	}

	state, wrote, err := placeKnowledge(link, clone, previousState, head, previous)
	if err != nil {
		return nil, err
	}
	if state == knowledgeUnlinked {
		result.Warnings = append(result.Warnings, knowledge.LinkPath+" exists and was not made by dashcrew; left as is. "+
			"Move it away and run dashcrew sync to link the knowledge clone.")
	}
	if wrote {
		result.Written = append(result.Written, knowledge.LinkPath)
	}
	entry.Set("state", state)
	entry.Set("target", clone)
	entry.Set("head", head)
	return entry, nil
}

// placeKnowledge makes link a symlink to clone, or a copy of it on Windows or when symlinks fail. A symlink is
// always repaired; a folder is replaced only when the lock says dashcrew copied it.
func placeKnowledge(link, clone, previousState string, head any, previous *omap.Map[any]) (string, bool, error) {
	info, err := os.Lstat(link)
	exists := err == nil
	isLink := exists && info.Mode()&fs.ModeSymlink != 0
	ownCopy := exists && info.IsDir() && previousState == knowledgeCopied
	if exists && !isLink && !ownCopy {
		return knowledgeUnlinked, false, nil
	}
	if isLink {
		if target, err := os.Readlink(link); err == nil && target == clone && goos != "windows" {
			return knowledgeLinked, false, nil
		}
	}
	if goos != "windows" {
		// The link is made beside the old one first, so a copy is only replaced once a symlink is known to work.
		fresh := link + ".dashcrew-new"
		os.Remove(fresh)
		if err := symlink(clone, fresh); err == nil {
			if err := os.RemoveAll(link); err != nil {
				return "", false, err
			}
			return knowledgeLinked, true, os.Rename(fresh, link)
		}
	}
	if isLink {
		ownCopy = false
	}
	if ownCopy && previous.Value("head") == head && head != nil {
		return knowledgeCopied, false, nil
	}
	if err := os.RemoveAll(link); err != nil {
		return "", false, err
	}
	return knowledgeCopied, true, copyClone(clone, link)
}

// removeOwnedLink removes the link or copy a previous sync made once the profile drops `knowledge`.
func removeOwnedLink(link, previousState string) (bool, error) {
	info, err := os.Lstat(link)
	if err != nil {
		return false, nil
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0 && previousState != "":
		return true, os.Remove(link)
	case info.IsDir() && previousState == knowledgeCopied:
		return true, os.RemoveAll(link)
	}
	return false, nil
}

// copyClone copies the clone's files without its git folder; links inside the clone are skipped.
func copyClone(clone, target string) error {
	return filepath.WalkDir(clone, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(clone, path)
		if relative == ".git" {
			return filepath.SkipDir
		}
		destination := filepath.Join(target, relative)
		switch {
		case entry.IsDir():
			return os.MkdirAll(destination, 0o755)
		case !entry.Type().IsRegular():
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
}

// writeIgnoreBlock puts entries in the dashcrew block of `.gitignore`, or removes the block when there are none;
// a `.gitignore` that held nothing but the block is removed with it.
func (s Sync) writeIgnoreBlock(entries []string) (wrote, removed bool, err error) {
	project := s.project()
	current, err := project.Read(".gitignore")
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, false, err
	}
	next := withIgnoreBlock(current, entries)
	switch {
	case next == current:
		return false, false, nil
	case next == "":
		return false, true, os.Remove(project.Path(".gitignore"))
	}
	wrote, err = project.WriteIfChanged(".gitignore", next)
	return wrote, false, err
}

// withIgnoreBlock is text with the dashcrew block holding entries, in place of an existing block or appended;
// without entries the block is removed. A begin marker without an end marker is left alone.
func withIgnoreBlock(text string, entries []string) string {
	block := ""
	if len(entries) > 0 {
		block = ignoreBegin + "\n" + strings.Join(entries, "\n") + "\n" + ignoreEnd + "\n"
	}
	if begin := strings.Index(text, ignoreBegin+"\n"); begin >= 0 && (begin == 0 || text[begin-1] == '\n') {
		if end := strings.Index(text[begin:], "\n"+ignoreEnd); end >= 0 {
			after := text[begin+end+1+len(ignoreEnd):]
			after = strings.TrimPrefix(after, "\n")
			return text[:begin] + block + after
		}
		if block == "" {
			return text
		}
	}
	if block == "" {
		return text
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + block
}
