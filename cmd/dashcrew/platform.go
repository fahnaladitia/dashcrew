package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/starter"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

func platformCommand(cli cliIO) *command {
	branch := newCommand("platform", "Platform starters: write a platform into .ai/platforms/<name>/ for the team to edit. Writes once.")
	create := newCommand("new", "Write the <name> starter (go) into .ai/platforms/<name>/, and .ai/project.yaml when there is none.\n"+
		"The folder is the team's afterwards; dashcrew never overwrites it.")
	create.invocation = "dashcrew platform new <name> [arguments]"
	create.option("project", "p", "Project root.", ptr("."))
	create.option("http", "", "HTTP pack: nethttp, chi, gin, echo, fiber or none; default from go.mod.", nil)
	create.option("db", "", "Database pack: pgx, sqlc, gorm or none; default from go.mod and sqlc.yaml.", nil)
	create.option("agents", "", "Agents for a new .ai/project.yaml, comma-separated.", ptr("claude-code"))
	create.run = func(r *results) (int, error) {
		if len(r.rest) != 1 {
			return 0, &usageError{create, "Pass one starter name: " + strings.Join(starter.Names(embedded.StartersFS), ", ") + "."}
		}
		name := r.rest[0]
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		stack := map[string]string{}
		if name == "go" {
			http, db := starter.GoDefaults(projectDir)
			stack["http"], stack["db"] = http, db
		}
		for _, family := range []string{"http", "db"} {
			value := r.option(family)
			if value == nil {
				continue
			}
			options := starter.Options(embedded.StartersFS, name, family)
			if !hasOption(options, *value) {
				return 0, &usageError{create, "--" + family + " must be one of " + strings.Join(options, ", ") + "."}
			}
			stack[family] = *value
		}
		written, err := starter.Write(embedded.StartersFS, name, projectDir, version.Current())
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		cli.out("wrote " + filepath.Join(".ai", "platforms", name) + "/ (" + fileCount(len(written)) + ")")
		profilePath := filepath.Join(projectDir, ".ai", "project.yaml")
		lines := "platform: " + name + "\nstack:\n  http: " + stack["http"] + "\n  db: " + stack["db"] + "\n"
		if _, err := os.Stat(profilePath); err == nil {
			cli.out(".ai/project.yaml exists and is left as is; set these keys in it, then run dashcrew sync:")
			for _, line := range strings.Split(strings.TrimSuffix(lines, "\n"), "\n") {
				cli.out("  " + line)
			}
			return 0, nil
		}
		agents := strings.Join(strings.Split(*r.option("agents"), ","), ", ")
		text := "toolkit: 1\n" + lines + "agents: [" + agents + "]\n"
		c, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		if c, err = c.WithProjectPacks(projectDir); err != nil {
			return 0, &exitError{1, "the starter does not load: " + err.Error()}
		}
		if _, err := profile.Parse(text, c.KnownOptions()); err != nil {
			return 0, &exitError{1, err.Error()}
		}
		if err := os.WriteFile(profilePath, []byte(text), 0o644); err != nil {
			return 0, err
		}
		cli.out("wrote .ai/project.yaml (platform " + name + ", http " + stack["http"] + ", db " + stack["db"] + "); run dashcrew sync")
		return 0, nil
	}
	return branch.add(create)
}

func fileCount(n int) string {
	if n == 1 {
		return "1 file"
	}
	return strconv.Itoa(n) + " files"
}

func hasOption(options []string, value string) bool {
	for _, option := range options {
		if option == value {
			return true
		}
	}
	return false
}
