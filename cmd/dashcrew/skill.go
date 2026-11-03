package main

import (
	"path/filepath"

	"github.com/fahnaladitia/dashcrew/internal/me"
	"github.com/fahnaladitia/dashcrew/internal/projectskill"
)

func skillCommand(cli cliIO) *command {
	branch := newCommand("skill", "Project skills in .ai/skills/: adopt one from your own skills or a local folder. Never commits.")
	adopt := newCommand("adopt", "Copy a skill into .ai/skills/<name>/ and write its meta.yaml from the SKILL.md front matter.\n"+
		"Optional by default: only members who list it in .ai/me.yaml skills get it.")
	adopt.invocation = "dashcrew skill adopt <name | folder>"
	adopt.option("project", "p", "Project root holding .ai/project.yaml.", ptr("."))
	adopt.flag("required", "", "On for every member and listed in the AGENTS.md skill index.", false, false)
	adopt.run = func(r *results) (int, error) {
		if len(r.rest) != 1 {
			return 0, &usageError{adopt, "Pass the name of a skill in ~/.dashcrew/skills, or a folder holding a SKILL.md."}
		}
		places, err := me.PlacesFrom(cli.env)
		if err != nil {
			return 0, err
		}
		required := r.flag("required")
		project, err := filepath.Abs(*r.option("project"))
		if err != nil {
			return 0, err
		}
		name, skipped, err := projectskill.Adopt(project, places.Home, r.rest[0], required)
		if err != nil {
			return 1, &exitError{1, err.Error()}
		}
		cli.out("wrote .ai/skills/" + name + "/meta.yaml and .ai/skills/" + name + "/SKILL.md")
		for _, file := range skipped {
			cli.out("left out  " + file)
		}
		if required {
			cli.out("next: dashcrew sync, then commit .ai/skills/" + name + " and the files sync writes")
		} else {
			cli.out("next: dashcrew sync, commit .ai/skills/" + name + "; members turn it on with skills: [" + name + "] in .ai/me.yaml, then dashcrew me sync")
		}
		cli.out("then remove your own copy from ~/.dashcrew/skills and me.yaml skills, if it came from there")
		return 0, nil
	}
	branch.add(adopt)
	return branch
}
