package doctor

import (
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/agents"
)

// TestEveryAgentHasItsChecks fails when an agent joins internal/agents without its row in agentChecks.
func TestEveryAgentHasItsChecks(t *testing.T) {
	for _, agent := range agents.All() {
		a, ok := agentChecks[agent]
		switch {
		case !ok:
			t.Errorf("%s: no agentChecks row", agent)
		case a.hook == nil || a.machine == nil || a.alwaysOn == nil || a.agentsMdLimit <= 0:
			t.Errorf("%s: incomplete row %+v", agent, a)
		}
	}
	for agent := range agentChecks {
		if !contains(agents.All(), agent) {
			t.Errorf("%s: a row for an agent internal/agents does not list", agent)
		}
	}
}

func TestAgentsMdLimitIsTheSmallestOfTheSelectedAgents(t *testing.T) {
	for _, c := range []struct {
		selected map[string]bool
		want     int
	}{
		{map[string]bool{"claude-code": true}, AgentsMdLimit},
		{map[string]bool{"claude-code": true, "codex": true}, AgentsMdLimit},
		{map[string]bool{"codex": true, "antigravity": true}, 24000},
		{map[string]bool{}, AgentsMdLimit},
	} {
		if got := AgentsMdLimitFor(c.selected); got != c.want {
			t.Errorf("%v: %d, want %d", c.selected, got, c.want)
		}
	}
}
