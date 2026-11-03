package guard

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// Env is what Run needs from the process: environment lookup and the working directory.
type Env struct {
	LookupEnv func(string) (string, bool)
	Getwd     func() (string, error)
	// Now stamps the malformed log; nil means time.Now.
	Now func() time.Time
}

// ProcessEnv is the running process's environment and working directory.
var ProcessEnv = Env{LookupEnv: os.LookupEnv, Getwd: os.Getwd, Now: time.Now}

// Run is `dashcrew guard`: hook JSON on stdin, the answer per HookResponse, the exit code returned.
// Policy path: first argument, else `DASHCREW_GUARD_POLICY`, else `<hook cwd or first workspace path>/.ai/guard.json`.
// Any read or parse failure allows the call (exit 0; Antigravity still gets its allow decision); `dashcrew doctor`
// reports a broken install.
// A malformed hook input is also recorded in the malformed log (see logMalformed), which doctor reports.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer, env Env) int {
	object, decision := judge(args, stdin, env)
	response := HookResponse(object, decision)
	if response.Stdout != "" {
		fmt.Fprintln(stdout, response.Stdout)
	}
	if response.Stderr != "" {
		fmt.Fprintln(stderr, response.Stderr)
	}
	return response.ExitCode
}

// judge reads the hook input and the policy and evaluates the call; every failure allows it. object is nil when the
// input is not a JSON object, so HookResponse answers only once it knows which agent called.
func judge(args []string, stdin io.Reader, env Env) (*omap.Map[any], Decision) {
	input, err := readJSON(stdin)
	if err != nil {
		reason := "input is not JSON"
		if errors.Is(err, errNotUTF8) {
			reason = "input is not UTF-8"
		}
		logMalformed(env, nil, reason)
		return nil, Allow()
	}
	object, ok := input.(*omap.Map[any])
	if !ok {
		logMalformed(env, nil, "input is not a JSON object")
		return nil, Allow()
	}
	policyPath, err := policyPath(args, object, env)
	if err != nil {
		logMalformed(env, object, malformedReason(err))
		return object, Allow()
	}
	resolver := newProjects(hookDir(object, env), homeOf(env))
	resolver.project = projectOf(policyPath)
	info, err := os.Stat(policyPath)
	switch {
	case err == nil && !info.IsDir():
		file, err := os.Open(policyPath)
		if err != nil {
			return object, Allow()
		}
		value, err := readJSON(file)
		file.Close()
		if err != nil {
			return object, Allow()
		}
		policy, err := LoadPolicy(value)
		if err != nil {
			return object, Allow()
		}
		policy = policy.at(hookDir(object, env), resolver.project, homeOf(env))
		resolver.policy = &policy
		resolver.perPath = resolver.project != ""
	case errors.Is(err, os.ErrNotExist) && resolver.project != "" && workspaceFolder(resolver.project):
		resolver.workspace = resolver.project
		resolver.project = ""
		resolver.perPath = true
	default:
		return object, Allow()
	}
	events, err := AllFromHook(object)
	if err != nil {
		logMalformed(env, object, malformedReason(err))
		return object, Allow()
	}
	for _, event := range events {
		if event.Malformed != "" {
			logMalformed(env, object, event.Malformed)
			break
		}
	}
	decision, err := resolver.evaluateAll(events)
	if err != nil {
		return object, Allow()
	}
	return object, decision
}

var errNotUTF8 = errors.New("input is not UTF-8")

// malformedReason is the log reason for an error reading the hook input: the field of the wrong type, never its value.
func malformedReason(err error) string {
	var field *fieldTypeError
	if errors.As(err, &field) {
		return "hook field " + field.key + " has the wrong type"
	}
	return "hook input could not be read"
}

// readJSON decodes UTF-8 JSON as Dart's strict decoder does: malformed UTF-8 is an error, a leading byte order
// mark is skipped.
func readJSON(r io.Reader) (any, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if !utf8.Valid(data) {
		return nil, errNotUTF8
	}
	return dartjson.Decode(strings.TrimPrefix(string(data), "\ufeff"))
}

// hookDir is the folder the agent ran the tool in: the hook's cwd, else its first workspace path, else the
// guard's working folder; empty when none is known.
func hookDir(input *omap.Map[any], env Env) string {
	if cwd, ok := input.Value("cwd").(string); ok && cwd != "" {
		return cwd
	}
	if paths, ok := input.Value("workspacePaths").([]any); ok && len(paths) > 0 {
		if first, ok := paths[0].(string); ok && first != "" {
			return first
		}
	}
	if dir, err := env.Getwd(); err == nil {
		return dir
	}
	return ""
}

// workspaceFolder is true when dir holds a dashcrew project one level down: a folder above several checkouts.
func workspaceFolder(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(dir, "*", ".ai", "guard.json"))
	return len(matches) > 0
}

// projectOf is the project whose `.ai/guard.json` is policyPath; empty for a policy stored anywhere else.
func projectOf(policyPath string) string {
	ai := filepath.Dir(policyPath)
	if filepath.Base(policyPath) != "guard.json" || filepath.Base(ai) != ".ai" {
		return ""
	}
	project, err := filepath.Abs(filepath.Dir(ai))
	if err != nil {
		return ""
	}
	return project
}

// homeOf is HOME, else USERPROFILE; empty when neither is set.
func homeOf(env Env) string {
	for _, key := range []string{"HOME", "USERPROFILE"} {
		if home, ok := env.LookupEnv(key); ok && home != "" {
			return home
		}
	}
	return ""
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
