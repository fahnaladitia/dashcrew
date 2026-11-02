package guard

import (
	"reflect"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/catalog"
	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/profile"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

func compiled(t *testing.T, yaml string) Policy {
	t.Helper()
	c, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatal(err)
	}
	p, err := profile.Parse(yaml, c.KnownOptions())
	if err != nil {
		t.Fatal(err)
	}
	return CompilePolicy(p, c.ResolvedFor(p.Platform))
}

const goOther = "toolkit: 1\nagents: [claude-code]\nplatform: other\nplatform_config:\n  detect: [go.mod]\n" +
	"  commands: { analyze: go vet ./..., test: go test ./..., test_scoped: go test -run, deps: go mod download }\n"

func TestBackendRulesRoundTripThroughGuardJSON(t *testing.T) {
	policy := compiled(t, goOther+"  guard:\n    commands: [[terraform, apply]]\n"+
		"    sql: { clients: [psql, mysql], statements: [DROP] }\n    migrations: { tools: [goose], local_hosts: [localhost, db] }\n")
	if !policy.HasBackendRules() || policy.Minimum() != version.BackendGuardMinimum {
		t.Fatalf("minimum %s", policy.Minimum())
	}
	text, err := dartjson.EncodeIndent(policy.ToJSON(), "  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"block_commands"`, `"block_sql"`, `"block_migrations"`} {
		if !strings.Contains(text, key) {
			t.Errorf("guard.json lacks %s:\n%s", key, text)
		}
	}
	value, err := dartjson.Decode(text)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPolicy(value)
	if err != nil {
		t.Fatal(err)
	}
	read, err := PolicyFromJSON(text)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []Policy{loaded, read} {
		if !reflect.DeepEqual(p.BlockCommands, policy.BlockCommands) || !reflect.DeepEqual(p.BlockSQL, policy.BlockSQL) ||
			!reflect.DeepEqual(p.BlockMigrations, policy.BlockMigrations) {
			t.Errorf("read back %+v %+v %+v", p.BlockCommands, p.BlockSQL, p.BlockMigrations)
		}
	}
}

func TestNoBackendRulesKeepThePolicyAndTheMinimum(t *testing.T) {
	for _, yaml := range []string{"toolkit: 1\nagents: [claude-code]\n", goOther} {
		policy := compiled(t, yaml)
		text, _ := dartjson.EncodeIndent(policy.ToJSON(), "  ")
		if policy.HasBackendRules() || policy.Minimum() != version.GuardMinimum || strings.Contains(text, "block_commands") ||
			strings.Contains(text, "block_sql") || strings.Contains(text, "block_migrations") {
			t.Errorf("%q: minimum %s\n%s", yaml, policy.Minimum(), text)
		}
	}
}

func TestBrokenBackendRulesLetTheCallThrough(t *testing.T) {
	base := `{"schema":1,"blocked_git":["push"],"block_formatters":true,"plans_dir":"p","role_capability":{},"forbidden_imports":[],"local_packages":[],`
	for _, broken := range []string{`"block_commands":"terraform"}`, `"block_commands":[[1]]}`, `"block_sql":{"clients":"psql","statements":[]}}`, `"block_migrations":[]}`} {
		value, err := dartjson.Decode(base + broken)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := LoadPolicy(value); err == nil {
			t.Errorf("%s loaded", broken)
		}
	}
}
