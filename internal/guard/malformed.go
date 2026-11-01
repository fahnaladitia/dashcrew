package guard

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// The malformed log (G4b, Go only): the guard lets a malformed payload through and appends one line about it,
// so `doctor` can tell that the guard is reading less than the agent sends.

const (
	// MalformedLogName is the log under `<git common dir>/dashcrew/`; LastSyncName holds the time of the last sync.
	MalformedLogName = "guard-malformed.log"
	LastSyncName     = "last-sync"
	// MalformedLogLimit is the log size past which lines are dropped.
	MalformedLogLimit = 64 * 1024
)

// GitCommonDir is the git common dir of the repository holding dir, shared by its worktrees; false outside one.
// It reads `.git` the way git does, without running git: a folder, or a file naming the worktree's git dir,
// whose `commondir` file names the common one.
func GitCommonDir(dir string) (string, bool) {
	for dir != "" {
		dotGit := filepath.Join(dir, ".git")
		if info, err := os.Stat(dotGit); err == nil {
			if info.IsDir() {
				return dotGit, true
			}
			data, err := os.ReadFile(dotGit)
			if err != nil {
				return "", false
			}
			gitDir, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
			if !ok {
				return "", false
			}
			gitDir = resolve(dir, strings.TrimSpace(gitDir))
			if common, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
				return resolve(gitDir, strings.TrimSpace(string(common))), true
			}
			return gitDir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func resolve(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

// MalformedLogPath is the malformed log of the repository holding dir; false outside one.
func MalformedLogPath(dir string) (string, bool) {
	common, ok := GitCommonDir(dir)
	if !ok {
		return "", false
	}
	return filepath.Join(common, "dashcrew", MalformedLogName), true
}

// MarkSynced records now as the last sync of the repository holding projectDir; doctor counts log lines after it.
// Nothing happens outside a repository.
func MarkSynced(projectDir string, now time.Time) error {
	common, ok := GitCommonDir(projectDir)
	if !ok {
		return nil
	}
	dir := filepath.Join(common, "dashcrew")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, LastSyncName), []byte(now.UTC().Format(time.RFC3339Nano)+"\n"), 0o644)
}

// MalformedSinceSync counts the log lines of the repository holding projectDir written after the last sync, or all
// of them when no sync was recorded. path is the log; zero when there is none.
func MalformedSinceSync(projectDir string) (count int, path string) {
	path, ok := MalformedLogPath(projectDir)
	if !ok {
		return 0, ""
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, path
	}
	defer file.Close()
	var since time.Time
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(path), LastSyncName)); err == nil {
		since, _ = time.Parse(time.RFC3339Nano, strings.TrimSpace(string(data)))
	}
	lines := bufio.NewScanner(file)
	for lines.Scan() {
		stamp, _, _ := strings.Cut(lines.Text(), "\t")
		if at, err := time.Parse(time.RFC3339Nano, stamp); err == nil && at.After(since) {
			count++
		}
	}
	return count, path
}

// codexTools are the tool names only Codex sends.
var codexTools = map[string]bool{"shell": true, "local_shell": true, "exec_command": true, "container.exec": true, "apply_patch": true}

// agentOf names the agent that sent input, from the payload's shape only: unknown when input is nil.
func agentOf(input *omap.Map[any]) string {
	switch {
	case input == nil:
		return "unknown"
	case input.Has("toolCall"):
		return "antigravity"
	}
	name, ok := input.Value("tool_name").(string)
	switch {
	case !ok:
		return "unknown"
	case codexTools[name]:
		return "codex"
	}
	return "claude-code"
}

// logMalformed appends one line, `<time>\t<agent>\t<reason>`, to the malformed log of the repository the hook ran
// in: the hook's `cwd`, else its first workspace path, else the process's. reason must never quote the payload.
// Every failure is ignored: the log must not stop a tool call.
func logMalformed(env Env, input *omap.Map[any], reason string) {
	dir := ""
	if input != nil {
		if cwd, ok := input.Value("cwd").(string); ok {
			dir = cwd
		} else if paths, ok := input.Value("workspacePaths").([]any); ok && len(paths) > 0 {
			dir, _ = paths[0].(string)
		}
	}
	if dir == "" {
		cwd, err := env.Getwd()
		if err != nil {
			return
		}
		dir = cwd
	}
	path, ok := MalformedLogPath(dir)
	if !ok {
		return
	}
	now := time.Now
	if env.Now != nil {
		now = env.Now
	}
	line := now().UTC().Format(time.RFC3339Nano) + "\t" + agentOf(input) + "\t" + reason + "\n"
	if info, err := os.Stat(path); err == nil && info.Size()+int64(len(line)) > MalformedLogLimit {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	file.WriteString(line)
	file.Close()
}
