// Package lines finds a project's release lines (`project.release_lines`) in git and resolves the line a branch
// belongs to. It only reads git, except RecordRelease, which writes one key of the local branch config.
package lines

import (
	"bytes"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// Line is one line branch.
type Line struct {
	// Kind is profile.MainLine, profile.ReleaseLine or profile.HotfixLine.
	Kind string
	// Version is the pattern's `version` group; empty for the main line.
	Version string
	// Branch is the local name, or `<remote>/<name>` for a line that exists only as a remote-tracking branch.
	Branch string
	// Ref is the full ref name and Head the commit it points at.
	Ref, Head string
}

// How tells how Resolve found a branch's line.
type How int

const (
	// None: detached HEAD, or no line at all.
	None How = iota
	// MainBranch: the branch is the main line.
	MainBranch
	// LineBranch: the branch is a release or hotfix line.
	LineBranch
	// Recorded: the branch records its release in `git config branch.<name>.dashcrew-release`.
	Recorded
	// Guessed: the line nearest to the branch by merge-base; a guess only.
	Guessed
)

// Resolution is the line of one branch.
type Resolution struct {
	// Branch is empty for a detached HEAD.
	Branch string
	How    How
	// Line is nil when no line branch holds the release, or for None.
	Line *Line
	// Version is the release: the line's, or the recorded one; empty for the main line.
	Version string
}

// Protected is true for a release or hotfix line branch and for a branch that records a release: sync keeps their
// toolkit at the lock's version. A guess never protects.
func (r Resolution) Protected() bool { return r.How == LineBranch || r.How == Recorded }

// Describe is one line for the terminal.
func (r Resolution) Describe() string {
	switch r.How {
	case MainBranch:
		return r.Branch + " is the main line"
	case LineBranch:
		return r.Branch + " is the " + r.Line.Kind + " line " + r.Version
	case Recorded:
		where := "no line branch for it"
		if r.Line != nil {
			where = "line " + r.Line.Branch
		}
		return r.Branch + " records release " + r.Version + " (" + ConfigKey(r.Branch) + "), " + where
	case Guessed:
		if r.Line.Kind == profile.MainLine {
			return r.Branch + " is nearest to the main line " + r.Line.Branch + " by merge-base, a guess"
		}
		return r.Branch + " is nearest to " + r.Line.Branch + " by merge-base, a guess; record it with git config " +
			ConfigKey(r.Branch) + " " + r.Version
	}
	if r.Branch == "" {
		return "detached HEAD: no line"
	}
	return r.Branch + ": no line found"
}

// ConfigKey is the branch config key that records a branch's release.
func ConfigKey(branch string) string { return "branch." + branch + ".dashcrew-release" }

// git runs git in dir and returns stdout without its trailing newline.
func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSuffix(stdout.String(), "\n"), nil
}

// Show is `git show <spec>` in dir, such as `HEAD:./.ai/lock.json`; false when git cannot show it.
func Show(dir, spec string) (string, bool) {
	cmd := exec.Command("git", "show", spec)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", false
	}
	return stdout.String(), true
}

