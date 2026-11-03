// Package ports checks that the fixes of an older release line reached the newer one (`dashcrew check --ports`):
// merged upward, or ported with a `Ported-from:` trailer, a `cherry-pick -x` note or an equal patch-id, and not
// reverted afterwards. It only reads git.
package ports

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/commit"
	"github.com/fahnaladitia/dashcrew/internal/lines"
	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// Status is how a fix was found on the newer line, in the order Check tries.
type Status int

const (
	// Missing: no port on the newer line.
	Missing Status = iota
	// Merged: the fix itself is in the newer line's history.
	Merged
	// Trailer: a commit on the newer line carries `Ported-from: <the fix>`.
	Trailer
	// CherryPick: a commit on the newer line carries `(cherry picked from commit <the fix>)`.
	CherryPick
	// PatchID: a commit on the newer line has the fix's `git patch-id --stable`.
	PatchID
	// Lost: found, then reverted on the newer line.
	Lost
)

func (s Status) String() string {
	return [...]string{"missing", "merged", "trailer", "cherry-pick", "patch-id", "lost"}[s]
}

// Fix is one fix commit of the older line and what became of it on the newer one.
type Fix struct {
	Hash, Subject string
	Status        Status
	// By is the commit that brings the fix to the newer line: the fix itself when merged.
	By string
	// RevertedBy is the newer line's commit that reverts By, for Lost.
	RevertedBy string
}

// Pair is the result for one older and one newer line.
type Pair struct {
	From, To lines.Line
	// Base is where the older line left the main line (lines.BranchPoint) and BaseFrom the branch it left.
	Base, BaseFrom string
	Fixes          []Fix
}

// Problems counts the fixes missing or lost.
func (p Pair) Problems() int {
	n := 0
	for _, fix := range p.Fixes {
		if fix.Status == Missing || fix.Status == Lost {
			n++
		}
	}
	return n
}

// Lines is the text report: a header, then one line per fix.
func (p Pair) Lines() []string {
	since := "since the first commit"
	if p.Base != "" {
		since = "since " + p.BaseFrom + " at " + short(p.Base)
	}
	header := fmt.Sprintf("%s -> %s: %s %s", p.From.Branch, p.To.Branch, plural(len(p.Fixes), "fix", "fixes"), since)
	if n := p.Problems(); n > 0 {
		header += fmt.Sprintf(", %d missing or lost", n)
	}
	out := []string{header}
	for _, fix := range p.Fixes {
		out = append(out, "  "+fix.Line())
	}
	return out
}

// Line is one fix: its state, short hash and subject, and the commit that ported or reverted it.
func (f Fix) Line() string {
	state, how := "ok", ""
	switch f.Status {
	case Missing:
		state = "missing"
	case Merged:
		how = "merged"
	case Trailer:
		how = "Ported-from: in " + short(f.By)
	case CherryPick:
		how = "cherry-pick -x in " + short(f.By)
	case PatchID:
		how = "same patch-id as " + short(f.By)
	case Lost:
		state, how = "lost", "ported as "+short(f.By)+", reverted by "+short(f.RevertedBy)
		if f.By == f.Hash {
			how = "merged, reverted by " + short(f.RevertedBy)
		}
	}
	line := fmt.Sprintf("%-8s %s %s", state, short(f.Hash), f.Subject)
	if how != "" {
		line += " (" + how + ")"
	}
	return line
}

func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

func short(sha string) string { return sha[:min(7, len(sha))] }

// logged is one commit with what the check reads from it.
type logged struct {
	hash, message string
	parsed        commit.Parsed
}

var (
	cherryPicked = regexp.MustCompile(`\(cherry picked from commit ([0-9a-f]{7,40})\)`)
	reverts      = regexp.MustCompile(`This reverts commit ([0-9a-f]{7,40})`)
)

// Check lists every fix of from since its branch point (a commit of type `fix`, or with `Port: yes`, and not
// reverted on from itself) and how it reached to. all is lines.List's result.
func Check(dir string, cfg *profile.Commit, all []lines.Line, from, to lines.Line) (Pair, error) {
	pair := Pair{From: from, To: to}
	pair.Base, pair.BaseFrom = lines.BranchPoint(dir, all, from)
	ownRange := from.Head
	if pair.Base != "" {
		ownRange = pair.Base + ".." + from.Head
	}
	own, err := log(dir, cfg, ownRange)
	if err != nil {
		return Pair{}, err
	}
	newer, err := log(dir, cfg, from.Head+".."+to.Head)
	if err != nil {
		return Pair{}, err
	}
	patchIDs, err := patchIDs(dir, ownRange, from.Head+".."+to.Head)
	if err != nil {
		return Pair{}, err
	}
	revertedOnOwn := revertsIn(own)
	revertedOnNewer := revertsIn(newer)
	for _, c := range own {
		if !isFix(c) || matchesAny(revertedOnOwn, c.hash) != "" {
			continue
		}
		fix := Fix{Hash: c.hash, Subject: c.parsed.Subject}
		candidates := find(dir, c, to.Head, newer, patchIDs)
		for i, candidate := range candidates {
			revert := matchesAny(revertedOnNewer, candidate.by)
			if revert == "" && candidate.status == Merged {
				revert = revertOfMerge(dir, c.hash, revertedOnNewer)
			}
			if revert == "" || i == len(candidates)-1 {
				fix.Status, fix.By, fix.RevertedBy = candidate.status, candidate.by, revert
				if revert != "" {
					fix.Status = Lost
				}
			}
			if revert == "" {
				break
			}
		}
		pair.Fixes = append(pair.Fixes, fix)
	}
	return pair, nil
}

