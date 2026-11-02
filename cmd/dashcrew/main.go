// Command dashcrew renders the AI toolkit of a project from `.ai/project.yaml` and checks it.
// `dashcrew guard` is the guard hook that `.ai/bin/guard` runs.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/check"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/initproject"
	"github.com/fahnaladitia/dashcrew/internal/knowledge"
	"github.com/fahnaladitia/dashcrew/internal/knowledgemcp"
	"github.com/fahnaladitia/dashcrew/internal/ledger"
	"github.com/fahnaladitia/dashcrew/internal/lines"
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
		add(knowledgeCommand(cli)).
		add(guardCommand()).
		add(checkCommand(cli)).
		add(commitCommand(cli)).
		add(verifyCommand(cli)).
		add(releaseCommand(cli)).
		add(specCommand(cli)).
		add(linkCommand(cli))
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
	c.flag("upgrade", "", "On a release line, take this dashcrew's catalog instead of holding its changes.", false, false)
	c.flag("after-merge", "", "Render fresh the generated files that carry merge conflict markers or disagree with the lock.", false, false)
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
		opts := sync.Options{Force: r.flag("force"), Adopt: r.flag("adopt"), Upgrade: r.flag("upgrade"), AfterMerge: r.flag("after-merge")}
		result, err := sync.Sync{Catalog: cat, ProjectDir: projectDir}.RunWith(opts)
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
		if opts.AfterMerge {
			for _, path := range result.Resolved {
				cli.out("fresh   " + path + " (conflict markers or out of step with the lock)")
			}
			if len(result.Stage) > 0 {
				cli.out("Stage them yourself once they look right (dashcrew never stages or commits):")
				cli.out("  git add -- " + strings.Join(result.Stage, " "))
			}
		}
		if len(result.Held) > 0 {
			fmt.Fprintln(cli.stderr, result.HeldMessage)
			return 1, nil
		}
		if len(result.Written) == 0 && len(result.Removed) == 0 && len(result.Stage) == 0 {
			cli.out("up to date")
		}
		return 0, nil
	}
	return c
}

func releaseCommand(cli cliIO) *command {
	branch := newCommand("release", "Release lines from project.release_lines. Reads git only.")
	linesCmd := newCommand("lines", "List the release and hotfix lines oldest first, then the main line, with their head commits,\n"+
		"and the line of the current branch with how it was found.")
	linesCmd.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	linesCmd.run = func(r *results) (int, error) {
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		cat, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		p, err := sync.Sync{Catalog: cat, ProjectDir: projectDir}.Profile()
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		if p.ReleaseLines == nil {
			return 0, &exitError{1, "No release_lines in .ai/project.yaml; add project.release_lines: { main, release, hotfix } first."}
		}
		all, err := lines.List(projectDir, p.ReleaseLines)
		if err != nil {
			return 0, &exitError{1, projectDir + " is not inside a git repository"}
		}
		versionWidth, branchWidth := len("main"), len(p.ReleaseLines.Main)
		for _, line := range all {
			versionWidth, branchWidth = max(versionWidth, len(line.Version)), max(branchWidth, len(line.Branch))
		}
		row := func(label, name, head string) {
			cli.out(fmt.Sprintf("  %-*s  %-*s  %s", versionWidth, label, branchWidth, name, head))
		}
		cli.out("lines (oldest first):")
		if len(all) == 0 || (len(all) == 1 && lines.Main(all) != nil) {
			cli.out("  no release or hotfix line branch found")
		}
		for _, line := range all {
			if line.Kind != profile.MainLine {
				row(line.Version, line.Branch, line.Head[:min(7, len(line.Head))])
			}
		}
		if main := lines.Main(all); main != nil {
			row("main", main.Branch, main.Head[:min(7, len(main.Head))])
		} else {
			row("main", p.ReleaseLines.Main, "no such branch")
		}
		cli.out("current: " + lines.Resolve(projectDir, p.ReleaseLines, all, "").Describe())
		return 0, nil
	}
	statusCmd := newCommand("status", "Regenerate <plans>/<version>/README.md, the release ledger: the task folders with their branch\n"+
		"and PR description, the commits on the line that carry Ported-from:, and the pending ports.\n"+
		"Writes only that file; reads git.")
	statusCmd.invocation = "dashcrew release status <version> [arguments]"
	statusCmd.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	statusCmd.run = func(r *results) (int, error) {
		if len(r.rest) != 1 {
			return 0, &usageError{statusCmd, "Pass one version, such as 2.4.5."}
		}
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		cat, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		p, err := sync.Sync{Catalog: cat, ProjectDir: projectDir}.Profile()
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		if p.ReleaseLines == nil {
			return 0, &exitError{1, "No release_lines in .ai/project.yaml; add project.release_lines: { main, release, hotfix } first."}
		}
		all, err := lines.List(projectDir, p.ReleaseLines)
		if err != nil {
			return 0, &exitError{1, projectDir + " is not inside a git repository"}
		}
		text, err := ledger.Status(projectDir, p, all, r.rest[0])
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		plans, _ := p.Paths.Value("plans").(string)
		path, err := ledger.Write(projectDir, plans, r.rest[0], text)
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		cli.out("wrote   " + path)
		cli.out("")
		for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
			cli.out(line)
		}
		return 0, nil
	}
	return branch.add(linesCmd).add(statusCmd)
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
	c.flag("fetch", "", "Fetch the knowledge clone and report how many commits it is behind its remote.", false, false)
	c.run = func(r *results) (int, error) {
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		cat, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		checks := doctor.Doctor{Catalog: cat, ProjectDir: projectDir, MachineChecks: true, Fetch: r.flag("fetch"), Now: clock(cli.env)}.Run()
		return printChecks(cli.stdout, checks, false), nil
	}
	return c
}

