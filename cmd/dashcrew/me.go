package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/me"
	"github.com/fahnaladitia/dashcrew/internal/worktree"
)

// meRepoRoot is the dashcrew repo holding dir: dir itself or its git top level, when it has .ai/project.yaml;
// empty outside a dashcrew repo.
func meRepoRoot(dir string) string {
	candidates := []string{dir}
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output(); err == nil {
		candidates = append(candidates, filepath.Clean(strings.TrimSpace(string(out))))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(filepath.Join(candidate, ".ai", "project.yaml")); err == nil {
			if real, err := filepath.EvalSymlinks(candidate); err == nil {
				return real
			}
			return candidate
		}
	}
	return ""
}

// meExclude lists the repo's personal files in its git exclude file; nothing outside a git checkout.
func meExclude(root string) error {
	if exec.Command("git", "-C", root, "rev-parse", "--git-dir").Run() != nil {
		return nil
	}
	return worktree.Exclude(root, me.RepoFiles)
}

func meCommand(cli cliIO) *command {
	branch := newCommand("me", "Your personal profile: ~/.dashcrew/me.yaml for you, a repo's .ai/me.yaml for your role there.\n"+
		"It never changes what a project allows, and the guard never reads it.")
	places := func() (me.Places, error) { return me.PlacesFrom(cli.env) }
	personal := func() (fs.FS, error) { return fs.Sub(embedded.PersonalFS, "personal") }
	root := func(r *results) (string, error) {
		dir, err := absolute(*r.option("project"))
		if err != nil {
			return "", err
		}
		return meRepoRoot(dir), nil
	}
	report := func(changes []me.Change) (int, error) {
		for _, change := range changes {
			cli.out(change.String())
		}
		if len(changes) == 0 {
			cli.out("nothing to do")
		}
		for _, change := range changes {
			if change.Kept {
				return 1, &exitError{1, "kept paths dashcrew did not write; move them away and run dashcrew me sync again"}
			}
		}
		return 0, nil
	}
	// syncAll registers repo (when it has .ai/me.yaml), excludes its personal files, and syncs everything.
	syncAll := func(pl me.Places, home *me.Profile, repo string) (int, error) {
		fsys, err := personal()
		if err != nil {
			return 0, err
		}
		if repo != "" {
			if profile, err := me.LoadRepo(repo); err != nil {
				return 0, err
			} else if profile != nil {
				if err := meExclude(repo); err != nil {
					return 0, err
				}
				if err := me.Register(pl, repo); err != nil {
					return 0, err
				}
			}
		}
		changes, err := me.Sync(home, pl, fsys)
		if err != nil {
			return 0, err
		}
		return report(changes)
	}

	show := newCommand("show", "Print your profile, your role and level in this repo, and whether the personal files are in step.")
	show.option("project", "p", "Any folder inside the repository.", ptr("."))
	show.run = func(r *results) (int, error) {
		pl, err := places()
		if err != nil {
			return 0, err
		}
		home, err := me.LoadOrNil(pl.Home)
		if err != nil {
			return 0, err
		}
		if home == nil {
			cli.out("no profile: run dashcrew me init")
			return 0, nil
		}
		cli.out(strings.TrimSuffix(home.Text(), "\n"))
		repo, err := root(r)
		if err != nil {
			return 0, err
		}
		if repo != "" {
			profile, err := me.LoadRepo(repo)
			if err != nil {
				return 0, err
			}
			cli.out("this repo: " + me.EffectiveIn(home, profile).Describe())
		}
		repos, err := me.Repos(pl)
		if err != nil {
			return 0, err
		}
		for _, dir := range repos {
			cli.out("repo: " + dir)
		}
		fsys, err := personal()
		if err != nil {
			return 0, err
		}
		changes, err := me.Status(home, pl, fsys)
		if err != nil {
			return 0, err
		}
		if len(changes) == 0 {
			cli.out("personal files: in step")
			return 0, nil
		}
		for _, change := range changes {
			cli.out("personal files: " + change.String())
		}
		cli.out("next: dashcrew me sync")
		return 0, nil
	}
	branch.add(show)

	initCmd := newCommand("init", "Set up your profile. The first time, or with --home: ~/.dashcrew/me.yaml (role, stacks,\n"+
		"level, chat language, agents). Inside a dashcrew repo afterwards: your role and level there, kept\n"+
		"in .ai/me.yaml when they differ from home (listed in .git/info/exclude). Then dashcrew me sync.")
	initCmd.option("project", "p", "Any folder inside the repository.", ptr("."))
	initCmd.flag("home", "", "Only write ~/.dashcrew/me.yaml.", false, false)
	initCmd.flag("yes", "y", "Take the proposed values without asking.", false, false)
	initCmd.run = func(r *results) (int, error) {
		pl, err := places()
		if err != nil {
			return 0, err
		}
		home, err := me.LoadOrNil(pl.Home)
		if err != nil {
			return 0, err
		}
		dir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		repo := ""
		if !r.flag("home") {
			repo = meRepoRoot(dir)
		}
		reader := bufio.NewReader(stdin)
		ask := func(key, help, proposed string) string {
			if r.flag("yes") {
				return proposed
			}
			fmt.Fprintf(cli.stdout, "%s (%s) [%s]: ", key, help, proposed)
			line, _ := reader.ReadString('\n')
			if line = strings.TrimSpace(line); line != "" {
				return line
			}
			return proposed
		}
		list := func(answer string) []string {
			return strings.FieldsFunc(answer, func(r rune) bool { return r == ',' || r == ' ' })
		}
		var repoProfile *me.RepoProfile
		if home == nil || r.flag("home") {
			platformDir := dir
			if repo != "" {
				platformDir = repo
			}
			p := me.Propose(home, me.RepoPlatform(platformDir), pl)
			p.Role = ask("role", strings.Join(me.Roles, " | "), p.Role)
			p.Stacks = list(ask("stacks", "platforms you work on, comma separated", strings.Join(p.Stacks, ", ")))
			p.Level = ask("level", strings.Join(me.Levels, " | "), p.Level)
			p.ChatLanguage = ask("chat_language", "ISO 639-1 such as id, empty keeps each project's", p.ChatLanguage)
			p.Agents = list(ask("agents", strings.Join(me.Agents, ", "), strings.Join(p.Agents, ", ")))
			if home, err = me.Parse(p.Text()); err != nil {
				return 1, &exitError{1, err.Error()}
			}
			if err := home.Save(pl.Home); err != nil {
				return 0, err
			}
			cli.out("wrote " + me.Path(pl.Home))
			if repo != "" {
				return syncAll(pl, home, repo)
			}
		} else if repo != "" {
			proposed, err := me.LoadRepo(repo)
			if err != nil {
				return 0, err
			}
			effective := me.EffectiveIn(home, proposed)
			text := "role: " + ask("role in this repo", strings.Join(me.Roles, " | "), effective.Role) + "\n" +
				"level: " + ask("level in this repo", strings.Join(me.Levels, " | "), effective.Level) + "\n"
			if repoProfile, err = me.ParseRepo(text); err != nil {
				return 1, &exitError{1, err.Error()}
			}
			if repoProfile.Role == home.Role {
				repoProfile.Role = ""
			}
			if repoProfile.Level == home.Level {
				repoProfile.Level = ""
			}
			if *repoProfile == (me.RepoProfile{}) {
				return meForget(cli, pl, home, repo)
			}
		}
		if repo == "" {
			cli.out("next: dashcrew me sync")
			return 0, nil
		}
		if err := repoProfile.Save(repo); err != nil {
			return 0, err
		}
		cli.out("wrote " + filepath.Join(repo, filepath.FromSlash(me.RepoFile)))
		return syncAll(pl, home, repo)
	}
	branch.add(initCmd)

	syncCmd := newCommand("sync", "Write your personal instructions where each agent in me.yaml reads them: in your home folders,\n"+
		"and in each repo with an .ai/me.yaml you set up. A file dashcrew did not write is never overwritten.")
	syncCmd.option("project", "p", "Any folder inside the repository.", ptr("."))
	syncCmd.flag("remove", "", "Take out every personal file, block and link dashcrew wrote, in home and in every repo; the me.yaml files stay.", false, false)
	syncCmd.flag("remove-repo", "", "Take out this repo's personal files and forget it; its .ai/me.yaml stays.", false, false)
	syncCmd.flag("prune", "", "Forget repos whose .ai/me.yaml is gone, then sync.", false, false)
	syncCmd.run = func(r *results) (int, error) {
		pl, err := places()
		if err != nil {
			return 0, err
		}
		if r.flag("remove") {
			changes, err := me.Remove(pl)
			if err != nil {
				return 0, err
			}
			return report(changes)
		}
		home, err := me.LoadOrNil(pl.Home)
		if err != nil {
			return 0, err
		}
		if home == nil {
			return 1, &exitError{1, "no profile: run dashcrew me init"}
		}
		repo, err := root(r)
		if err != nil {
			return 0, err
		}
		if r.flag("remove-repo") {
			if repo == "" {
				return 1, &exitError{1, "not inside a dashcrew repo: pass -p <repo>"}
			}
			fsys, err := personal()
			if err != nil {
				return 0, err
			}
			changes, err := me.RemoveRepo(home, pl, fsys, repo)
			if err != nil {
				return 0, err
			}
			return report(changes)
		}
		if r.flag("prune") {
			gone, err := me.Prune(pl)
			if err != nil {
				return 0, err
			}
			for _, dir := range gone {
				cli.out("forgot  " + dir)
			}
		}
		return syncAll(pl, home, repo)
	}
	branch.add(syncCmd)
	return branch
}

// meForget drops a repo's .ai/me.yaml when it holds nothing the home profile does not, and its personal files.
func meForget(cli cliIO, pl me.Places, home *me.Profile, repo string) (int, error) {
	cli.out("role and level match your home profile; nothing to keep in this repo")
	if err := os.Remove(filepath.Join(repo, filepath.FromSlash(me.RepoFile))); err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	fsys, err := fs.Sub(embedded.PersonalFS, "personal")
	if err != nil {
		return 0, err
	}
	changes, err := me.RemoveRepo(home, pl, fsys, repo)
	if err != nil {
		return 0, err
	}
	for _, change := range changes {
		if change.What != "unchanged" {
			cli.out(change.String())
		}
	}
	return 0, nil
}