func isFix(c logged) bool {
	return c.parsed.Type == "fix" || c.parsed.Trailer(commit.PortTrailer) == "yes"
}

type candidate struct {
	status Status
	by     string
}

// find is every commit that brings c to the newer line, in the order of Status.
func find(dir string, c logged, toHead string, newer []logged, patchIDs map[string]string) []candidate {
	keys := []string{c.hash}
	keys = append(keys, c.parsed.Values(commit.PortedFromTrailer)...)
	for _, m := range cherryPicked.FindAllStringSubmatch(c.message, -1) {
		keys = append(keys, m[1])
	}
	var found []candidate
	if _, err := git(dir, "merge-base", "--is-ancestor", c.hash, toHead); err == nil {
		found = append(found, candidate{Merged, c.hash})
	}
	for _, n := range newer {
		for _, value := range n.parsed.Values(commit.PortedFromTrailer) {
			if sameCommit(keys, value) {
				found = append(found, candidate{Trailer, n.hash})
				break
			}
		}
	}
	for _, n := range newer {
		for _, m := range cherryPicked.FindAllStringSubmatch(n.message, -1) {
			if sameCommit(keys, m[1]) {
				found = append(found, candidate{CherryPick, n.hash})
				break
			}
		}
	}
	if id := patchIDs[c.hash]; id != "" {
		for _, n := range newer {
			if patchIDs[n.hash] == id {
				found = append(found, candidate{PatchID, n.hash})
			}
		}
	}
	if len(found) == 0 {
		return []candidate{{Missing, ""}}
	}
	return found
}

// sameCommit is true when ref, a full or abbreviated hash of at least 7 characters, names one of keys.
func sameCommit(keys []string, ref string) bool {
	if len(ref) < 7 {
		return false
	}
	for _, key := range keys {
		if len(key) >= 7 && (strings.HasPrefix(key, ref) || strings.HasPrefix(ref, key)) {
			return true
		}
	}
	return false
}

// revertsIn maps each reverted commit, as the revert message names it, to the revert; a revert that is itself
// reverted later in the same list no longer counts.
func revertsIn(commits []logged) map[string]string {
	out := map[string]string{}
	for _, c := range commits {
		for _, m := range reverts.FindAllStringSubmatch(c.message, -1) {
			out[m[1]] = c.hash
		}
	}
	for target, revert := range out {
		if undo := matchesAny(out, revert); undo != "" {
			delete(out, target)
		}
	}
	return out
}

// revertOfMerge is the revert, in reverted, of a merge commit that brought sha in: a merge whose first parent lacks
// sha and whose other parent has it. `git revert -m 1` of a port merge undoes the fix while sha stays in history,
// which ancestry alone reads as merged. Empty when no such revert exists.
func revertOfMerge(dir, sha string, reverted map[string]string) string {
	for target, revert := range reverted {
		out, err := git(dir, "rev-list", "--parents", "-n", "1", target)
		if err != nil {
			continue
		}
		parents := strings.Fields(out)
		if len(parents) < 3 {
			continue
		}
		if _, err := git(dir, "merge-base", "--is-ancestor", sha, parents[1]); err == nil {
			continue
		}
		for _, parent := range parents[2:] {
			if _, err := git(dir, "merge-base", "--is-ancestor", sha, parent); err == nil {
				return revert
			}
		}
	}
	return ""
}

// matchesAny is the revert of sha in reverted; empty when none.
func matchesAny(reverted map[string]string, sha string) string {
	for target, revert := range reverted {
		if sameCommit([]string{sha}, target) {
			return revert
		}
	}
	return ""
}

func git(dir string, args ...string) (string, error) {
	return gitInput(dir, nil, args...)
}

func gitInput(dir string, stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return stdout.String(), nil
}

// log is the commits of revRange oldest first, merges left out.
func log(dir string, cfg *profile.Commit, revRange string) ([]logged, error) {
	out, err := git(dir, "log", "--no-merges", "--reverse", "-z", "--format=%H%n%B", revRange, "--")
	if err != nil {
		return nil, err
	}
	var commits []logged
	for _, entry := range strings.Split(out, "\x00") {
		hash, message, ok := strings.Cut(entry, "\n")
		if !ok || hash == "" {
			continue
		}
		commits = append(commits, logged{hash: hash, message: message, parsed: commit.Parse(cfg, message)})
	}
	return commits, nil
}

// patchIDs maps each commit of the ranges to its `git patch-id --stable`; a commit without changes has none.
func patchIDs(dir string, revRanges ...string) (map[string]string, error) {
	out := map[string]string{}
	for _, revRange := range revRanges {
		diff, err := git(dir, "log", "--no-merges", "-p", "--no-color", "--no-ext-diff", "--format=commit %H", revRange, "--")
		if err != nil {
			return nil, err
		}
		ids, err := gitInput(dir, []byte(diff), "patch-id", "--stable")
		if err != nil {
			return nil, err
		}
		for _, row := range strings.Split(ids, "\n") {
			if id, hash, ok := strings.Cut(row, " "); ok {
				out[hash] = id
			}
		}
	}
	return out, nil
}
