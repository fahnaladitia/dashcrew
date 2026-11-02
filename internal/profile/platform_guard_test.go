package profile

import (
	"reflect"
	"testing"
)

const otherBase = "toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config:\n  detect: [go.mod]\n  commands: { analyze: a, test: b, test_scoped: c, deps: d }\n"

func TestPlatformGuardIsReadFromPlatformConfig(t *testing.T) {
	p := parse(t, otherBase+"  guard:\n"+
		"    commands: [[terraform, apply], [kubectl, delete]]\n"+
		"    sql: { clients: [psql], statements: [drop, Truncate] }\n"+
		"    migrations: { tools: [goose] }\n")
	g := p.PlatformConfig.Guard
	want := &PlatformGuard{
		Commands:       [][]string{{"terraform", "apply"}, {"kubectl", "delete"}},
		SQLClients:     []string{"psql"},
		SQLStatements:  []string{"DROP", "TRUNCATE"},
		MigrationTools: []string{"goose"},
		LocalHosts:     []string{"localhost", "127.0.0.1", "::1"},
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("guard %#v", g)
	}
	if g.Empty() {
		t.Error("a guard with rules is empty")
	}
	if p := parse(t, otherBase); p.PlatformConfig.Guard != nil || !p.PlatformConfig.Guard.Empty() {
		t.Error("no guard key should be nil and empty")
	}
}

func TestPlatformGuardErrors(t *testing.T) {
	cases := []struct{ name, guard, message string }{
		{"not a map", "  guard: [a]\n", "project.yaml platform_config.guard: expected a map with commands, sql and migrations"},
		{"unknown key", "  guard: { deploy: [a] }\n", "project.yaml platform_config.guard.deploy: unknown key"},
		{"command not a list", "  guard: { commands: [terraform] }\n", "project.yaml platform_config.guard.commands[0]: expected [program, subcommand...], plain words"},
		{"command with a space", "  guard: { commands: [['terraform apply']] }\n", "project.yaml platform_config.guard.commands[0]: expected [program, subcommand...], plain words"},
		{"sql without statements", "  guard: { sql: { clients: [psql] } }\n", "project.yaml platform_config.guard.sql.statements: required, a non-empty list"},
		{"sql unknown key", "  guard: { sql: { clients: [psql], statements: [DROP], mode: x } }\n", "project.yaml platform_config.guard.sql.mode: unknown key"},
		{"migrations without tools", "  guard: { migrations: { local_hosts: [db] } }\n", "project.yaml platform_config.guard.migrations.tools: required, a non-empty list"},
		{"bad host", "  guard: { migrations: { tools: [goose], local_hosts: ['a b'] } }\n", "project.yaml platform_config.guard.migrations.local_hosts: expected a non-empty list of plain words"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseError(t, otherBase+c.guard).Error(); got != c.message {
				t.Errorf("got %q\nwant %q", got, c.message)
			}
		})
	}
}
