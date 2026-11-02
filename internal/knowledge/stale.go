package knowledge

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// ProposalMaxDays is how long a proposal may wait in inbox/ before it counts as old.
	ProposalMaxDays = 14
	// VerifiedMaxDays is how old a page's newest verified marker may be before the page counts as unverified.
	VerifiedMaxDays = 180
)

// StaleReport is what Staleness found. It never edits anything.
type StaleReport struct {
	// Missing are code pointers whose path is not in their repo at HEAD, by page path and pointer line.
	Missing []StalePointer
	// Checked counts the pointers whose repo was resolved.
	Checked int
	// Unresolved are the repo names of pointers that could not be checked, sorted.
	Unresolved []string
	// OldProposals waited more than ProposalMaxDays, oldest first.
	OldProposals []OldProposal
	// Unverified pages have no verified marker newer than VerifiedMaxDays, by path.
	Unverified []UnverifiedPage
}

// StalePointer is a pointer on a page whose path is gone; RepoDir is where its repo was found.
type StalePointer struct {
	Page    string
	Pointer Pointer
	RepoDir string
}

// OldProposal is a proposal and the days it has waited.
type OldProposal struct {
	Path string
	Days int
}

// UnverifiedPage is a page and its newest verified date with its age in days; Newest is empty without any marker.
type UnverifiedPage struct {
	Path   string
	Newest string
	Days   int
}

var datedName = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})-`)

// Staleness checks the clone's pages and inbox against now. A pointer's repo resolves to the project when the
// name matches the project's folder or its origin remote, else to a sibling folder of the project with that name;
// its path is looked up at HEAD, or on disk when that folder is not a git repo with commits.
func Staleness(clone, project string, now time.Time) (StaleReport, error) {
	var report StaleReport
	today := day(now)
	pages, err := Pages(clone)
	if err != nil {
		return report, err
	}
	repos := newRepoResolver(project)
	unresolved := map[string]bool{}
	for _, page := range pages {
		for _, pointer := range page.Page.Pointers {
			dir, ok := repos.resolve(pointer.Repo)
			if !ok {
				unresolved[pointer.Repo] = true
				continue
			}
			report.Checked++
			if !repos.has(dir, pointer.Path) {
				report.Missing = append(report.Missing, StalePointer{Page: page.Path, Pointer: pointer, RepoDir: dir})
			}
		}
		newest := NewestVerified(page.Page)
		if newest == "" {
			report.Unverified = append(report.Unverified, UnverifiedPage{Path: page.Path})
			continue
		}
		if date, err := time.Parse("2006-01-02", newest); err == nil {
			if days := daysBetween(date, today); days > VerifiedMaxDays {
				report.Unverified = append(report.Unverified, UnverifiedPage{Path: page.Path, Newest: newest, Days: days})
			}
		}
	}
	for repo := range unresolved {
		report.Unresolved = append(report.Unresolved, repo)
	}
	sort.Strings(report.Unresolved)

	proposals, err := Inbox(clone)
	if err != nil && !os.IsNotExist(err) {
		return report, err
	}
	for _, proposal := range proposals {
		written, ok := proposalDate(filepath.Join(clone, filepath.FromSlash(proposal)))
		if !ok {
			continue
		}
		if days := daysBetween(written, today); days > ProposalMaxDays {
			report.OldProposals = append(report.OldProposals, OldProposal{Path: proposal, Days: days})
		}
	}
	return report, nil
}

// proposalDate is the date a proposal's name starts with, else the day its file was last changed.
func proposalDate(file string) (time.Time, bool) {
	if match := datedName.FindStringSubmatch(filepath.Base(file)); match != nil {
		if date, err := time.Parse("2006-01-02", match[1]); err == nil {
			return date, true
		}
	}
	info, err := os.Stat(file)
	if err != nil {
		return time.Time{}, false
	}
	return day(info.ModTime()), true
}

func day(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysBetween(from, to time.Time) int { return int(to.Sub(from).Hours() / 24) }

// repoResolver finds the folder of a repo named in a pointer and caches the files at its HEAD.
type repoResolver struct {
	project string
	// root and remote are the project's repo root and the name its origin remote gives, read once.
	root, remote string
	read         bool
	dirs         map[string]string
	files        map[string]map[string]bool
}

func newRepoResolver(project string) *repoResolver {
	return &repoResolver{project: project, dirs: map[string]string{}, files: map[string]map[string]bool{}}
}

func (r *repoResolver) resolve(name string) (string, bool) {
	if dir, ok := r.dirs[name]; ok {
		return dir, dir != ""
	}
	if !r.read {
		r.read = true
		r.root = r.project
		if top, err := runGit(r.project, "rev-parse", "--show-toplevel"); err == nil && top != "" {
			r.root = filepath.Clean(top)
		}
		if url, err := runGit(r.root, "remote", "get-url", "origin"); err == nil {
			r.remote = repoName(url)
		}
	}
	dir := ""
	switch sibling := filepath.Join(filepath.Dir(r.root), name); {
	case name == filepath.Base(r.root) || name == r.remote:
		dir = r.root
	case isDir(sibling):
		dir = sibling
	}
	r.dirs[name] = dir
	return dir, dir != ""
}

// has reports whether the slash-separated path is a file at the repo's HEAD, or on disk outside git.
func (r *repoResolver) has(dir, file string) bool {
	file = strings.TrimPrefix(path.Clean("/"+file), "/")
	files, ok := r.files[dir]
	if !ok {
		files = nil
		if out, err := runGit(dir, "ls-tree", "-r", "-z", "--name-only", "HEAD"); err == nil {
			files = map[string]bool{}
			for _, name := range strings.Split(out, "\x00") {
				files[name] = true
			}
		}
		r.files[dir] = files
	}
	if files == nil {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(file)))
		return err == nil && !info.IsDir()
	}
	return files[file]
}

// repoName is the last segment of a remote URL, scp-like address or path, without a trailing `.git`.
func repoName(url string) string {
	url = strings.TrimRight(strings.TrimSpace(url), "/")
	if i := strings.LastIndexAny(url, "/:"); i >= 0 {
		url = url[i+1:]
	}
	return strings.TrimSuffix(url, ".git")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
