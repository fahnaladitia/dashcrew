package knowledge

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LinkPath is where sync links the clone inside a project.
const LinkPath = ".ai/knowledge"

// pullStamp is the file under the clone's .git folder that records the last pull, as RFC 3339.
const pullStamp = "dashcrew-last-pull"

// CloneDir is the local clone of the knowledge repo name for the user whose home folder is home.
func CloneDir(home, name string) string { return filepath.Join(home, ".dashcrew", "knowledge", name) }

// PolicyClonePath is CloneDir as `.ai/guard.json` records it: under `~`, so the policy is the same on every machine.
func PolicyClonePath(name string) string { return "~/.dashcrew/knowledge/" + name }

// ServerName is the MCP server `dashcrew knowledge serve` runs as, and the key sync registers it under.
const ServerName = "dashcrew-knowledge"

// ServeArgs are the arguments of the `dashcrew` command sync registers as the knowledge MCP server: the clone is
// given under `~`, which serve expands, so the registration is the same on every machine.
func ServeArgs(name string) []string {
	return []string{"knowledge", "serve", "--clone", PolicyClonePath(name)}
}

// ExpandHome replaces a leading `~/` with home.
func ExpandHome(path, home string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		return filepath.Join(home, filepath.FromSlash(rest))
	}
	return filepath.FromSlash(path)
}

// ResolveRepo is the repo git clones: a URL or scp-like address as is, a relative path joined to projectDir.
func ResolveRepo(repo, projectDir string) string {
	if strings.Contains(repo, "://") || filepath.IsAbs(repo) {
		return repo
	}
	if before, _, ok := strings.Cut(repo, ":"); ok && !strings.ContainsAny(before, `/\`) && len(before) > 1 {
		return repo
	}
	return filepath.Join(projectDir, filepath.FromSlash(repo))
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// DivergedError is a clone with commits its remote lacks and the other way round; Pull never resets it.
type DivergedError struct {
	Clone         string
	Local, Remote int
}

func (e *DivergedError) Error() string {
	return fmt.Sprintf("%s has diverged from its remote (%d local, %s). dashcrew never resets it; merge or rebase it by hand.",
		e.Clone, e.Local, plural(e.Remote, "remote commit"))
}

// runGit runs git in dir and returns its trimmed stdout; a failure carries git's stderr.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// isAncestor is `git merge-base --is-ancestor a b`.
func isAncestor(dir, a, b string) (bool, error) {
	cmd := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", a, b)
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
}

func count(dir, revisions string) (int, error) {
	out, err := runGit(dir, "rev-list", "--count", revisions)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// Pull clones repo into clone when clone is missing, else fetches and fast-forwards it. A clone that is ahead is
// left alone; a diverged one is a *DivergedError and is never reset. now is recorded as the last pull.
func Pull(repo, clone string, now time.Time) (string, error) {
	if _, err := os.Stat(clone); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(clone), 0o755); err != nil {
			return "", err
		}
		if _, err := runGit(filepath.Dir(clone), "clone", "-q", repo, clone); err != nil {
			return "", err
		}
		head, _ := HeadSHA(clone)
		return "cloned " + repo + " into " + clone + " at " + short(head), stampPull(clone, now)
	}
	if err := Fetch(clone); err != nil {
		return "", err
	}
	if err := stampPull(clone, now); err != nil {
		return "", err
	}
	head, err := runGit(clone, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	upstream, err := runGit(clone, "rev-parse", "@{u}")
	if err != nil {
		return "", fmt.Errorf("%s has no upstream branch; check out a branch that tracks its remote", clone)
	}
	if head == upstream {
		return clone + " is up to date at " + short(head), nil
	}
	if behind, err := isAncestor(clone, head, upstream); err != nil {
		return "", err
	} else if behind {
		commits, err := count(clone, head+".."+upstream)
		if err != nil {
			return "", err
		}
		if _, err := runGit(clone, "merge", "-q", "--ff-only", upstream); err != nil {
			return "", err
		}
		return fmt.Sprintf("fast-forwarded %s %s..%s (%s)", clone, short(head), short(upstream), plural(commits, "commit")), nil
	}
	local, err := count(clone, upstream+".."+head)
	if err != nil {
		return "", err
	}
	if ahead, err := isAncestor(clone, upstream, head); err != nil {
		return "", err
	} else if ahead {
		return clone + " is " + plural(local, "commit") + " ahead of its remote; nothing to pull", nil
	}
	remote, err := count(clone, head+".."+upstream)
	if err != nil {
		return "", err
	}
	return "", &DivergedError{Clone: clone, Local: local, Remote: remote}
}

// Fetch is `git fetch` in an existing clone; a folder that is not a clone is an error.
func Fetch(clone string) error {
	if _, err := gitDir(clone); err != nil {
		return err
	}
	_, err := runGit(clone, "fetch", "-q")
	return err
}

// Behind is how many commits the clone's upstream has that HEAD lacks, as of the last fetch.
func Behind(clone string) (int, error) { return count(clone, "HEAD..@{u}") }

func stampPull(clone string, now time.Time) error {
	dir, err := gitDir(clone)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, pullStamp), []byte(now.UTC().Format(time.RFC3339)+"\n"), 0o644)
}

// LastPull is the time of the last Pull, else the time of the last fetch; false when neither is known.
func LastPull(clone string) (time.Time, bool) {
	dir, err := gitDir(clone)
	if err != nil {
		return time.Time{}, false
	}
	if data, err := os.ReadFile(filepath.Join(dir, pullStamp)); err == nil {
		if stamp, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data))); err == nil {
			return stamp, true
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "FETCH_HEAD")); err == nil {
		return info.ModTime(), true
	}
	return time.Time{}, false
}

// gitDir is the clone's git folder: `.git`, or the folder a `.git` file points to.
func gitDir(clone string) (string, error) {
	dot := filepath.Join(clone, ".git")
	info, err := os.Stat(dot)
	if err != nil {
		return "", fmt.Errorf("%s is not a git clone; move it away and run dashcrew knowledge pull", clone)
	}
	if info.IsDir() {
		return dot, nil
	}
	data, err := os.ReadFile(dot)
	if err != nil {
		return "", err
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !ok {
		return "", fmt.Errorf("%s is not a git clone; move it away and run dashcrew knowledge pull", clone)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(clone, target)
	}
	return target, nil
}

// HeadSHA is the commit the clone has checked out, read from its git files without running git (sync stays
// offline and fast); empty for a repo without commits.
func HeadSHA(clone string) (string, error) {
	dir, err := gitDir(clone)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, "HEAD"))
	if err != nil {
		return "", err
	}
	head := strings.TrimSpace(string(data))
	ref, symbolic := strings.CutPrefix(head, "ref: ")
	if !symbolic {
		return head, nil
	}
	if data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(ref))); err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	packed, err := os.Open(filepath.Join(dir, "packed-refs"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer packed.Close()
	scanner := bufio.NewScanner(packed)
	for scanner.Scan() {
		if sha, name, ok := strings.Cut(scanner.Text(), " "); ok && name == ref {
			return sha, nil
		}
	}
	return "", scanner.Err()
}