func checkCommand(cli cliIO) *command {
	c := newCommand("check", "Merge gate for a pull request: toolkit current, PR sections present, commits in the pattern. Writes nothing.")
	c.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	c.option("pr-body", "", "File holding the PR description; a pull_request workflow supplies it from the event.", nil)
	c.option("base", "", "Lint the commits of <base>..HEAD; a pull_request workflow supplies the range from the event.", nil)
	c.flag("ports", "", "Only check that each fix of an older release line reached the newer one (exit 1 if not).", false, false)
	c.option("from", "", "With --ports: the older line (a version, a line branch or main); default every adjacent pair.", nil)
	c.option("to", "", "With --ports: the newer line; default the line after --from.", nil)
	c.flag("links", "", "Only check how each dependency in project.links is committed (exit 1 if one is not merged).", false, false)
	c.run = func(r *results) (int, error) {
		if r.flag("links") {
			if r.flag("ports") || r.option("pr-body") != nil || r.option("base") != nil {
				return 0, &usageError{c, "--links runs alone, without --ports, --pr-body or --base."}
			}
			return checkLinks(cli, r)
		}
		if r.flag("ports") {
			if r.option("pr-body") != nil || r.option("base") != nil {
				return 0, &usageError{c, "--ports runs alone, without --pr-body or --base."}
			}
			return checkPorts(cli, r)
		}
		if r.option("from") != nil || r.option("to") != nil {
			return 0, &usageError{c, "--from and --to need --ports."}
		}
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
		checks, err := check.PrCheck{Catalog: cat, ProjectDir: projectDir, Environment: cli.env, Base: r.option("base")}.Run(body)
		if err != nil {
			return 0, err
		}
		return printChecks(cli.stdout, checks, cli.env["GITHUB_ACTIONS"] == "true"), nil
	}
	return c
}

// checkLinks is `dashcrew check --links`: one line per dependency, then a summary.
func checkLinks(cli cliIO, r *results) (int, error) {
	projectDir, err := absolute(*r.option("project"))
	if err != nil {
		return 0, err
	}
	cat, err := catalog.LoadEmbedded()
	if err != nil {
		return 0, err
	}
	p, err := sync.Sync{Catalog: cat, ProjectDir: projectDir}.Profile()
	if err != nil {
		return 0, &exitError{1, err.Error()}
	}
	if len(p.Links) == 0 {
		cli.out("no project.links in .ai/project.yaml: nothing to check")
		return 0, nil
	}
	results, err := check.Links(projectDir, p, cat)
	if err != nil {
		return 0, &exitError{1, err.Error()}
	}
	for _, result := range results {
		cli.out(check.Line(result))
	}
	if check.Failed(results) {
		cli.out("fix each refused dependency as its line says, then run dashcrew check --links again")
		return 1, nil
	}
	cli.out("every linked dependency is committed at a merged or released version")
	return 0, nil
}

