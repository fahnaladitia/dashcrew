package guard

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// Env is what Run needs from the process: environment lookup and the working directory.
type Env struct {
	LookupEnv func(string) (string, bool)
	Getwd     func() (string, error)
}

// ProcessEnv is the running process's environment and working directory.
var ProcessEnv = Env{LookupEnv: os.LookupEnv, Getwd: os.Getwd}

// Run is `dashcrew guard`: hook JSON on stdin, the answer per HookResponse, the exit code returned.
// Policy path: first argument, else `DASHCREW_GUARD_POLICY`, else `<hook cwd or first workspace path>/.ai/guard.json`.
// Any read or parse failure allows the call (exit 0, no output); `dashcrew doctor` reports a broken install.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, env Env) int {
	input, err := readJSON(stdin)
	if err != nil {
		return 0
	}
	object, ok := input.(*omap.Map[any])
	if !ok {
		return 0
	}
	policyPath, err := policyPath(args, object, env)
	if err != nil {
		return 0
	}
	if info, err := os.Stat(policyPath); err != nil || info.IsDir() {
		return 0
	}
	file, err := os.Open(policyPath)
	if err != nil {
		return 0
	}
	value, err := readJSON(file)
	file.Close()
	if err != nil {
		return 0
	}
	policy, err := LoadPolicy(value)
	if err != nil {
		return 0
	}
	events, err := AllFromHook(object)
	if err != nil {
		return 0
	}
	decision, err := EvaluateAll(events, policy)
	if err != nil {
		return 0
	}
	response := HookResponse(object, decision)
	if response.Stdout != "" {
		fmt.Fprintln(stdout, response.Stdout)
	}
	if response.Stderr != "" {
		fmt.Fprintln(stderr, response.Stderr)
	}
	return response.ExitCode
}

// readJSON decodes UTF-8 JSON as Dart's strict decoder does: malformed UTF-8 is an error, a leading byte order
// mark is skipped.
func readJSON(r io.Reader) (any, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("input is not UTF-8")
	}
	return dartjson.Decode(strings.TrimPrefix(string(data), "\ufeff"))
}

func policyPath(args []string, input *omap.Map[any], env Env) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	if path, ok := env.LookupEnv("DASHCREW_GUARD_POLICY"); ok {
		return path, nil
	}
	cwd, err := optionalString("cwd", input.Value("cwd"))
	if err != nil {
		return "", err
	}
	if cwd == nil {
		if paths, ok := input.Value("workspacePaths").([]any); ok && len(paths) > 0 {
			if first, ok := paths[0].(string); ok {
				cwd = &first
			}
		}
	}
	if cwd == nil {
		dir, err := env.Getwd()
		if err != nil {
			return "", err
		}
		cwd = &dir
	}
	return *cwd + "/.ai/guard.json", nil
}
