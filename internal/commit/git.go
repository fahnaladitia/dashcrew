package commit

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/profile"
)

// Repo is the work tree a command runs in, at its top level.
type Repo struct {
	Dir string
}

// Open finds the work tree that holds dir.
func Open(dir string) (Repo, error) {
	top, err := runGit(dir, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repo{}, fmt.Errorf("%s is not inside a git repository", dir)
	}
	return Repo{Dir: filepath.Clean(top)}, nil
}

// runGit runs git in dir with extra environment entries and returns stdout without its trailing newline.
func runGit(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", &gitError{args: args, detail: detail}
	}
	return strings.TrimSuffix(stdout.String(), "\n"), nil
}

// gitError is a failed git call with git's own message.
type gitError struct {
	args   []string
	detail string
}

func (e *gitError) Error() string { return "git " + strings.Join(e.args, " ") + ": " + e.detail }

func (r Repo) git(args ...string) (string, error) { return runGit(r.Dir, nil, args...) }

// Branch is the checked-out branch; empty when HEAD is detached.
func (r Repo) Branch() string {
	branch, _ := r.git("symbolic-ref", "--quiet", "--short", "HEAD")
	return branch
}

// Changes are the tracked files with staged or unstaged changes, a rename by its new path, and the untracked
// files apart; both sorted, relative to the top level.
func (r Repo) Changes() (changed, untracked []string, err error) {
	out, err := r.git("status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return nil, nil, err
	}
	entries := strings.Split(out, "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		status, path := entry[:2], entry[3:]
		switch {
		case status == "??":
			untracked = append(untracked, path)
		default:
			changed = append(changed, path)
			if status[0] == 'R' || status[0] == 'C' {
				i++
			}
		}
	}
	sort.Strings(changed)
	sort.Strings(untracked)
	return changed, untracked, nil
}

// WorkTree is the tree hash of the working copy, untracked files included and ignored ones left out. It is built
// in a temporary index (GIT_INDEX_FILE) seeded from a copy of the real one, so the user's index and staging are never
// touched; only blob and tree objects are written.
func (r Repo) WorkTree() (string, error) {
	scratch, err := os.MkdirTemp("", "dashcrew-index-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(scratch)
	index := filepath.Join(scratch, "index")
	if real, err := r.git("rev-parse", "--path-format=absolute", "--git-path", "index"); err == nil {
		if data, err := os.ReadFile(real); err == nil {
			if err := os.WriteFile(index, data, 0o600); err != nil {
				return "", err
			}
		}
	}
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := runGit(r.Dir, env, "add", "--all", "--", "."); err != nil {
		// A copy that git cannot read on its own (a split index, say) is rebuilt from HEAD instead.
		os.Remove(index)
		_, _ = runGit(r.Dir, env, "read-tree", "HEAD")
		if _, err := runGit(r.Dir, env, "add", "--all", "--", "."); err != nil {
			return "", err
		}
	}
	return runGit(r.Dir, env, "write-tree")
}

// VerifyRecord is where `dashcrew verify record` keeps the tree it checked: per worktree, under its git dir.
func (r Repo) VerifyRecord() (string, error) {
	return r.git("rev-parse", "--path-format=absolute", "--git-path", "dashcrew/last-verify")
}

// RecordVerify writes the tree hash of the working copy as the last verified tree.
func (r Repo) RecordVerify() (string, error) {
	tree, err := r.WorkTree()
	if err != nil {
		return "", err
	}
	path, err := r.VerifyRecord()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return tree, os.WriteFile(path, []byte(tree+"\n"), 0o644)
}

// SinceVerify lists the files that differ from the last verified tree. known is false without a record, or when
// the recorded tree is no longer in the repository.
func (r Repo) SinceVerify() (files []string, known bool, err error) {
	path, err := r.VerifyRecord()
	if err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	} else if err != nil {
		return nil, false, err
	}
	recorded := strings.TrimSpace(string(data))
	if _, err := r.git("cat-file", "-e", recorded+"^{tree}"); err != nil {
		return nil, false, nil
	}
	current, err := r.WorkTree()
	if err != nil {
		return nil, false, err
	}
	out, err := r.git("diff-tree", "-r", "--name-only", "-z", recorded, current)
	if err != nil {
		return nil, false, err
	}
	files = []string{}
	for _, file := range strings.Split(out, "\x00") {
		if file != "" {
			files = append(files, file)
		}
	}
	return files, true, nil
}

// Logged is one commit of a range.
type Logged struct {
	Hash, Message string
}

// Messages are the commits of an `a..b` range, oldest first, without merge commits.
func (r Repo) Messages(revRange string) ([]Logged, error) {
	if strings.HasPrefix(revRange, "-") || !strings.Contains(revRange, "..") {
		return nil, fmt.Errorf("range %q: expected <base>..<head>", revRange)
	}
	out, err := r.git("log", "--no-merges", "--reverse", "-z", "--format=%H%n%B", revRange, "--")
	if err != nil {
		var failed *gitError
		if errors.As(err, &failed) {
			return nil, fmt.Errorf("cannot read %s: %s", revRange, failed.detail)
		}
		return nil, err
	}
	var commits []Logged
	for _, entry := range strings.Split(out, "\x00") {
		hash, message, ok := strings.Cut(entry, "\n")
		if !ok || hash == "" {
			continue
		}
		commits = append(commits, Logged{Hash: hash, Message: message})
	}
	return commits, nil
}

// LintRange lints every commit of an `a..b` range but merges; each problem is prefixed with the short hash.
func LintRange(repo Repo, cfg *profile.Commit, revRange string) (problems []string, commits int, err error) {
	logged, err := repo.Messages(revRange)
	if err != nil {
		return nil, 0, err
	}
	for _, c := range logged {
		for _, problem := range Lint(cfg, c.Message) {
			problems = append(problems, c.Hash[:min(7, len(c.Hash))]+" "+problem)
		}
	}
	return problems, len(logged), nil
}

// Follow is the summary of a range that passed: how many commits follow the pattern.
func Follow(commits int, revRange, pattern string) string {
	if commits == 1 {
		return "1 commit in " + revRange + " follows " + pattern
	}
	return fmt.Sprintf("%d commits in %s follow %s", commits, revRange, pattern)
}
