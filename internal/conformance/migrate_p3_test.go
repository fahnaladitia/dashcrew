package conformance

import (
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/doctor"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/sync"
)

// sample-migrate-p3/p3/ is the output of the last pre-P4 dashcrew (three agents, plan gate on, storage and design
// packs), plus files the team wrote: a plan with handoffs, project memory and an AGENTS.md project block.
// It is frozen: never regenerate it. These are the assertions of test/golden/migrate_p3_test.dart.

var p3Roles = []string{
	"dart-tdd-guide", "design-refiner", "flutter-architect", "flutter-build-error-resolver", "flutter-code-reviewer",
	"flutter-engineer", "flutter-planner", "flutter-security-reviewer", "local-storage-engineer", "product-manager",
	"qa-engineer", "tech-lead", "worktree-monitor",
}

var p3RoleFiles = map[string]string{".claude/agents": ".md", ".codex/agents": ".toml", ".agents/agents": ".md"}

func teamFile(path string) bool {
	return strings.HasPrefix(path, ".ai/plans/") || strings.HasPrefix(path, ".ai/memory/") || path == ".ai/project.yaml"
}

type migrated struct {
	project string
	fresh   string
	before  map[string]string
	result  *sync.Result
	catalog *catalog.Catalog
}

func migrate(t *testing.T) migrated {
	t.Helper()
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	m := migrated{project: t.TempDir(), fresh: t.TempDir(), catalog: c}
	m.before = tree(t, filepath.Join(goldenDir(t), "sample-migrate-p3", "p3"))
	copyInto(t, m.before, m.project)
	if m.result, err = (sync.Sync{Catalog: c, ProjectDir: m.project}).Run(false, false); err != nil {
		t.Fatal(err)
	}
	copyInto(t, map[string]string{".ai/project.yaml": m.before[".ai/project.yaml"]}, m.fresh)
	if _, err := (sync.Sync{Catalog: c, ProjectDir: m.fresh}).Run(false, false); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestMigrateP3FixtureIsAP3Output(t *testing.T) {
	before := tree(t, filepath.Join(goldenDir(t), "sample-migrate-p3", "p3"))

	for dir, extension := range p3RoleFiles {
		for _, role := range p3Roles {
			if _, ok := before[dir+"/"+role+extension]; !ok {
				t.Errorf("fixture misses %s/%s%s", dir, role, extension)
			}
		}
	}
	for _, path := range []string{".claude/skills/orchestrate/SKILL.md", ".agents/skills/checkpoint/SKILL.md"} {
		if _, ok := before[path]; !ok {
			t.Errorf("fixture misses %s", path)
		}
	}
	if !strings.Contains(before["AGENTS.md"], "dashcrew gate submit plan") {
		t.Error("fixture AGENTS.md has no plan gate rule")
	}
}

func TestMigrateP3RemovesEveryOldRoleOrchestrateCheckpointAndThePlanGateRule(t *testing.T) {
	m := migrate(t)
	after := tree(t, m.project)

	for dir, extension := range p3RoleFiles {
		for _, role := range p3Roles {
			path := dir + "/" + role + extension
			if _, ok := after[path]; ok {
				t.Errorf("%s left", path)
			}
			if !slices.Contains(m.result.Removed, path) {
				t.Errorf("%s not reported as removed", path)
			}
		}
		var left []string
		for path := range after {
			if strings.HasPrefix(path, dir+"/") {
				left = append(left, path)
			}
		}
		sort.Strings(left)
		if want := []string{dir + "/explorer" + extension, dir + "/reviewer" + extension}; !reflect.DeepEqual(left, want) {
			t.Errorf("%s holds %v", dir, left)
		}
	}
	for _, skill := range []string{"orchestrate", "checkpoint"} {
		for _, dir := range []string{".claude/skills", ".agents/skills"} {
			if _, ok := after[dir+"/"+skill+"/SKILL.md"]; ok {
				t.Errorf("%s/%s left", dir, skill)
			}
		}
	}
	if strings.Contains(after["AGENTS.md"], "dashcrew gate") || strings.Contains(after["AGENTS.md"], "Plan gate") {
		t.Error("AGENTS.md keeps the plan gate")
	}
	if strings.Contains(after[".ai/guard.json"], "plan_gate") {
		t.Error("guard.json keeps plan_gate")
	}
}

func TestMigrateP3LeavesTheTeamFilesUntouched(t *testing.T) {
	m := migrate(t)
	after := tree(t, m.project)

	var team []string
	for path := range m.before {
		if teamFile(path) {
			team = append(team, path)
		}
	}
	for _, path := range []string{".ai/plans/todo-filter/handoff/00-roster.md", ".ai/memory/router-default-transition.md"} {
		if !slices.Contains(team, path) {
			t.Errorf("fixture misses %s", path)
		}
	}
	for _, path := range team {
		if after[path] != m.before[path] {
			t.Errorf("%s changed", path)
		}
	}
	block := func(agentsMd string) string {
		return agentsMd[strings.Index(agentsMd, sync.ProjectBegin) : strings.Index(agentsMd, sync.ProjectEnd)+len(sync.ProjectEnd)]
	}
	if block(after["AGENTS.md"]) != block(m.before["AGENTS.md"]) || !strings.Contains(block(after["AGENTS.md"]), "Every list screen has an empty state") {
		t.Error("AGENTS.md project block changed")
	}
}

func TestMigrateP3EqualsAFreshSyncBesidesTheTeamFiles(t *testing.T) {
	m := migrate(t)
	after := tree(t, m.project)
	expected := tree(t, m.fresh)
	for path := range after {
		if teamFile(path) {
			delete(after, path)
		}
	}
	for path := range expected {
		if teamFile(path) {
			delete(expected, path)
		}
	}

	if !reflect.DeepEqual(sortedKeys(after), sortedKeys(expected)) {
		t.Fatalf("file sets differ:\n%v\n%v", sortedKeys(after), sortedKeys(expected))
	}
	for _, path := range sortedKeys(expected) {
		if path == "AGENTS.md" {
			if sync.FileHash(path, after[path]) != sync.FileHash(path, expected[path]) {
				t.Error("AGENTS.md differs outside the project block")
			}
		} else if after[path] != expected[path] {
			t.Errorf("%s differs", path)
		}
	}
	left, err := dartjson.Decode(after[".ai/lock.json"])
	if err != nil {
		t.Fatal(err)
	}
	right, err := dartjson.Decode(expected[".ai/lock.json"])
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(left, right) {
		t.Error("locks differ")
	}
}

func TestMigrateP3PrintsTheP4MigrationNoteOnce(t *testing.T) {
	m := migrate(t)

	if !reflect.DeepEqual(m.result.Notes, []string{sync.P4MigrationNote}) {
		t.Errorf("notes %v", m.result.Notes)
	}
	if !slices.Contains(m.result.Warnings, profile.PlanGateRemoved) {
		t.Errorf("warnings %v", m.result.Warnings)
	}

	again, err := (sync.Sync{Catalog: m.catalog, ProjectDir: m.project}).Run(false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Notes) != 0 || len(again.Written) != 0 || len(again.Removed) != 0 {
		t.Errorf("second sync: notes %v, written %v, removed %v", again.Notes, again.Written, again.Removed)
	}
}

func TestMigrateP3DoctorReportsTheLeftoverRosterAndThePlanGateKeyAndNothingFails(t *testing.T) {
	m := migrate(t)

	checks := map[string]doctor.Check{}
	for _, check := range (doctor.Doctor{Catalog: m.catalog, ProjectDir: m.project}).Run() {
		checks[check.ID] = check
	}

	if checks["profile"].Status != doctor.Warn || !strings.Contains(checks["profile"].Detail, profile.PlanGateRemoved) {
		t.Errorf("profile %v", checks["profile"])
	}
	if checks["handoff-rosters"].Status != doctor.Info || !strings.Contains(checks["handoff-rosters"].Detail, ".ai/plans/todo-filter/handoff/00-roster.md") {
		t.Errorf("handoff-rosters %v", checks["handoff-rosters"])
	}
	for _, check := range checks {
		if check.Status == doctor.Fail {
			t.Errorf("%s: %s", check.ID, check.Detail)
		}
	}
}
