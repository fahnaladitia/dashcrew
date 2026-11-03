package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"strings"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/me"
)

func meCommand(cli cliIO) *command {
	branch := newCommand("me", "Your personal profile, ~/.dashcrew/me.yaml: how agents work with you in every project.\n"+
		"It never changes what a project allows, and the guard never reads it.")
	places := func() (me.Places, error) { return me.PlacesFrom(cli.env) }
	personal := func() (fs.FS, error) { return fs.Sub(embedded.PersonalFS, "personal") }

	show := newCommand("show", "Print your profile and whether the personal files are in step with it.")
	show.run = func(r *results) (int, error) {
		pl, err := places()
		if err != nil {
			return 0, err
		}
		p, err := me.LoadOrNil(pl.Home)
		if err != nil {
			return 0, err
		}
		if p == nil {
			cli.out("no profile: run dashcrew me init")
			return 0, nil
		}
		cli.out(strings.TrimSuffix(p.Text(), "\n"))
		fsys, err := personal()
		if err != nil {
			return 0, err
		}
		changes, err := me.Status(p, pl, fsys)
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

	initCmd := newCommand("init", "Write ~/.dashcrew/me.yaml, proposing values from the repository you run it in.\n"+
		"Run it again in a repository of another platform to add that platform to your stacks.")
	initCmd.option("project", "p", "A repository whose .ai/project.yaml names a platform.", ptr("."))
	initCmd.flag("yes", "y", "Write the proposed profile without asking.", false, false)
	initCmd.run = func(r *results) (int, error) {
		pl, err := places()
		if err != nil {
			return 0, err
		}
		existing, err := me.LoadOrNil(pl.Home)
		if err != nil {
			return 0, err
		}
		dir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		p := me.Propose(existing, me.RepoPlatform(dir), pl)
		if !r.flag("yes") {
			reader := bufio.NewReader(stdin)
			ask := func(key, help, proposed string) string {
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
			p.Role = ask("role", strings.Join(me.Roles, " | "), p.Role)
			p.Stacks = list(ask("stacks", "platforms you work on, comma separated", strings.Join(p.Stacks, ", ")))
			p.Level = ask("level", strings.Join(me.Levels, " | "), p.Level)
			p.ChatLanguage = ask("chat_language", "ISO 639-1 such as id, empty keeps each project's", p.ChatLanguage)
			p.Agents = list(ask("agents", strings.Join(me.Agents, ", "), strings.Join(p.Agents, ", ")))
		}
		checked, err := me.Parse(p.Text())
		if err != nil {
			return 1, &exitError{1, err.Error()}
		}
		if err := checked.Save(pl.Home); err != nil {
			return 0, err
		}
		cli.out("wrote " + me.Path(pl.Home))
		cli.out("next: dashcrew me sync")
		return 0, nil
	}
	branch.add(initCmd)

	syncCmd := newCommand("sync", "Write your personal instructions where each agent in me.yaml reads them, and link\n"+
		"your personal skills for Claude Code. A file dashcrew did not write is never overwritten.")
	syncCmd.flag("remove", "", "Take out every personal file, block and link dashcrew wrote; me.yaml stays.", false, false)
	syncCmd.run = func(r *results) (int, error) {
		pl, err := places()
		if err != nil {
			return 0, err
		}
		var changes []me.Change
		if r.flag("remove") {
			changes, err = me.Remove(pl)
		} else {
			var p *me.Profile
			if p, err = me.LoadOrNil(pl.Home); err != nil {
				return 0, err
			}
			if p == nil {
				return 1, &exitError{1, "no profile: run dashcrew me init"}
			}
			var fsys fs.FS
			if fsys, err = personal(); err != nil {
				return 0, err
			}
			changes, err = me.Sync(p, pl, fsys)
		}
		for _, change := range changes {
			cli.out(change.String())
		}
		if err != nil {
			return 0, err
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
	branch.add(syncCmd)
	return branch
}