// checkPorts is `dashcrew check --ports`: one block per line pair, one line per fix, then a summary.
func checkPorts(cli cliIO, r *results) (int, error) {
	projectDir, err := absolute(*r.option("project"))
	if err != nil {
		return 0, err
	}
	cat, err := catalog.LoadEmbedded()
	if err != nil {
		return 0, err
	}
	p, err := sync.Sync{Catalog: cat, ProjectDir: projectDir}.Profile()
	if err != nil {
		return 0, &exitError{1, err.Error()}
	}
	from, to := "", ""
	if value := r.option("from"); value != nil {
		from = *value
	}
	if value := r.option("to"); value != nil {
		to = *value
	}
	pairs, err := check.Ports(projectDir, p, from, to)
	if err != nil {
		return 0, &exitError{1, err.Error()}
	}
	if len(pairs) == 0 {
		cli.out("fewer than two lines: nothing to compare; dashcrew release lines lists them")
		return 0, nil
	}
	problems := 0
	for _, pair := range pairs {
		for _, line := range pair.Lines() {
			cli.out(line)
		}
		problems += pair.Problems()
	}
	if problems > 0 {
		fixes := "1 fix is"
		if problems > 1 {
			fixes = strconv.Itoa(problems) + " fixes are"
		}
		cli.out(fixes + " missing or lost; port them with the port workflow, then run this again")
		return 1, nil
	}
	cli.out("every fix reached the newer lines")
	return 0, nil
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

func knowledgeCommand(cli cliIO) *command {
	branch := newCommand("knowledge", "Shared product knowledge: scaffold the repo, keep its clone current, review proposals, find stale pages.")

	initRepo := newCommand("init", "Write the knowledge repo layout into <dir>, which must be new or empty. Runs no git.")
	initRepo.invocation = "dashcrew knowledge init <dir>"
	initRepo.run = func(r *results) (int, error) {
		if len(r.rest) == 0 {
			return 0, &usageError{initRepo, "Missing <dir>."}
		}
		dir := r.rest[0]
		written, err := knowledge.Init(dir)
		for _, file := range written {
			cli.out("wrote   " + filepath.ToSlash(filepath.Join(dir, file)))
		}
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		cli.out("note: commit it in a new git repo, push it, then set knowledge.repo in each project's .ai/project.yaml.")
		return 0, nil
	}

	pull := newCommand("pull", "Clone the knowledge repo into ~/.dashcrew/knowledge/<name>, or fetch and fast-forward it. Never resets.")
	pull.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	pull.run = func(r *results) (int, error) {
		projectDir, p, err := loadKnowledgeProfile(r)
		if err != nil {
			return 0, err
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return 0, err
		}
		message, err := knowledge.Pull(knowledge.ResolveRepo(p.Knowledge.Repo, projectDir), knowledge.CloneDir(home, p.Knowledge.Name), time.Now())
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		cli.out(message)
		if _, err := os.Lstat(filepath.Join(projectDir, filepath.FromSlash(knowledge.LinkPath))); err != nil {
			cli.out("note: run dashcrew sync to link it as " + knowledge.LinkPath + ".")
		}
		return 0, nil
	}

	review := newCommand("review", "Walk the inbox/ proposals oldest first and accept, edit, skip or delete each.\n"+
		"Accept merges a proposal into its page. Then the README.md index is rewritten and the git commands for a pull\n"+
		"request are printed; it never runs git.")
	review.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	review.option("clone", "", "Knowledge clone to review; defaults to the profile's ~/.dashcrew/knowledge/<name>.", nil)
	review.run = func(r *results) (int, error) {
		clone, _, err := knowledgeClone(r)
		if err != nil {
			return 0, err
		}
		terminal := isTerminal(stdin)
		reader := bufio.NewReader(stdin)
		_, err = knowledge.Review{
			Clone: clone,
			Now:   clock(cli.env),
			Ask: func(prompt string) (string, bool) {
				fmt.Fprint(cli.stdout, prompt)
				line, err := reader.ReadString('\n')
				if err != nil && line == "" {
					fmt.Fprintln(cli.stdout)
					return "", false
				}
				line = strings.TrimRight(line, "\r\n")
				if !terminal {
					fmt.Fprintln(cli.stdout, line)
				}
				return line, true
			},
			Edit: func(path string) error {
				editor := strings.Fields(cli.env["EDITOR"])
				if len(editor) == 0 {
					return errors.New("set $EDITOR to edit a proposal")
				}
				cmd := exec.Command(editor[0], append(editor[1:], path)...)
				if terminal {
					cmd.Stdin = os.Stdin
				}
				cmd.Stdout, cmd.Stderr = cli.stdout, cli.stderr
				return cmd.Run()
			},
			Out: cli.out,
		}.Run()
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		return 0, nil
	}

	doctorCmd := newCommand("doctor", "Report stale code pointers, proposals waiting over 14 days, pages not verified in 180 days and\n"+
		"spec lint problems. Writes nothing; exits 1 when a code pointer's path is gone from its repo or a spec has a problem.")
	doctorCmd.option("project", "p", "Project root; its repo and the sibling folders resolve code pointers.", ptr("."))
	doctorCmd.option("clone", "", "Knowledge clone to check; defaults to the profile's ~/.dashcrew/knowledge/<name>.", nil)
	doctorCmd.run = func(r *results) (int, error) {
		clone, projectDir, err := knowledgeClone(r)
		if err != nil {
			return 0, err
		}
		if info, err := os.Stat(clone); err != nil || !info.IsDir() {
			return 0, &exitError{1, "No knowledge clone at " + clone + "; run dashcrew knowledge pull."}
		}
		report, err := knowledge.Staleness(clone, projectDir, clock(cli.env)())
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		line := func(kind, where, what string) { cli.out(fmt.Sprintf("%-10s%s  %s", kind, where, what)) }
		for _, stale := range report.Missing {
			line("pointer", stale.Page+":"+strconv.Itoa(stale.Pointer.Line), stale.Pointer.String()+" not found in "+stale.RepoDir)
		}
		for _, repo := range report.Unresolved {
			line("unchecked", repo, "no such repo: not the project and no sibling folder of that name")
		}
		for _, old := range report.OldProposals {
			line("proposal", old.Path, fmt.Sprintf("waiting %d days; run dashcrew knowledge review", old.Days))
		}
		for _, page := range report.Unverified {
			if page.Newest == "" {
				line("page", page.Path, "no verified marker")
			} else {
				line("page", page.Path, fmt.Sprintf("newest verified marker %s, %d days ago", page.Newest, page.Days))
			}
		}
		specs, err := specsOf(clone)
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		specProblems := printSpecProblems(specs, func(where, message string) { line("spec", where, message) })
		stale := "none stale"
		if len(report.Missing) > 0 {
			stale = strconv.Itoa(len(report.Missing)) + " stale"
		}
		summary := fmt.Sprintf("%s checked, %s; %s; %s", count(report.Checked, "pointer"), stale,
			count(len(report.OldProposals), "old proposal"), count(len(report.Unverified), "unverified page"))
		if len(specs) > 0 {
			summary += fmt.Sprintf("; %s checked, %s", count(len(specs), "spec file"), count(specProblems, "spec problem"))
		}
		cli.out(summary)
		if len(report.Missing) > 0 || specProblems > 0 {
			return 1, nil
		}
		return 0, nil
	}
	serve := newCommand("serve", "Serve the knowledge clone over stdio MCP: knowledge_search, knowledge_read, knowledge_sources.\n"+
		"sync registers it for every agent when knowledge is set. Writes nothing.")
	serve.option("project", "p", "Project root; its repo and the sibling folders resolve code pointers.", ptr("."))
	serve.option("clone", "", "Knowledge clone to serve; defaults to the profile's ~/.dashcrew/knowledge/<name>.", nil)
	serve.run = func(r *results) (int, error) {
		clone, projectDir, err := knowledgeClone(r)
		if err != nil {
			return 0, err
		}
		if err := knowledgemcp.Run(context.Background(), clone, projectDir, stdin, cli.stdout); err != nil {
			return 0, &exitError{1, err.Error()}
		}
		return 0, nil
	}
	return branch.add(initRepo).add(pull).add(review).add(doctorCmd).add(serve)
}

// loadKnowledgeProfile is the project folder and its profile, which must set `knowledge`.
func loadKnowledgeProfile(r *results) (string, *profile.Profile, error) {
	projectDir, err := absolute(*r.option("project"))
	if err != nil {
		return "", nil, err
	}
	cat, err := catalog.LoadEmbedded()
	if err != nil {
		return "", nil, err
	}
	p, err := sync.Sync{Catalog: cat, ProjectDir: projectDir}.Profile()
	if err != nil {
		return "", nil, err
	}
	if p.Knowledge == nil {
		return "", nil, &exitError{1, "No knowledge in .ai/project.yaml; add knowledge: { repo: <git url or path> } first."}
	}
	return projectDir, p, nil
}

// knowledgeClone is --clone, with a leading `~/` expanded as the server registration needs, else the profile's
// clone, with the project folder.
func knowledgeClone(r *results) (string, string, error) {
	if clone := r.option("clone"); clone != nil {
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return "", "", err
		}
		path := *clone
		if strings.HasPrefix(path, "~/") {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", "", err
			}
			path = knowledge.ExpandHome(path, home)
		}
		return filepath.Clean(path), filepath.Clean(projectDir), nil
	}
	projectDir, p, err := loadKnowledgeProfile(r)
	if err != nil {
		return "", "", err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}
	return knowledge.CloneDir(home, p.Knowledge.Name), filepath.Clean(projectDir), nil
}

// clock is the time SOURCE_DATE_EPOCH names when it is set, so a recorded run dates its output the same every time;
// else the current time.
func clock(env map[string]string) func() time.Time {
	if seconds, err := strconv.ParseInt(env["SOURCE_DATE_EPOCH"], 10, 64); err == nil {
		return func() time.Time { return time.Unix(seconds, 0).UTC() }
	}
	return time.Now
}

// isTerminal is true when r is a terminal, where the user's typing already shows the answer.
func isTerminal(r io.Reader) bool {
	file, ok := r.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func count(n int, word string) string {
	switch n {
	case 0:
		return "no " + word + "s"
	case 1:
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
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
