package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/commit"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// commitProfile is the profile in dir; a folder without .ai/project.yaml gets the defaults of an empty profile.
func commitProfile(dir string) (*profile.Profile, error) {
	cat, err := catalog.LoadEmbedded()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, ".ai", "project.yaml")); errors.Is(err, os.ErrNotExist) {
		return profile.Parse("toolkit: 1\nagents: [claude-code]\n", cat.KnownOptions())
	}
	p, err := sync.Sync{Catalog: cat, ProjectDir: dir}.Profile()
	if err != nil {
		return nil, &exitError{1, err.Error()}
	}
	return p, nil
}

// openCommitRepo is the repository holding --project and the profile at its top level.
func openCommitRepo(r *results) (commit.Repo, *profile.Profile, error) {
	repo, err := commit.Open(*r.option("project"))
	if err != nil {
		return commit.Repo{}, nil, &exitError{1, err.Error()}
	}
	p, err := commitProfile(repo.Dir)
	return repo, p, err
}

func commitCommand(cli cliIO) *command {
	branch := newCommand("commit", "Commit messages in the team's pattern (project.commit): a ready command, and a lint. Never commits.")

	suggest := newCommand("suggest", "Print the git add and git commit commands for the current changes, in the pattern.\n"+
		"Type, scope and ticket come from the branch and the changed files; untracked files are listed, never added.\n"+
		"Reads git only; with --subject the message is linted and a problem exits 1.")
	suggest.option("project", "p", "Any folder inside the repository.", ptr("."))
	suggest.option("subject", "", "The subject; without it the command keeps a <subject> placeholder.", nil)
	suggest.option("body", "", "The body; without it and without --subject the command keeps a <body> placeholder.", nil)
	suggest.option("type", "", "Override the type derived from the branch.", nil)
	suggest.option("scope", "", "Override the scope derived from the files; an empty value drops it.", nil)
	suggest.flag("json", "", "Print the derived parts and commands as JSON.", false, false)
	suggest.run = func(r *results) (int, error) {
		repo, p, err := openCommitRepo(r)
		if err != nil {
			return 0, err
		}
		cwd, err := os.Getwd()
		if err != nil {
			return 0, err
		}
		if real, err := filepath.EvalSymlinks(cwd); err == nil {
			cwd = real
		}
		relative := func(path string) string {
			rel, err := filepath.Rel(cwd, filepath.Join(repo.Dir, filepath.FromSlash(path)))
			if err != nil {
				return path
			}
			return filepath.ToSlash(rel)
		}
		prefixes, _ := p.Project.Value("branch_prefix").(*omap.Map[any])
		in := commit.Input{Subject: r.option("subject"), Body: r.option("body"), Type: r.option("type"), Scope: r.option("scope")}
		s, err := commit.Suggest(repo, p.Commit, prefixes, in, relative)
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		if r.flag("json") {
			text, err := dartjson.EncodeIndent(s.JSON(), "  ")
			if err != nil {
				return 0, err
			}
			cli.out(text)
		} else {
			for _, line := range s.Lines() {
				cli.out(line)
			}
			for _, problem := range s.Problems {
				fmt.Fprintln(cli.stderr, "lint: "+problem)
			}
		}
		if len(s.Problems) > 0 || s.AddCommand == "" {
			return 1, nil
		}
		return 0, nil
	}

	lint := newCommand("lint", "Check commit messages against project.commit; exit 1 with one line per problem.\n"+
		"Checks the pattern, type, subject length, body, trailers and AI attribution of one message, or of every\n"+
		"commit of --range but merges.")
	lint.invocation = "dashcrew commit lint <message> | - | --range <base>..<head>"
	lint.option("project", "p", "Any folder inside the repository.", ptr("."))
	lint.option("range", "", "Lint the commits of <base>..<head> instead of a message.", nil)
	lint.run = func(r *results) (int, error) {
		revRange := r.option("range")
		if (revRange == nil) == (len(r.rest) != 1) {
			return 0, &usageError{lint, "Pass one message, - for stdin, or --range <base>..<head>."}
		}
		repo, p, err := openLintProfile(r, revRange != nil)
		if err != nil {
			return 0, err
		}
		var problems []string
		summary := ""
		if revRange != nil {
			var count int
			if problems, count, err = commit.LintRange(repo, p.Commit, *revRange); err != nil {
				return 0, &exitError{1, err.Error()}
			}
			summary = commit.Follow(count, *revRange, p.Commit.Pattern)
		} else {
			message := r.rest[0]
			if message == "-" {
				data, err := io.ReadAll(stdin)
				if err != nil {
					return 0, err
				}
				message = string(data)
			}
			problems = commit.Lint(p.Commit, message)
			summary = "the message follows " + p.Commit.Pattern
		}
		for _, problem := range problems {
			cli.out(problem)
		}
		if len(problems) > 0 {
			return 1, nil
		}
		cli.out(summary)
		return 0, nil
	}
	return branch.add(suggest).add(lint)
}

// openLintProfile is openCommitRepo, except that a message is also linted outside a repository, with the profile
// in --project.
func openLintProfile(r *results, needsRepo bool) (commit.Repo, *profile.Profile, error) {
	if _, err := commit.Open(*r.option("project")); err == nil || needsRepo {
		return openCommitRepo(r)
	}
	p, err := commitProfile(*r.option("project"))
	return commit.Repo{}, p, err
}

func verifyCommand(cli cliIO) *command {
	branch := newCommand("verify", "Helpers for the verify workflow.")
	record := newCommand("record", "Record the tree of the working copy as verified, for dashcrew commit suggest to compare.\n"+
		"Hashes it in a temporary index, so the real index and staging are never touched.")
	record.option("project", "p", "Any folder inside the repository.", ptr("."))
	record.run = func(r *results) (int, error) {
		repo, err := commit.Open(*r.option("project"))
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		tree, err := repo.RecordVerify()
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		cli.out("recorded verified tree " + tree + "; dashcrew commit suggest lists files changed after it")
		return 0, nil
	}
	return branch.add(record)
}
