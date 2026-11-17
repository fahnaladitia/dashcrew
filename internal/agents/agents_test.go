package agents

import (
	"slices"
	"testing"
)

func TestAllListsEachAgentOnceInMessageOrder(t *testing.T) {
	want := []string{"claude-code", "codex", "antigravity"}
	if got := All(); !slices.Equal(got, want) {
		t.Errorf("All() = %v, want %v", got, want)
	}
}

func TestPersonalAgentsAreAgentsInTheSameOrder(t *testing.T) {
	all := All()
	last := -1
	for _, name := range Personal() {
		at := slices.Index(all, name)
		if at <= last {
			t.Errorf("%s is not an agent, or out of order in %v", name, Personal())
		}
		last = at
	}
}

func TestTheListsAreCopies(t *testing.T) {
	All()[0] = "changed"
	Personal()[0] = "changed"
	if All()[0] != "claude-code" || Personal()[0] != "claude-code" {
		t.Error("a caller changed the agent list")
	}
}
