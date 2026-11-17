// Package agents is the one list of the agents dashcrew writes files for. Profile validation, the adapters,
// me sync and doctor read it, so a new agent starts here.
package agents

type agent struct {
	name string
	// personal is true when me sync writes the member's personal instructions for it.
	personal bool
}

var table = []agent{
	{"claude-code", true},
	{"codex", true},
	{"antigravity", true},
}

// All are the `agents` values of `.ai/project.yaml`, in the order messages list them.
func All() []string {
	out := make([]string, 0, len(table))
	for _, a := range table {
		out = append(out, a.name)
	}
	return out
}

// Personal are the `agents` values of `me.yaml`, the agents me sync writes personal instructions for, in the
// order of All.
func Personal() []string {
	var out []string
	for _, a := range table {
		if a.personal {
			out = append(out, a.name)
		}
	}
	return out
}
