package spec

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Ref is the spec a task builds, as step 0 `Spec:` and `branch.<name>.dashcrew-spec` write it:
// `specs/<release>/<feature>@rev<n>`.
type Ref struct {
	// Folder is slash-separated from `specs/` on: `specs/<release>/<feature>`.
	Folder string
	Rev    int
}

func (r Ref) String() string { return r.Folder + "@rev" + strconv.Itoa(r.Rev) }

var refPattern = regexp.MustCompile(`^specs/([A-Za-z0-9][A-Za-z0-9._-]*)/([a-z0-9][a-z0-9-]*)@rev([1-9][0-9]*)$`)

// ParseRef reads a recorded spec; false for `none`, an empty value or anything else that is not a spec reference.
func ParseRef(value string) (Ref, bool) {
	m := refPattern.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil || strings.Contains(m[1], "..") {
		return Ref{}, false
	}
	rev, err := strconv.Atoi(m[3])
	if err != nil {
		return Ref{}, false
	}
	return Ref{Folder: Dir + "/" + m[1] + "/" + m[2], Rev: rev}, true
}

// State is a spec as the knowledge clone holds it now.
type State struct {
	// Rev is the highest rev of its files; Status is the earliest status in life order among them, so a spec half
	// moved back to review counts as review.
	Rev    int
	Status string
	// ACs are the live acceptance criteria of prd.md, in file order; Repos are the repos its front matter names.
	ACs   []string
	Repos []string
}

// Load reads the spec at folder (from `specs/` on) in clone; an error when neither file exists.
func Load(clone, folder string) (State, error) {
	var state State
	found := false
	for _, name := range Files {
		data, err := os.ReadFile(filepath.Join(clone, filepath.FromSlash(folder), name))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return State{}, err
		}
		doc := Parse(string(data))
		if !found || lifeOrder(doc.Status) < lifeOrder(state.Status) {
			state.Status = doc.Status
		}
		found = true
		state.Rev = max(state.Rev, doc.Rev)
		if name == "prd.md" {
			state.Repos = doc.Repos
			for _, id := range doc.IDs {
				if id.Kind == "AC" && !id.Dropped {
					state.ACs = append(state.ACs, id.String())
				}
			}
		}
	}
	if !found {
		return State{}, fmt.Errorf("no spec at %s in the knowledge clone", folder)
	}
	return state, nil
}

// lifeOrder is the position of status in Statuses; an unknown status comes first.
func lifeOrder(status string) int {
	for i, s := range Statuses {
		if s == status {
			return i
		}
	}
	return -1
}

var acReference = regexp.MustCompile(`\bAC-([1-9][0-9]*)\b`)

// Claims maps each AC ID to the plan files (slash paths from project) under plansDir that name folder and that ID.
// A plan names a spec when it holds its folder followed by `@`, `/`, whitespace, punctuation or the end of a line.
func Claims(project, plansDir, folder string) (map[string][]string, error) {
	claims := map[string][]string{}
	root := filepath.Join(project, filepath.FromSlash(plansDir))
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return claims, nil
	}
	names := regexp.MustCompile(regexp.QuoteMeta(folder) + `(?:[@/\s.,;:)]|$)`)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".md" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !names.Match(data) {
			return nil
		}
		relative, _ := filepath.Rel(project, path)
		seen := map[string]bool{}
		for _, m := range acReference.FindAllSubmatch(data, -1) {
			id := "AC-" + string(m[1])
			if !seen[id] {
				seen[id] = true
				claims[id] = append(claims[id], filepath.ToSlash(relative))
			}
		}
		return nil
	})
	for _, files := range claims {
		sort.Strings(files)
	}
	return claims, err
}

// Drift is what a branch that records recorded should hear about the spec's state: a newer revision, or a spec still
// in draft while the branch has own commits past its base. Each is a warning; none blocks.
func Drift(recorded Ref, state State, ownCommits int) []string {
	var out []string
	if state.Rev > recorded.Rev {
		out = append(out, "spec moved to rev "+strconv.Itoa(state.Rev)+"; re-read it")
	}
	if state.Status == Draft && ownCommits > 0 {
		commits := strconv.Itoa(ownCommits) + " commits"
		if ownCommits == 1 {
			commits = "1 commit"
		}
		out = append(out, "spec is still draft while the branch has "+commits+" past its base; get it reviewed")
	}
	return out
}

// RepoNames are the names other files may use for the repository at dir: its top folder's name and its origin
// remote's last path segment without `.git`. Empty outside a repository.
func RepoNames(dir string) []string {
	top, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil
	}
	names := []string{filepath.Base(strings.TrimSpace(top))}
	if url, err := git(dir, "remote", "get-url", "origin"); err == nil {
		url = strings.TrimRight(strings.TrimSpace(url), "/")
		if i := strings.LastIndexAny(url, "/:"); i >= 0 {
			url = url[i+1:]
		}
		if name := strings.TrimSuffix(url, ".git"); name != "" && name != names[0] {
			names = append(names, name)
		}
	}
	return names
}
