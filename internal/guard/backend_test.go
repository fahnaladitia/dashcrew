package guard

import (
	"strings"
	"testing"
)

func backendPolicy() Policy {
	return Policy{
		BlockedGit:    map[string]bool{"push": true},
		BlockCommands: [][]string{{"terraform", "apply"}, {"terraform", "destroy"}, {"kubectl", "apply"}, {"helm", "upgrade"}, {"gcloud", "run", "deploy"}},
		BlockSQL:      SQLBlock{Clients: []string{"psql", "mysql", "sqlite3"}, Statements: []string{"DROP", "TRUNCATE"}},
		BlockMigrations: MigrationBlock{Tools: []string{"goose", "migrate", "atlas"},
			LocalHosts: []string{"localhost", "127.0.0.1", "::1"}},
	}
}

func TestBackendRules(t *testing.T) {
	cases := []struct {
		script  string
		blocked string // a word of the reason; empty when allowed
	}{
		{"terraform apply", "terraform apply"},
		{"terraform apply -auto-approve", "terraform apply"},
		{"terraform plan", ""},
		{"/usr/local/bin/terraform destroy", "terraform destroy"},
		{"TF_LOG=1 sudo terraform apply", "terraform apply"},
		{`bash -c "cd infra && terraform apply"`, "terraform apply"},
		{"kubectl -n prod apply -f deploy.yaml", "kubectl apply"},
		{"kubectl get pods", ""},
		{"helm upgrade api ./chart", "helm upgrade"},
		{"gcloud --project x run deploy api", "gcloud run deploy"},
		{"gcloud run services list", ""},

		{`psql -c "DROP TABLE users"`, "DROP"},
		{`psql -c "select 1"`, ""},
		{`psql -c "select dropped_at from t"`, ""},
		{"psql app <<EOF\ndrop table x;\nEOF", "DROP"},
		{`echo "TRUNCATE orders" | psql app`, "TRUNCATE"},
		{`mysql -e 'drop database app'`, "DROP"},
		{`psql <<< "DROP TABLE x"`, "DROP"},
		{`bash -c "psql -c 'TRUNCATE t'"`, "TRUNCATE"},
		{"psql -f cleanup.sql", ""},
		{"echo DROP", ""},

		{`goose postgres "postgres://app:pw@db.prod.example.com:5432/app" up`, "db.prod.example.com"},
		{`goose postgres "postgres://app:pw@localhost:5432/app" up`, ""},
		{"migrate -database postgres://app@127.0.0.1/app up", ""},
		{"migrate -database 'postgres://app@[::1]:5432/app' up", ""},
		{"GOOSE_DBSTRING=postgres://app@db.prod/app goose up", "db.prod"},
		{`goose postgres "host=db.prod user=app dbname=app" up`, "db.prod"},
		{`goose -dir db postgres "host=/var/run/postgresql dbname=app" up`, ""},
		{"atlas migrate apply --url mysql://root:pw@db.prod:3306/app", "db.prod"},
		{`goose mysql "root:pw@tcp(db.prod:3306)/app" up`, "db.prod"},
		{`goose mysql "root:pw@tcp(localhost:3306)/app" up`, ""},
		{"goose sqlite3 ./app.db up", ""},
		{"goose up", ""},
		{`sh -c 'migrate -database postgres://u@db.prod/app down'`, "db.prod"},
	}
	p := backendPolicy()
	for _, c := range cases {
		decision, err := Evaluate(Event{Kind: Shell, Command: c.script}, p)
		if err != nil {
			t.Fatalf("%q: %v", c.script, err)
		}
		switch {
		case c.blocked == "" && decision.Blocked:
			t.Errorf("%q blocked: %s", c.script, decision.Reason)
		case c.blocked != "" && (!decision.Blocked || !strings.Contains(decision.Reason, c.blocked)):
			t.Errorf("%q: blocked %v %q, want a reason with %q", c.script, decision.Blocked, decision.Reason, c.blocked)
		}
	}
}

func TestBackendRulesOnAnArgvList(t *testing.T) {
	p := backendPolicy()
	for argv, blocked := range map[string]bool{
		"goose|postgres|postgres://x@prod.internal/db|up": true,
		"terraform|apply":                             true,
		"psql|-c|DROP TABLE t":                        true,
		"goose|postgres|postgres://x@localhost/db|up": false,
	} {
		decision, _ := Evaluate(Event{Kind: Shell, Argv: strings.Split(argv, "|")}, p)
		if decision.Blocked != blocked {
			t.Errorf("%s: blocked %v %q", argv, decision.Blocked, decision.Reason)
		}
	}
}

func TestWithoutBackendRulesNothingChanges(t *testing.T) {
	p := Policy{BlockedGit: map[string]bool{"push": true}}
	for _, script := range []string{"terraform apply", `psql -c "DROP TABLE x"`, "goose postgres postgres://x@prod/db up"} {
		if decision, _ := Evaluate(Event{Kind: Shell, Command: script}, p); decision.Blocked {
			t.Errorf("%q blocked without backend rules: %s", script, decision.Reason)
		}
	}
}
