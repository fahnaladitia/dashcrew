package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// PrLookup is the PR state of a branch as one line (`#12 OPEN`, `#12 OPEN CONFLICTING`, `#12 MERGED`), or nil.
type PrLookup func(branch string) *string

// DoneEvent starts the last line a watch prints.
const DoneEvent = "done"

// Watch emits one line per new event in the watched worktrees; state in StateDir keeps a rerun from repeating
// events. Events: `commit`, `pushed`, `pr`, `base-moved`, `merge-tree`, `overlap`, `error`, and a final `done`.
type Watch struct {
	Worktrees *Worktrees
	StateDir  string
	// PrLookup defaults to GhPrLookup of the main checkout.
	PrLookup PrLookup
}

var spaces = regexp.MustCompile(`\s+`)

func (w *Watch) git(args ...string) result { return git(w.Worktrees.MainDir(), args...) }

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func mapOf(value any) *omap.Map[any] {
	if m, ok := value.(*omap.Map[any]); ok {
		return m
	}
	return omap.New[any]()
}

func stringOrNil(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

// Pass runs once over the worktrees named in names (folder names), or every worktree with a recorded base.
// A name that is neither a worktree nor watched before is an Error.
func (w *Watch) Pass(names []string) ([]string, error) {
	lookup := w.PrLookup
	if lookup == nil {
		lookup = GhPrLookup(w.Worktrees.MainDir())
	}
	state, err := w.readState()
	if err != nil {
		return nil, err
	}
	seen := []string{}
	if list, ok := state.Value("watched").([]any); ok {
		for _, item := range list {
			if name, ok := item.(string); ok {
				seen = append(seen, name)
			}
		}
	}
	infos, err := w.Worktrees.List()
	if err != nil {
		return nil, err
	}
	all := []Info{}
	for _, info := range infos {
		if info.Base != nil {
			all = append(all, info)
		}
	}
	for _, name := range names {
		known := contains(seen, name)
		for _, info := range all {
			known = known || info.Name == name
		}
		if !known {
			return nil, &Error{"No worktree named " + name + "."}
		}
	}
	current := all
	if len(names) > 0 {
		current = []Info{}
		for _, info := range all {
			if contains(names, info.Name) {
				current = append(current, info)
			}
		}
	}
	watched := append([]string{}, seen...)
	for _, info := range current {
		if !contains(watched, info.Name) {
			watched = append(watched, info.Name)
		}
	}

	events := []string{}
	branches := mapOf(state.Value("branches")).Copy()
	bases := mapOf(state.Value("bases")).Copy()

	baseNames := []string{}
	for _, info := range current {
		if !contains(baseNames, *info.Base) {
			baseNames = append(baseNames, *info.Base)
		}
	}
	sort.Strings(baseNames)
	for _, base := range baseNames {
		fetch := w.git("fetch", "--quiet", "origin", base)
		if fetch.code != 0 {
			events = append(events, "error origin/"+base+": fetch failed: "+firstLine(strings.TrimSpace(fetch.stderr)))
			continue
		}
		sha := strings.TrimSpace(w.git("rev-parse", "refs/remotes/origin/"+base).stdout)
		previous, _ := bases.Value(base).(string)
		_, hadPrevious := bases.Value(base).(string)
		bases.Set(base, sha)
		if !hadPrevious || previous == sha {
			continue
		}
		events = append(events, "base-moved origin/"+base+": "+short(previous)+".."+short(sha))
		for _, info := range current {
			if *info.Base == base {
				events = append(events, "merge-tree "+info.Branch+" vs origin/"+base+": "+w.mergeTree(base, info.Branch))
			}
		}
	}

	for _, info := range current {
		previous := mapOf(branches.Value(info.Branch))
		next := previous.Copy()
		ahead, err := strconv.Atoi(strings.TrimSpace(w.git("rev-list", "--count", "origin/"+*info.Base+".."+info.Branch).stdout))
		if err != nil {
			ahead = 0
		}
		previousAhead, _ := previous.Value("ahead").(int)
		if ahead > 0 && previousAhead == 0 {
			events = append(events, "commit "+info.Branch+": first commit, "+strconv.Itoa(ahead)+" ahead of origin/"+*info.Base)
		}
		next.Set("ahead", ahead)

		remote := strings.TrimSpace(w.git("ls-remote", "origin", "refs/heads/"+info.Branch).stdout)
		var pushed *string
		if remote != "" {
			first := spaces.Split(remote, -1)[0]
			pushed = &first
		}
		if pushed != nil && !yamlutil.Equal(*pushed, previous.Value("pushed")) {
			events = append(events, "pushed "+info.Branch+": "+short(*pushed))
		}
		next.Set("pushed", stringOrNil(pushed))

		pr := lookup(info.Branch)
		if pr != nil && !yamlutil.Equal(*pr, previous.Value("pr")) {
			events = append(events, "pr "+info.Branch+": "+*pr)
		}
		next.Set("pr", stringOrNil(pr))
		branches.Set(info.Branch, next)
	}

	overlaps := mapOf(state.Value("overlaps")).Copy()
	changed := map[string]map[string]bool{}
	for _, info := range current {
		changed[info.Branch] = w.changedFiles(info)
	}
	branchNames := make([]string, 0, len(changed))
	for branch := range changed {
		branchNames = append(branchNames, branch)
	}
	sort.Strings(branchNames)
	for i := range branchNames {
		for j := i + 1; j < len(branchNames); j++ {
			key := branchNames[i] + " + " + branchNames[j]
			files := []string{}
			for file := range changed[branchNames[i]] {
				if changed[branchNames[j]][file] {
					files = append(files, file)
				}
			}
			sort.Strings(files)
			before := map[string]bool{}
			if list, ok := overlaps.Value(key).([]any); ok {
				for _, item := range list {
					if file, ok := item.(string); ok {
						before[file] = true
					}
				}
			}
			grew := false
			for _, file := range files {
				grew = grew || !before[file]
			}
			if grew {
				events = append(events, "overlap "+key+": "+strings.Join(files, ", "))
			}
			list := make([]any, len(files))
			for k, file := range files {
				list[k] = file
			}
			overlaps.Set(key, list)
		}
	}

	branchByName := map[string]string{}
	for _, info := range current {
		branchByName[info.Name] = info.Branch
	}
	merged := 0
	for _, name := range watched {
		branch, ok := branchByName[name]
		if !ok {
			merged++
			continue
		}
		if pr, _ := mapOf(branches.Value(branch)).Value("pr").(string); strings.Contains(pr, "MERGED") {
			merged++
		}
	}
	if len(watched) > 0 && merged == len(watched) {
		events = append(events, DoneEvent+": "+strconv.Itoa(len(watched))+" worktrees merged or removed")
	}

	sortedWatched := append([]string{}, watched...)
	sort.Strings(sortedWatched)
	watchedList := make([]any, len(sortedWatched))
	for i, name := range sortedWatched {
		watchedList[i] = name
	}
	if err := w.writeState(omap.Of[any]("watched", watchedList, "bases", bases, "branches", branches, "overlaps", overlaps)); err != nil {
		return nil, err
	}
	return events, nil
}

// Run passes every interval until `done`; once stops after the first pass.
func (w *Watch) Run(names []string, once bool, interval time.Duration, out func(string)) error {
	for {
		events, err := w.Pass(names)
		if err != nil {
			return err
		}
		done := false
		for _, event := range events {
			out(event)
			done = done || strings.HasPrefix(event, DoneEvent)
		}
		if once || done {
			return nil
		}
		time.Sleep(interval)
	}
}

// GhPrLookup asks `gh pr list` for the branch; nil when gh is missing, fails, or finds no PR.
func GhPrLookup(mainDir string) PrLookup {
	return func(branch string) *string {
		if _, err := exec.LookPath("gh"); err != nil {
			return nil
		}
		r := run(mainDir, "gh", "pr", "list", "--head", branch, "--state", "all", "--limit", "1", "--json", "number,state,mergeable")
		if r.code != 0 {
			return nil
		}
		decoded, err := dartjson.Decode(r.stdout)
		list, ok := decoded.([]any)
		if err != nil || !ok || len(list) == 0 {
			return nil
		}
		pr := mapOf(list[0])
		line := "#" + yamlutil.DartString(pr.Value("number")) + " " + yamlutil.DartString(pr.Value("state"))
		if pr.Value("mergeable") == "CONFLICTING" {
			line += " CONFLICTING"
		}
		return &line
	}
}

func (w *Watch) changedFiles(info Info) map[string]bool {
	files := map[string]bool{}
	committed := strings.TrimSpace(w.git("diff", "--name-only", "origin/"+*info.Base+"..."+info.Branch).stdout)
	if committed != "" {
		for _, file := range strings.Split(committed, "\n") {
			files[file] = true
		}
	}
	for _, line := range strings.Split(git(info.Path, "status", "--porcelain").stdout, "\n") {
		if len(line) > 3 {
			files[line[3:]] = true
		}
	}
	return files
}

func (w *Watch) mergeTree(base, branch string) string {
	r := w.git("merge-tree", "--write-tree", "--name-only", "--no-messages", "origin/"+base, branch)
	if r.code == 0 {
		return "CLEAN"
	}
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if r.code == 1 && len(lines) > 1 {
		return "CONFLICT " + strings.Join(lines[1:], ", ")
	}
	return "ERROR " + firstLine(strings.TrimSpace(r.stderr))
}

func (w *Watch) stateFile() string { return filepath.Join(w.StateDir, "state.json") }

func (w *Watch) readState() (*omap.Map[any], error) {
	data, err := os.ReadFile(w.stateFile())
	if err != nil {
		return omap.New[any](), nil
	}
	decoded, err := dartjson.Decode(string(data))
	if err != nil {
		return nil, err
	}
	return mapOf(decoded), nil
}

func (w *Watch) writeState(state *omap.Map[any]) error {
	text, err := dartjson.EncodeIndent(state, "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(w.StateDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(w.stateFile(), []byte(text), 0o644)
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
