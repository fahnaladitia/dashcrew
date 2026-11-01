// Command dashcrew renders the AI toolkit of a project from `.ai/project.yaml` and checks it.
// `dashcrew guard` is the guard hook that `.ai/bin/guard` runs.
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/check"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/initproject"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/sync"
	"github.com/fahnaladitia/dashcrew/internal/version"
	"github.com/fahnaladitia/dashcrew/internal/worktree"
)

// stdin is the hook input `dashcrew guard` reads and the answers `dashcrew init` reads.
var stdin io.Reader = os.Stdin

func main() {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		if key, value, ok := strings.Cut(entry, "="); ok {
			env[key] = value
		}
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, env))
}

// exitError ends the run with its message on stderr and its code, without usage text.
type exitError struct {
	code    int
	message string
}

func (e *exitError) Error() string { return e.message }

// cliIO holds the streams and environment of one run.
type cliIO struct {
	stdout, stderr io.Writer
	env            map[string]string
}

func (c cliIO) out(line string) { fmt.Fprintln(c.stdout, line) }

// run is the CLI: exit 0 on success, 1 when a check fails or a command is refused, 64 on a usage error.
func run(args []string, stdout, stderr io.Writer, env map[string]string) int {
	if len(args) > 1 && args[0] == "guard" && args[1] == "install" {
		fmt.Fprintln(stderr, guardInstallRemoved)
		return 64
	}
	if isGuardHook(args) {
		return guard.Run(args[1:], stdin, stdout, stderr, guard.ProcessEnv)
	}
	code, err := runCommand(newRoot(cliIO{stdout, stderr, env}), args, stdout, stderr)
	var usage *usageError
	var exit *exitError
	switch {
	case errors.As(err, &usage):
		fmt.Fprintf(stderr, "%s\n\n%s\n", usage.message, usage.command.usageWithoutDescription())
		return 64
	case errors.As(err, &exit):
		fmt.Fprintln(stderr, exit.message)
		return exit.code
	case err != nil:
		fmt.Fprintln(stderr, err)
		return 1
	}
	return code
}

// guardInstallRemoved answers `guard install`, which a shim from before the switch to `dashcrew guard` still calls.
const guardInstallRemoved = "dashcrew guard install was removed: hooks run dashcrew guard from PATH. " +
	"Run dashcrew sync to update .ai/bin/guard."

// compileGuardNotice is printed when `sync` gets `--[no-]compile-guard`, a flag kept so old scripts still run.
const compileGuardNotice = "note: --compile-guard is deprecated and does nothing: hooks run dashcrew guard from PATH."

// isGuardHook is `dashcrew guard [<policy>]`: anything after guard but help belongs to the hook.
func isGuardHook(args []string) bool {
	if len(args) == 0 || args[0] != "guard" {
		return false
	}
	return len(args) == 1 || (args[1] != "--help" && args[1] != "-h")
}

func ptr(value string) *string { return &value }

// absolute is Dart's `Directory(path).absolute.path`: the working directory joined without normalizing.
func absolute(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(cwd, string(filepath.Separator)) {
		return cwd + path, nil
	}
	return cwd + string(filepath.Separator) + path, nil
}

func newRoot(cli cliIO) *command {
	root := newCommand(executableName, "Agent-agnostic AI coding toolkit with Flutter built in.")
	root.flag("version", "", "Print the dashcrew version.", false, false)
	root.versionLine = executableName + " " + version.Current()
	root.add(newHelpCommand(root, cli.stdout)).
		add(initCommand(cli)).
		add(syncCommand(cli)).
		add(doctorCommand(cli)).
		add(worktreeCommand(cli)).
		add(guardCommand()).
		add(checkCommand(cli))
	return root
}

func initCommand(cli cliIO) *command {
	c := newCommand("init", "Detect the platform and stack and write .ai/project.yaml. Writes nothing else.")
	c.option("project", "p", "Project root.", ptr("."))
	c.flag("yes", "y", "Write the detected profile without asking.", false, false)
	c.multiOption("agents", "Agents to render for.", profile.SupportedAgents, []string{"claude-code"})
	c.flag("force", "", "Replace an existing .ai/project.yaml.", false, false)
	c.run = func(r *results) (int, error) {
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		projectDir = filepath.Clean(projectDir)
		cat, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		reader := bufio.NewReader(stdin)
		ask := func(prompt string) *string {
			fmt.Fprint(cli.stdout, prompt)
			line, err := reader.ReadString('\n')
			if err != nil && line == "" {
				return nil
			}
			line = strings.TrimSuffix(line, "\n")
			line = strings.TrimSuffix(line, "\r")
			return &line
		}
		in := initproject.Init{Catalog: cat, ProjectDir: projectDir, Agents: r.multiOption("agents")}
		return 0, in.Run(r.flag("yes"), r.flag("force"), initproject.RemoteBranches(projectDir), ask, cli.out)
	}
	return c
}

