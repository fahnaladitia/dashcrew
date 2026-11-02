package doctor

import (
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/knowledge"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// knowledge reports the `.ai/knowledge` link and the clone's last pull, and with Fetch how far the clone is behind
// its remote; false when the profile has no `knowledge`. A missing clone or link warns, never fails.
func (d Doctor) knowledge(lock *omap.Map[any]) (Check, bool) {
	const id = "knowledge"
	p, err := d.parseProfile()
	if err != nil || p.Knowledge == nil {
		return Check{}, false
	}
	home := d.Home
	if home == "" {
		home, _ = homeDir(environment())
	}
	shown := knowledge.PolicyClonePath(p.Knowledge.Name)
	clone := knowledge.CloneDir(home, p.Knowledge.Name)
	if !isDir(clone) {
		return Check{id, Warn, "no clone of " + p.Knowledge.Name + " at " + shown + "; run dashcrew knowledge pull"}, true
	}

	link := d.path(knowledge.LinkPath)
	info, err := os.Lstat(link)
	var state string
	switch {
	case err != nil:
		return Check{id, Warn, knowledge.LinkPath + " is missing; run dashcrew sync"}, true
	case info.Mode()&fs.ModeSymlink != 0:
		if target, _ := os.Readlink(link); target != clone {
			return Check{id, Warn, knowledge.LinkPath + " links to " + target + ", which is not the clone; run dashcrew sync"}, true
		}
		state = "linked to " + shown
	case info.IsDir() && lockedKnowledge(lock, "state") == "copied":
		if head, _ := knowledge.HeadSHA(clone); head != lockedKnowledge(lock, "head") {
			return Check{id, Warn, knowledge.LinkPath + ": the copy is older than the clone; run dashcrew sync"}, true
		}
		state = "copied from " + shown
	default:
		return Check{id, Warn, knowledge.LinkPath + " is not the link dashcrew made; move it away and run dashcrew sync"}, true
	}

	detail := state + ", " + d.pullAge(clone)
	if !d.Fetch {
		return Check{id, OK, detail}, true
	}
	if err := knowledge.Fetch(clone); err != nil {
		return Check{id, Warn, detail + "; fetch failed: " + err.Error()}, true
	}
	behind, err := knowledge.Behind(clone)
	switch {
	case err != nil:
		return Check{id, Warn, detail + "; fetch failed: " + err.Error()}, true
	case behind > 0:
		commits := strconv.Itoa(behind) + " commits"
		if behind == 1 {
			commits = "1 commit"
		}
		return Check{id, Warn, detail + ", " + commits + " behind its remote; run dashcrew knowledge pull"}, true
	}
	return Check{id, OK, detail + ", up to date with its remote"}, true
}

func lockedKnowledge(lock *omap.Map[any], key string) string {
	if lock == nil {
		return ""
	}
	entry, _ := lock.Value("knowledge").(*omap.Map[any])
	if entry == nil {
		return ""
	}
	value, _ := entry.Value(key).(string)
	return value
}

// pullAge is how long ago the clone was last pulled, in the largest whole unit up to days.
func (d Doctor) pullAge(clone string) string {
	pulled, ok := knowledge.LastPull(clone)
	if !ok {
		return "last pull unknown"
	}
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	age := now().Sub(pulled)
	switch hours := int(age.Hours()); {
	case hours < 1:
		return "pulled less than an hour ago"
	case hours == 1:
		return "pulled 1 hour ago"
	case hours < 48:
		return "pulled " + strconv.Itoa(hours) + " hours ago"
	default:
		return "pulled " + strconv.Itoa(hours/24) + " days ago"
	}
}

// knowledgeStale is `dashcrew knowledge doctor` as one warn-level line; false without `knowledge` or a clone, which
// the knowledge check already reports.
func (d Doctor) knowledgeStale() (Check, bool) {
	const id = "knowledge-stale"
	p, err := d.parseProfile()
	if err != nil || p.Knowledge == nil {
		return Check{}, false
	}
	home := d.Home
	if home == "" {
		home, _ = homeDir(environment())
	}
	clone := knowledge.CloneDir(home, p.Knowledge.Name)
	if !isDir(clone) {
		return Check{}, false
	}
	now := time.Now
	if d.Now != nil {
		now = d.Now
	}
	report, err := knowledge.Staleness(clone, d.ProjectDir, now())
	if err != nil {
		return Check{id, Warn, err.Error()}, true
	}
	var parts []string
	for _, part := range []struct {
		n    int
		word string
	}{{len(report.Missing), "stale pointer"}, {len(report.OldProposals), "old proposal"}, {len(report.Unverified), "unverified page"}} {
		if part.n == 1 {
			parts = append(parts, "1 "+part.word)
		} else if part.n > 1 {
			parts = append(parts, strconv.Itoa(part.n)+" "+part.word+"s")
		}
	}
	if len(parts) == 0 {
		return Check{id, OK, "no stale pointers, old proposals or unverified pages"}, true
	}
	return Check{id, Warn, strings.Join(parts, ", ") + "; run dashcrew knowledge doctor"}, true
}