// CurrentBranch is the checked-out branch in dir; empty for a detached HEAD or outside a repository.
func CurrentBranch(dir string) string {
	branch, _ := git(dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	return branch
}

// MergeInProgress is true while dir's repository has a MERGE_HEAD.
func MergeInProgress(dir string) bool {
	_, err := git(dir, "rev-parse", "-q", "--verify", "MERGE_HEAD")
	return err == nil
}

// RecordedRelease is the release branch records; empty when it records none.
func RecordedRelease(dir, branch string) string {
	value, _ := git(dir, "config", "--get", ConfigKey(branch))
	return strings.TrimSpace(value)
}

// RecordRelease writes the release into the local branch config, which is never committed.
func RecordRelease(dir, branch, release string) error {
	_, err := git(dir, "config", ConfigKey(branch), release)
	return err
}

// List is every line branch, local and remote-tracking, ordered by version (release before hotfix on a tie), with
// the main line last. A line that exists locally is listed once, by its local branch.
func List(dir string, rl *profile.ReleaseLines) ([]Line, error) {
	out, err := git(dir, "for-each-ref", "--format=%(refname)%00%(objectname)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var versioned []Line
	var main *Line
	for _, row := range strings.Split(out, "\n") {
		ref, head, ok := strings.Cut(row, "\x00")
		if !ok {
			continue
		}
		branch, shown, ok := branchName(ref)
		if !ok {
			continue
		}
		kind, release, ok := rl.Match(branch)
		if !ok || seen[kind+"\x00"+release] {
			continue
		}
		seen[kind+"\x00"+release] = true
		line := Line{Kind: kind, Version: release, Branch: shown, Ref: ref, Head: head}
		if kind == profile.MainLine {
			main = &line
			continue
		}
		versioned = append(versioned, line)
	}
	sort.SliceStable(versioned, func(i, j int) bool {
		if order := version.Compare(versioned[i].Version, versioned[j].Version); order != 0 {
			return order < 0
		}
		return versioned[i].Kind == profile.ReleaseLine && versioned[j].Kind != profile.ReleaseLine
	})
	if main != nil {
		versioned = append(versioned, *main)
	}
	return versioned, nil
}

// branchName is the branch a ref names, without its remote, and how it is shown; false for a remote's HEAD.
func branchName(ref string) (branch, shown string, ok bool) {
	if name, isLocal := strings.CutPrefix(ref, "refs/heads/"); isLocal {
		return name, name, true
	}
	rest, _ := strings.CutPrefix(ref, "refs/remotes/")
	_, name, found := strings.Cut(rest, "/")
	if !found || name == "HEAD" {
		return "", "", false
	}
	return name, rest, true
}

// Main is the main line of all; nil when no branch holds it.
func Main(all []Line) *Line {
	for i := range all {
		if all[i].Kind == profile.MainLine {
			return &all[i]
		}
	}
	return nil
}

// Resolve finds the line of branch (the current branch when empty): its own name when it is a line, else the
// release it records, else the line nearest by merge-base, as a guess. all is List's result.
func Resolve(dir string, rl *profile.ReleaseLines, all []Line, branch string) Resolution {
	if branch == "" {
		branch = CurrentBranch(dir)
		if branch == "" {
			return Resolution{How: None}
		}
	}
	for i := range all {
		if all[i].Branch == branch {
			if all[i].Kind == profile.MainLine {
				return Resolution{Branch: branch, How: MainBranch, Line: &all[i]}
			}
			return Resolution{Branch: branch, How: LineBranch, Line: &all[i], Version: all[i].Version}
		}
	}
	if kind, release, ok := rl.Match(branch); ok {
		if kind == profile.MainLine {
			return Resolution{Branch: branch, How: MainBranch}
		}
		return Resolution{Branch: branch, How: LineBranch, Line: &Line{Kind: kind, Version: release, Branch: branch}, Version: release}
	}
	if release := RecordedRelease(dir, branch); release != "" {
		resolution := Resolution{Branch: branch, How: Recorded, Version: release}
		for i := range all {
			if all[i].Kind != profile.MainLine && all[i].Version == release {
				resolution.Line = &all[i]
				break
			}
		}
		return resolution
	}
	if nearest := nearestLine(dir, all, branch); nearest != nil {
		return Resolution{Branch: branch, How: Guessed, Line: nearest, Version: nearest.Version}
	}
	return Resolution{Branch: branch, How: None}
}

// nearestLine is the line whose merge-base with branch is fewest commits behind it; on a tie, the line whose head is
// closest to that merge-base, then the main line, then the newer version.
func nearestLine(dir string, all []Line, branch string) *Line {
	tip, err := git(dir, "rev-parse", "--verify", "-q", branch+"^{commit}")
	if err != nil {
		return nil
	}
	candidates := make([]*Line, 0, len(all))
	for i := len(all) - 1; i >= 0; i-- {
		candidates = append(candidates, &all[i])
	}
	var best *Line
	bestBehind, bestAhead := 0, 0
	for _, line := range candidates {
		base, err := git(dir, "merge-base", tip, line.Head)
		if err != nil {
			continue
		}
		behind, ahead := count(dir, base+".."+tip), count(dir, base+".."+line.Head)
		if best == nil || behind < bestBehind || (behind == bestBehind && ahead < bestAhead) {
			best, bestBehind, bestAhead = line, behind, ahead
		}
	}
	return best
}

func count(dir, revRange string) int {
	out, err := git(dir, "rev-list", "--count", revRange)
	if err != nil {
		return 1 << 30
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 1 << 30
	}
	return n
}

// Find is the line named by a version, a branch (local, or `<remote>/<name>`, or the name of a remote-only line) or
// `main`; nil when none matches.
func Find(all []Line, name string) *Line {
	if name == profile.MainLine {
		return Main(all)
	}
	for i := range all {
		if all[i].Branch == name || (all[i].Kind != profile.MainLine && all[i].Version == name) {
			return &all[i]
		}
	}
	for i := range all {
		if _, short, ok := strings.Cut(all[i].Branch, "/"); ok && strings.HasPrefix(all[i].Ref, "refs/remotes/") && short == name {
			return &all[i]
		}
	}
	return nil
}

// Pairs are the adjacent lines of all in version order, the last pair ending at the main line.
func Pairs(all []Line) [][2]Line {
	var pairs [][2]Line
	for i := 0; i+1 < len(all); i++ {
		pairs = append(pairs, [2]Line{all[i], all[i+1]})
	}
	return pairs
}

// BranchPoint is the commit where line left the main line: the newest commit of its history on the main line's
// first-parent chain, so a later merge of the line back into the main line does not move it. Without a main line
// branch it is the merge-base with the previous line. Empty base means the line's whole history.
func BranchPoint(dir string, all []Line, line Line) (base, from string) {
	main := Main(all)
	if main == nil || main.Branch == line.Branch {
		return mergeBaseWithPrevious(dir, all, line)
	}
	chain, err := git(dir, "rev-list", "--first-parent", main.Head)
	if err != nil {
		return "", ""
	}
	onMain := map[string]bool{}
	for _, sha := range strings.Split(chain, "\n") {
		onMain[sha] = true
	}
	history, err := git(dir, "rev-list", "--topo-order", line.Head)
	if err != nil {
		return "", ""
	}
	for _, sha := range strings.Split(history, "\n") {
		if onMain[sha] {
			return sha, main.Branch
		}
	}
	return "", main.Branch
}

func mergeBaseWithPrevious(dir string, all []Line, line Line) (string, string) {
	for i := range all {
		if all[i].Branch != line.Branch {
			continue
		}
		if i == 0 {
			return "", ""
		}
		base, err := git(dir, "merge-base", all[i-1].Head, line.Head)
		if err != nil {
			return "", ""
		}
		return base, all[i-1].Branch
	}
	return "", ""
}

// Previous is where the release before line ends: the newest older line, or a newer tag of an older version
// (`2.4.0` or `v2.4.0`); a line wins over a tag of the same version. Without either it is BranchPoint.
func Previous(dir string, all []Line, line Line) (ref, label string) {
	bestVersion := ""
	for _, other := range all {
		if other.Kind != profile.MainLine && version.Compare(other.Version, line.Version) < 0 {
			ref, label, bestVersion = other.Head, other.Branch, other.Version
		}
	}
	if tags, err := git(dir, "tag", "--list"); err == nil {
		for _, tag := range strings.Split(tags, "\n") {
			v := strings.TrimPrefix(tag, "v")
			if tag == "" || !version.IsSemantic(v) || version.Compare(v, line.Version) >= 0 {
				continue
			}
			if bestVersion != "" && version.Compare(v, bestVersion) <= 0 {
				continue
			}
			if sha, err := git(dir, "rev-parse", "--verify", "-q", "refs/tags/"+tag+"^{commit}"); err == nil {
				ref, label, bestVersion = sha, "tag "+tag, v
			}
		}
	}
	if bestVersion == "" {
		return BranchPoint(dir, all, line)
	}
	return ref, label
}