func syncCommand(cli cliIO) *command {
	c := newCommand("sync", "Render rules, roles, skills, guard and hooks for the agents in .ai/project.yaml.")
	c.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	c.flag("force", "", "Overwrite generated files that were edited by hand.", false, false)
	c.flag("adopt", "", "Keep an existing AGENTS.md as the project block.", false, false)
	c.flag("compile-guard", "", "Deprecated, does nothing: hooks run dashcrew guard from PATH.", false, true)
	c.run = func(r *results) (int, error) {
		if r.wasParsed("compile-guard") {
			fmt.Fprintln(cli.stderr, compileGuardNotice)
		}
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		cat, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		result, err := sync.Sync{Catalog: cat, ProjectDir: projectDir}.Run(r.flag("force"), r.flag("adopt"))
		if err != nil {
			return 0, err
		}
		// Go only (G4b): doctor's guard-log check counts malformed payloads from here on. Best effort.
		_ = guard.MarkSynced(projectDir, time.Now())
		for _, path := range result.Written {
			cli.out("wrote   " + path)
		}
		for _, path := range result.Removed {
			cli.out("removed " + path)
		}
		for _, warning := range result.Warnings {
			fmt.Fprintln(cli.stderr, "warning: "+warning)
		}
		for _, note := range result.Notes {
			cli.out("note: " + note)
		}
		if len(result.Written) == 0 && len(result.Removed) == 0 {
			cli.out("up to date")
		}
		return 0, nil
	}
	return c
}

// printChecks prints one line per check and, when annotate is on (GitHub Actions), an `::error` line per failure.
// The exit code is 1 when any check fails.
func printChecks(stdout io.Writer, checks []doctor.Check, annotate bool) int {
	code := 0
	for _, c := range checks {
		fmt.Fprintf(stdout, "%-6s%-16s%s\n", c.Status, c.ID, c.Detail)
		if c.Status == doctor.Fail {
			code = 1
			if annotate {
				fmt.Fprintf(stdout, "::error title=dashcrew %s::%s\n", c.ID, c.Detail)
			}
		}
	}
	return code
}

func doctorCommand(cli cliIO) *command {
	c := newCommand("doctor", "Check that the generated toolkit is installed, untouched and current. Writes nothing.")
	c.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	c.run = func(r *results) (int, error) {
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		cat, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		checks := doctor.Doctor{Catalog: cat, ProjectDir: projectDir, MachineChecks: true}.Run()
		return printChecks(cli.stdout, checks, false), nil
	}
	return c
}

func checkCommand(cli cliIO) *command {
	c := newCommand("check", "Merge gate for a pull request: toolkit current, PR sections present. Writes nothing.")
	c.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	c.option("pr-body", "", "File holding the PR description; a pull_request workflow supplies it from the event.", nil)
	c.run = func(r *results) (int, error) {
		var body *string
		if bodyFile := r.option("pr-body"); bodyFile != nil {
			info, err := os.Stat(*bodyFile)
			if err != nil || info.IsDir() {
				return 0, &usageError{c, "--pr-body file " + *bodyFile + " not found."}
			}
			data, err := os.ReadFile(*bodyFile)
			if err != nil {
				return 0, err
			}
			text := string(data)
			body = &text
		}
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		cat, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		checks, err := check.PrCheck{Catalog: cat, ProjectDir: projectDir, Environment: cli.env}.Run(body)
		if err != nil {
			return 0, err
		}
		return printChecks(cli.stdout, checks, cli.env["GITHUB_ACTIONS"] == "true"), nil
	}
	return c
}

