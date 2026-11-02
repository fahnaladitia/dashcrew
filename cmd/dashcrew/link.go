package main

import (
	"os"
	"path/filepath"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/link"
)

func linkCommand(cli cliIO) *command {
	branch := newCommand("link", "Point a dependency in project.links at a local checkout. Writes only a git-excluded file.\n"+
		"The file is the platform's link file: pubspec_overrides.yaml or go.work.")
	sub := func(name, description, invocation string) *command {
		c := newCommand(name, description)
		c.invocation = invocation
		c.option("project", "p", "Any folder inside the repository.", ptr("."))
		branch.add(c)
		return c
	}
	open := func(r *results) (*link.Links, error) {
		c, err := catalog.LoadEmbedded()
		if err != nil {
			return nil, err
		}
		return link.Open(*r.option("project"), c)
	}
	show := func(l *link.Links, entries []link.Entry) {
		if len(entries) == 0 {
			cli.out("no links")
			return
		}
		branch := link.Branch(l.Dir)
		for _, entry := range entries {
			target := filepath.Join(l.Dir, filepath.FromSlash(entry.Path))
			line := entry.Package + "  " + entry.Path
			switch targetBranch := link.Branch(target); {
			case !isDir(target):
				line += "  missing"
			case targetBranch == "":
				line += "  not on a branch"
			case targetBranch == branch:
				line += "  branch " + targetBranch
			default:
				line += "  branch " + targetBranch + " (this checkout: " + branch + ")"
			}
			cli.out(line)
		}
	}

	add := sub("add", "Link <package> to the checkout at <path>, after checking that <path> holds that package.",
		"dashcrew link add <package> <path>")
	add.flag("force", "", "Replace an override file dashcrew did not write.", false, false)
	add.flag("deps", "", "Run the platform's deps command afterwards.", true, true)
	add.run = func(r *results) (int, error) {
		if len(r.rest) != 2 {
			return 0, &usageError{add, "Pass <package> and <path>."}
		}
		l, err := open(r)
		if err != nil {
			return 0, err
		}
		entries, err := l.Add(r.rest[0], r.rest[1], r.flag("force"))
		if err != nil {
			return 0, err
		}
		show(l, entries)
		if r.flag("deps") {
			cli.out("deps: " + l.Deps())
		}
		if name, _ := l.File(); name == "pubspec_overrides.yaml" {
			cli.out("note: pubspec.lock now resolves " + r.rest[0] + " from that path; dashcrew link remove restores it, and check --links refuses it until then")
		}
		return 0, nil
	}

	list := sub("ls", "Show each link: package, path, and the branch of the linked checkout.", "dashcrew link ls")
	list.run = func(r *results) (int, error) {
		l, err := open(r)
		if err != nil {
			return 0, err
		}
		entries, err := l.Read()
		if err != nil {
			return 0, err
		}
		show(l, entries)
		return 0, nil
	}

	remove := sub("remove", "Unlink <package>; the override file goes when nothing is left.", "dashcrew link remove <package>")
	remove.flag("deps", "", "Run the platform's deps command afterwards.", true, true)
	remove.run = func(r *results) (int, error) {
		if len(r.rest) != 1 {
			return 0, &usageError{remove, "Pass <package>."}
		}
		l, err := open(r)
		if err != nil {
			return 0, err
		}
		entries, err := l.Remove(r.rest[0])
		if err != nil {
			return 0, err
		}
		show(l, entries)
		if r.flag("deps") {
			cli.out("deps: " + l.Deps())
		}
		return 0, nil
	}
	return branch
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
