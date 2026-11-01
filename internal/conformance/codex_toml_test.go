package conformance

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// codexTomlScript parses the Codex TOML files of a sample with Python's tomllib, so escaping is checked by a real
// TOML parser, and compares each role's developer_instructions with the Claude Code body of the same role.
const codexTomlScript = `
import glob, os, sys, tomllib
root = sys.argv[1]
problems = []
for path in sorted(glob.glob(os.path.join(root, '.codex', 'agents', '*.toml'))):
    name = os.path.basename(path)[:-5]
    with open(path, 'rb') as f:
        data = tomllib.load(f)
    if data.get('name') != name or not data.get('description', '').strip():
        problems.append(f'{name}: name or description')
    claude = open(os.path.join(root, '.claude', 'agents', name + '.md')).read().split('\n---\n', 1)[1].lstrip('\n')
    if data.get('developer_instructions') != claude:
        problems.append(f'{name}: developer_instructions differ from the claude code body')
with open(os.path.join(root, '.codex', 'config.toml'), 'rb') as f:
    if 'dart-mcp-server' not in tomllib.load(f).get('mcp_servers', {}):
        problems.append('config.toml: dart-mcp-server missing')
print('\n'.join(problems))
sys.exit(1 if problems else 0)
`

// tomlPython is the first Python on PATH with tomllib (3.11+), or "" when there is none.
func tomlPython() string {
	for _, candidate := range []string{"python3.14", "python3.13", "python3.12", "python3.11", "python3"} {
		if exec.Command(candidate, "-c", "import tomllib").Run() == nil {
			return candidate
		}
	}
	return ""
}

func TestCodexRoleAndConfigFilesParseAsTomlAndKeepTheRoleBodies(t *testing.T) {
	python := tomlPython()
	if python == "" {
		t.Skip("no Python 3.11+ with tomllib on PATH")
	}
	expected := filepath.Join(goldenDir(t), "sample-codex", "expected")
	out, err := exec.Command(python, "-I", "-c", codexTomlScript, expected).CombinedOutput()
	if err != nil {
		t.Errorf("%v\n%s", err, out)
	}
}
