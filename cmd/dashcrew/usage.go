package main

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/usage"
)

func usageCommand(cli cliIO) *command {
	c := newCommand("usage", "Tokens per session and role from the Claude Code and Codex transcripts of this project. Writes nothing.\n"+
		"Counts input, cache write, cache read and output per role (main, reviewer, explorer, other subagents), the context\n"+
		"loaded before the first answer (start), and the skills each role loaded. The project is the git checkout and its worktrees.")
	c.option("project", "p", "Any folder inside the repository.", ptr("."))
	c.option("since", "", "Sessions started on or after <yyyy-mm-dd>, or in the last <N>d.", ptr("7d"))
	c.option("session", "", "Only the sessions whose id starts with this.", nil)
	c.option("agent", "", "Only claude or codex.", nil)
	c.flag("json", "", "Print the sessions as JSON, numbers unformatted.", false, false)
	c.run = func(r *results) (int, error) {
		if len(r.rest) > 0 {
			return 0, &usageError{c, "usage takes no arguments."}
		}
		now := clock(cli.env)()
		since, ok := parseSince(*r.option("since"), now)
		if !ok {
			return 0, &usageError{c, `--since takes <yyyy-mm-dd> or <N>d, such as 30d.`}
		}
		q := usage.Query{Since: since}
		if v := r.option("agent"); v != nil {
			if *v != "claude" && *v != "codex" {
				return 0, &usageError{c, `--agent takes claude or codex.`}
			}
			q.Agent = *v
		}
		if v := r.option("session"); v != nil {
			q.Session = *v
		}
		dir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		q.Dirs = projectDirs(dir)
		sessions, err := usage.Load(usage.Places{ClaudeHome: claudeHome(cli.env), CodexHome: doctor.DefaultCodexHome(cli.env)}, q)
		if err != nil {
			return 0, err
		}
		if r.flag("json") {
			return 0, usage.JSON(cli.stdout, sessions)
		}
		if len(sessions) == 0 {
			cli.out("no Claude Code or Codex sessions for " + q.Dirs[0] + " since " + since.Format("2006-01-02"))
			return 0, nil
		}
		usage.Text(cli.stdout, sessions, time.Local)
		return 0, nil
	}
	return c
}

// parseSince reads `<yyyy-mm-dd>` (local midnight) or `<N>d` (N days before now).
func parseSince(value string, now time.Time) (time.Time, bool) {
	if days, ok := strings.CutSuffix(value, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil || n < 0 {
			return time.Time{}, false
		}
		return now.AddDate(0, 0, -n), true
	}
	t, err := time.ParseInLocation("2006-01-02", value, time.Local)
	return t, err == nil
}

// projectDirs is the git top level of dir and every worktree of it; dir alone outside git.
func projectDirs(dir string) []string {
	top, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		if real, err := filepath.EvalSymlinks(dir); err == nil {
			return []string{real}
		}
		return []string{filepath.Clean(dir)}
	}
	dirs := []string{filepath.Clean(strings.TrimSpace(string(top)))}
	seen := map[string]bool{dirs[0]: true}
	out, err := exec.Command("git", "-C", dirs[0], "worktree", "list", "--porcelain").Output()
	if err != nil {
		return dirs
	}
	for _, line := range strings.Split(string(out), "\n") {
		if path, ok := strings.CutPrefix(line, "worktree "); ok {
			path = filepath.Clean(path)
			if !seen[path] {
				seen[path] = true
				dirs = append(dirs, path)
			}
		}
	}
	return dirs
}

// claudeHome is `$CLAUDE_CONFIG_DIR`, else `~/.claude`; empty when no home directory is known.
func claudeHome(env map[string]string) string {
	if configDir := env["CLAUDE_CONFIG_DIR"]; configDir != "" {
		return configDir
	}
	if home := env["HOME"]; home != "" {
		return filepath.Join(home, ".claude")
	}
	if home := env["USERPROFILE"]; home != "" {
		return filepath.Join(home, ".claude")
	}
	return ""
}
