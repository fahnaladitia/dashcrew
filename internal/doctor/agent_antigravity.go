package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

func (d Doctor) antigravityHook() Check {
	const id = "antigravity-hook"
	if !d.exists(".agents/hooks.json") {
		return Check{id, Fail, ".agents/hooks.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".agents/hooks.json")
	if err != nil {
		return Check{id, Fail, ".agents/hooks.json invalid: " + err.Error()}
	}
	entry := config.Value(adapters.AntigravityGuardKey)
	if encode(entry) == encode(adapters.AntigravityGuardHook()) {
		present := "PreToolUse guard hook present under " + adapters.AntigravityGuardKey
		if !d.MachineChecks {
			return Check{id, OK, present}
		}
		switch loaded, ok := d.agyLoadsGuard(); {
		case !ok:
			return Check{id, OK, present}
		case !loaded:
			return Check{id, Warn, present + ", but agy does not load it here: trust this folder in agy, since workspace hooks load only in a trusted folder"}
		}
		return Check{id, OK, present + "; agy loads it"}
	}
	problem := "guard hook differs from what sync writes"
	if entry == nil {
		problem = "guard hook missing"
	}
	return Check{id, Fail, problem + "; run dashcrew sync"}
}

// agyLoadsGuard asks agy for its active hooks in the project (`agy -p /hooks`, one tab-separated record per line).
// loaded needs a record whose first field is the guard key and that says nothing of being skipped; a failing agy
// that names trust means not loaded. ok is false when agy is not on PATH, does not answer within 15 seconds or fails
// for another reason, signed out for one; the file check then stands.
func (d Doctor) agyLoadsGuard() (loaded, ok bool) {
	pathEnv := d.searchPath()
	agy, found := lookPath("agy", pathEnv)
	if !found {
		return false, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, agy, "-p", "/hooks")
	cmd.Dir = d.ProjectDir
	cmd.Env = append(os.Environ(), "PATH="+pathEnv)
	// A child that keeps the pipes open would hold Output past the timeout; WaitDelay closes them.
	cmd.WaitDelay = 2 * time.Second
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if strings.Contains(strings.ToLower(string(out)+stderr.String()), "trust") {
			return false, true
		}
		return false, false
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.FieldsFunc(line, func(r rune) bool { return r == '\t' || r == ' ' })
		lower := strings.ToLower(line)
		if len(fields) > 0 && fields[0] == adapters.AntigravityGuardKey &&
			!strings.Contains(lower, "skip") && !strings.Contains(lower, "disabled") && !strings.Contains(lower, "trust") {
			return true, true
		}
	}
	return false, true
}

func (d Doctor) antigravityMcp() Check {
	const id = "antigravity-mcp"
	if !d.exists(".agents/mcp_config.json") {
		return Check{id, Warn, ".agents/mcp_config.json missing; run dashcrew sync"}
	}
	config, err := d.readObject(".agents/mcp_config.json")
	var servers *omap.Map[any]
	if err == nil {
		servers, err = mcpServers(config)
	}
	if err != nil {
		return Check{id, Warn, ".agents/mcp_config.json invalid: " + err.Error()}
	}
	adapter := adapters.NewAntigravity(d.Catalog, d.ProjectDir, d.platform())
	adapter.Knowledge = d.profileKnowledge()
	expected := adapter.McpServers().Keys()
	var missing []string
	for _, name := range expected {
		if !servers.Has(name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return Check{id, Warn, "missing: " + strings.Join(missing, ", ") + "; run dashcrew sync"}
	}
	return Check{id, OK, definedDetail(expected)}
}

// antigravityRoles fails a role whose `tools` names a tool Antigravity does not know: the subagent may hang.
func (d Doctor) antigravityRoles() Check {
	const id = "antigravity-roles"
	dir := d.path(".agents/agents")
	if !isDir(dir) {
		return Check{id, Fail, ".agents/agents missing; run dashcrew sync"}
	}
	entries, _ := os.ReadDir(dir)
	var unknown []string
	count := 0
	for _, entry := range entries {
		file := filepath.Join(dir, entry.Name())
		if !strings.HasSuffix(entry.Name(), ".md") || !isFile(file) {
			continue
		}
		count++
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		tools := ""
		if match := toolsLine.FindStringSubmatch(string(data)); match != nil {
			tools = match[1]
		}
		for _, tool := range strings.Split(tools, ",") {
			tool = strings.TrimSpace(tool)
			if tool != "" && !contains(adapters.AntigravityKnownTools, tool) {
				unknown = append(unknown, strings.TrimSuffix(entry.Name(), ".md")+": "+tool)
			}
		}
	}
	if len(unknown) > 0 {
		return Check{id, Fail, "unknown tool names (the subagent may hang): " + strings.Join(unknown, ", ")}
	}
	return Check{id, OK, strconv.Itoa(count) + " roles, every tool name known"}
}

// antigravityAlwaysOn adds AGENTS.md and the `.agents/rules` files whose trigger is `always_on`.
func antigravityAlwaysOn(d Doctor, add func(relative string) bool) []string {
	add("AGENTS.md")
	for _, rule := range d.ruleFiles(".agents/rules", false) {
		if trigger, _ := frontMatterValue(d.readOrEmpty(rule), "trigger"); trigger == "always_on" {
			add(rule)
		}
	}
	return nil
}