func worktreeCommand(cli cliIO) *command {
	branch := newCommand("worktree", "One worktree per parallel task. Never commits, pushes or deletes a branch.")
	sub := func(name, description, invocation string) *command {
		c := newCommand(name, description)
		c.invocation = invocation
		c.option("project", "p", "Any folder inside the repository.", ptr("."))
		branch.add(c)
		return c
	}
	// open comes before the positional arguments, as the Dart `worktrees` getter is evaluated first.
	open := func(r *results) (*worktree.Worktrees, error) { return worktree.New(*r.option("project"), nil) }
	positional := func(c *command, r *results, index int, label string) (string, error) {
		if len(r.rest) <= index {
			return "", &usageError{c, "Missing <" + label + ">."}
		}
		return r.rest[index], nil
	}

	create := sub("create", "Create <branch> from <base> in <repo>.worktrees/<name>, with no upstream and the toolkit linked.",
		"dashcrew worktree create <branch> <base>")
	create.option("name", "", "Folder name; defaults to the branch with / replaced by -.", nil)
	create.flag("pub", "", "Run the platform's deps command (platform.commands.deps) in the new worktree.", true, true)
	create.run = func(r *results) (int, error) {
		w, err := open(r)
		if err != nil {
			return 0, err
		}
		name, err := positional(create, r, 0, "branch")
		if err != nil {
			return 0, err
		}
		base, err := positional(create, r, 1, "base")
		if err != nil {
			return 0, err
		}
		_, err = w.Create(name, base, r.option("name"), r.flag("pub"), cli.out)
		return 0, err
	}

	list := sub("list", "Show each task worktree: branch, base, upstream, dirty lines, missing links.", "")
	list.run = func(r *results) (int, error) {
		w, err := open(r)
		if err != nil {
			return 0, err
		}
		infos, err := w.List()
		if err != nil {
			return 0, err
		}
		if len(infos) == 0 {
			cli.out("no task worktrees")
		}
		orDash := func(value *string) string {
			if value == nil {
				return "-"
			}
			return *value
		}
		for _, info := range infos {
			links := "links ok"
			if len(info.MissingLinks) > 0 {
				links = "missing links: " + strings.Join(info.MissingLinks, ", ")
			}
			cli.out(info.Name + "  " + info.Branch + "  base " + orDash(info.Base) + "  upstream " + orDash(info.Upstream) +
				"  dirty " + strconv.Itoa(info.Dirty) + "  " + links)
		}
		return 0, nil
	}

	link := sub("link", "Add missing toolkit links to a worktree and drop an upstream it should not have.",
		"dashcrew worktree link <name>")
	link.run = func(r *results) (int, error) {
		w, err := open(r)
		if err != nil {
			return 0, err
		}
		name, err := positional(link, r, 0, "name")
		if err != nil {
			return 0, err
		}
		_, err = w.Link(name, cli.out)
		return 0, err
	}

	remove := sub("remove", "Remove a clean worktree. The branch is kept.", "dashcrew worktree remove <name>")
	remove.run = func(r *results) (int, error) {
		w, err := open(r)
		if err != nil {
			return 0, err
		}
		name, err := positional(remove, r, 0, "name")
		if err != nil {
			return 0, err
		}
		return 0, w.Remove(name, cli.out)
	}

	watch := sub("watch", "Print one line per new event (commit, pushed, pr, base-moved, merge-tree, overlap) until done.",
		"dashcrew worktree watch [<name>...]")
	watch.flag("once", "", "Run one pass and exit.", false, false)
	watch.option("state-dir", "", "Where seen events are kept; defaults to <git dir>/dashcrew-watch.", nil)
	watch.option("interval", "", "Seconds between passes.", ptr("60"))
	watch.run = func(r *results) (int, error) {
		w, err := open(r)
		if err != nil {
			return 0, err
		}
		interval, ok := dartTryParseInt(*r.option("interval"))
		if !ok || interval < 1 {
			return 0, &usageError{watch, "--interval must be a positive number of seconds."}
		}
		stateDir := w.GitDir + string(filepath.Separator) + "dashcrew-watch"
		if dir := r.option("state-dir"); dir != nil {
			stateDir = *dir
		}
		return 0, (&worktree.Watch{Worktrees: w, StateDir: stateDir}).Run(r.rest, r.flag("once"),
			time.Duration(interval)*time.Second, cli.out)
	}
	return branch
}

// dartTryParseInt is Dart's `int.tryParse` without a radix: surrounding whitespace, a sign and a `0x` prefix.
func dartTryParseInt(text string) (int64, bool) {
	text = strings.TrimSpace(text)
	sign := ""
	if strings.HasPrefix(text, "+") || strings.HasPrefix(text, "-") {
		sign, text = text[:1], text[1:]
	}
	base := 10
	if strings.HasPrefix(text, "0x") || strings.HasPrefix(text, "0X") {
		base, text = 16, text[2:]
	}
	if text == "" || strings.ContainsAny(text, "+-_") {
		return 0, false
	}
	value, err := strconv.ParseInt(sign+text, base, 64)
	return value, err == nil
}

// guardCommand holds the help text of `dashcrew guard`; running it is the hook (see isGuardHook), so the parser only
// reaches it for `--help` and `help guard`.
func guardCommand() *command {
	c := newCommand("guard", "Check one agent tool call: hook JSON on stdin, the answer on stdout or stderr and in the exit code.\n"+
		"Hooks run it through .ai/bin/guard. The policy is <policy>, else .ai/guard.json in the hook's working directory.")
	c.invocation = "dashcrew guard [<policy>]"
	c.run = func(r *results) (int, error) {
		return guard.Run(r.rest, stdin, os.Stdout, os.Stderr, guard.ProcessEnv), nil
	}
	return c
}
