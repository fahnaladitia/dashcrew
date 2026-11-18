package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	embedded "github.com/fahnaladitia/dashcrew"
	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/eject"
	"github.com/fahnaladitia/dashcrew/internal/initproject"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

func platformCommand(cli cliIO) *command {
	branch := newCommand("platform", "Built-in platforms: eject one into .ai/platforms/<name>/ for the team to own. Writes once.")
	create := newCommand("new", "Copy built-in platform <name> (flutter, go) into .ai/platforms/<name>/, where it replaces the built-in one,\n"+
		"and write .ai/project.yaml when there is none. The folder is the team's afterwards: dashcrew never overwrites it,\n"+
		"so it no longer gets the built-in platform's updates.")
	create.invocation = "dashcrew platform new <name> [arguments]"
	create.option("project", "p", "Project root.", ptr("."))
	create.option("http", "", "go only. HTTP pack: nethttp, chi, gin, echo, fiber or none; default from go.mod.", nil)
	create.option("db", "", "go only. Database pack: pgx, sqlc, gorm or none; default from go.mod and sqlc.yaml.", nil)
	create.option("agents", "", "Agents for a new .ai/project.yaml, comma-separated.", ptr("claude-code"))
	create.run = func(r *results) (int, error) {
		names := eject.Names(embedded.CatalogFS)
		if len(r.rest) != 1 {
			return 0, &usageError{create, "Pass one built-in platform name: " + strings.Join(names, ", ") + "."}
		}
		name := r.rest[0]
		projectDir, err := absolute(*r.option("project"))
		if err != nil {
			return 0, err
		}
		var families []string
		stack := map[string]string{}
		if name == "go" {
			families = []string{"http", "db"}
			stack["http"], stack["db"] = initproject.GoDefaults(projectDir)
		}
		for _, family := range []string{"http", "db"} {
			value := r.option(family)
			if value == nil {
				continue
			}
			options := eject.Options(embedded.CatalogFS, name, family)
			if options == nil {
				return 0, &usageError{create, "--" + family + " is for the go platform only."}
			}
			if !hasOption(options, *value) {
				return 0, &usageError{create, "--" + family + " must be one of " + strings.Join(options, ", ") + "."}
			}
			stack[family] = *value
		}
		written, err := eject.Write(embedded.CatalogFS, name, projectDir, version.Current())
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		cli.out("wrote " + filepath.Join(".ai", "platforms", name) + "/ (" + fileCount(len(written)) + "); it replaces the built-in " + name + " platform")
		profilePath := filepath.Join(projectDir, ".ai", "project.yaml")
		if _, err := os.Stat(profilePath); err == nil {
			cli.out(".ai/project.yaml exists and is left as is; set these keys in it, then run dashcrew sync:")
			cli.out("  platform: " + name)
			if len(families) > 0 {
				cli.out("  stack:")
				for _, family := range families {
					cli.out("    " + family + ": " + stack[family])
				}
			}
			return 0, nil
		}
		c, err := catalog.LoadEmbedded()
		if err != nil {
			return 0, err
		}
		text, err := initproject.PlatformProfile(c, projectDir, name, families, stack, strings.Split(*r.option("agents"), ","))
		if err != nil {
			return 0, &exitError{1, err.Error()}
		}
		if err := os.WriteFile(profilePath, []byte(text), 0o644); err != nil {
			return 0, err
		}
		summary := "platform " + name
		for _, family := range families {
			summary += ", " + family + " " + stack[family]
		}
		cli.out("wrote .ai/project.yaml (" + summary + "); run dashcrew sync")
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
