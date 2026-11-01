package guard

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/guard/shell"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// Decision is the guard's answer to one tool call; Reason is set when Blocked.
type Decision struct {
	Blocked bool
	Reason  string
}

// Allow lets the call through.
func Allow() Decision { return Decision{} }

// Block refuses the call with reason, which the agent shows to the model.
func Block(reason string) Decision { return Decision{Blocked: true, Reason: reason} }

// Response is what the guard prints and its exit code.
type Response struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// HookResponse answers the agent that called: Antigravity (`toolCall` input) reads a JSON decision on stdout,
// Claude Code and Codex read exit code 2 with the reason on stderr. An allowed call prints nothing.
func HookResponse(input *omap.Map[any], decision Decision) Response {
	if !decision.Blocked {
		return Response{}
	}
	reason := "[dashcrew guard] " + decision.Reason
	if _, ok := input.Value("toolCall").(*omap.Map[any]); ok {
		text, _ := dartjson.Encode(omap.Of[any]("decision", "deny", "reason", reason))
		return Response{Stdout: text}
	}
	return Response{ExitCode: 2, Stderr: reason}
}

// LoadPolicy reads a decoded `.ai/guard.json` as the Dart guard does: every key but `schema` is required with its
// type and `blocked_git` holds strings only. Like Dart's lazy casts, an entry of `role_capability`,
// `forbidden_imports` or `local_packages` that is not a string fails only when Evaluate reaches it.
func LoadPolicy(value any) (Policy, error) {
	object, ok := value.(*omap.Map[any])
	if !ok {
		return Policy{}, errors.New("guard policy is not a JSON object")
	}
	blockedList, ok := object.Value("blocked_git").([]any)
	if !ok {
		return Policy{}, errors.New("blocked_git is not a list")
	}
	blocked := map[string]bool{}
	for _, item := range blockedList {
		sub, ok := item.(string)
		if !ok {
			return Policy{}, errors.New("blocked_git holds a value that is not a string")
		}
		blocked[sub] = true
	}
	blockFormatters, ok := object.Value("block_formatters").(bool)
	if !ok {
		return Policy{}, errors.New("block_formatters is not a bool")
	}
	plansDir, ok := object.Value("plans_dir").(string)
	if !ok {
		return Policy{}, errors.New("plans_dir is not a string")
	}
	roleMap, ok := object.Value("role_capability").(*omap.Map[any])
	if !ok {
		return Policy{}, errors.New("role_capability is not a map")
	}
	p := Policy{BlockedGit: blocked, BlockFormatters: blockFormatters, PlansDir: plansDir, RoleCapability: map[string]string{}}
	for _, name := range roleMap.Keys() {
		switch capability := roleMap.Value(name).(type) {
		case string:
			p.RoleCapability[name] = capability
		case nil:
		default:
			if p.brokenRoles == nil {
				p.brokenRoles = map[string]bool{}
			}
			p.brokenRoles[name] = true
		}
	}
	var err error
	if p.ForbiddenImports, p.forbiddenBroken, err = lazyStrings(object, "forbidden_imports"); err != nil {
		return Policy{}, err
	}
	if p.LocalPackages, p.localBroken, err = lazyStrings(object, "local_packages"); err != nil {
		return Policy{}, err
	}
	return p, nil
}

// lazyStrings is the list under key up to its first entry that is not a string; broken tells that one exists.
func lazyStrings(object *omap.Map[any], key string) (items []string, broken bool, err error) {
	list, ok := object.Value(key).([]any)
	if !ok {
		return nil, false, errors.New(key + " is not a list")
	}
	items = []string{}
	for _, item := range list {
		text, ok := item.(string)
		if !ok {
			return items, true, nil
		}
		items = append(items, text)
	}
	return items, false, nil
}

// EvaluateAll blocks when any of events is blocked, with the first reason. An error (a policy entry of the wrong
// type reached on the way) means the call is allowed, as in the Dart guard.
func EvaluateAll(events []Event, p Policy) (Decision, error) {
	for _, event := range events {
		decision, err := Evaluate(event, p)
		if err != nil || decision.Blocked {
			return decision, err
		}
	}
	return Allow(), nil
}

// Evaluate allows or blocks one tool call under p.
func Evaluate(event Event, p Policy) (Decision, error) {
	switch event.Kind {
	case Shell:
		return evaluateShell(event.Command, p), nil
	case Write, Edit:
		return evaluateWrite(event, p)
	}
	return Allow(), nil
}

var (
	wrappers            = map[string]bool{"sudo": true, "env": true, "command": true, "nohup": true, "time": true, "rtk": true, "fvm": true}
	gitOptionsWithValue = map[string]bool{"-C": true, "-c": true, "--git-dir": true, "--work-tree": true, "--namespace": true, "--exec-path": true, "--config-env": true}
	assignment          = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

func evaluateShell(script string, p Policy) Decision {
	for _, raw := range shell.Commands(script) {
		argv := stripPrefixes(raw)
		if len(argv) == 0 {
			continue
		}
		program := argv[0][strings.LastIndexByte(argv[0], '/')+1:]

		if program == "gh" {
			for _, word := range argv {
				if strings.Contains(word, "/approve") {
					return Block("Approvals are the user's, on GitHub. Ask the user to approve, then stop.")
				}
			}
		}
		if program == "git" {
			if sub, ok := gitSubcommand(argv); ok && p.BlockedGit[sub] {
				return Block("git " + sub + " is reserved for the user. Ask them to run it.")
			}
		}
		if p.BlockFormatters && len(argv) > 1 {
			sub := argv[1]
			if (program == "dart" && (sub == "format" || sub == "fix")) || (program == "flutter" && sub == "format") {
				return Block(program + " " + sub + " rewrites untouched lines. Fix only the lines your task needs.")
			}
		}
	}
	return Allow()
}

func stripPrefixes(argv []string) []string {
	i := 0
	for i < len(argv) && (assignment.MatchString(argv[i]) || wrappers[argv[i]]) {
		i++
	}
	return argv[i:]
}

func gitSubcommand(argv []string) (string, bool) {
	for i := 1; i < len(argv); {
		word := argv[i]
		switch {
		case gitOptionsWithValue[word]:
			i += 2
		case strings.HasPrefix(word, "-"):
			i++
		default:
			return word, true
		}
	}
	return "", false
}

// jsWhitespace is `\s` of a Dart (JavaScript) regular expression, which also matches Unicode spaces.
const jsWhitespace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

func evaluateWrite(event Event, p Policy) (Decision, error) {
	path := strings.ReplaceAll(event.FilePath, `\`, "/")
	var capability string
	if event.Role != nil {
		if p.brokenRoles[*event.Role] {
			return Decision{}, fmt.Errorf("role_capability of %s is not a string", *event.Role)
		}
		capability = p.RoleCapability[*event.Role]
	}

	if capability == "read" {
		return Block(*event.Role + " is read-only. Return your findings instead of writing files."), nil
	}
	if capability == "write-plans" {
		if event.Kind == Edit {
			return Block(*event.Role + " may not edit files; write whole files under " + p.PlansDir + "/ only."), nil
		}
		plans := strings.TrimRight(p.PlansDir, "/")
		if !strings.Contains(path, "/"+plans+"/") && !strings.HasPrefix(path, plans+"/") {
			return Block(*event.Role + " may only write under " + plans + "/. Source and tests belong to a write-code role."), nil
		}
	}

	if strings.HasSuffix(path, ".dart") {
		appLib, err := isAppLib(path, p)
		if err != nil || !appLib {
			return Allow(), err
		}
		for _, prefix := range p.ForbiddenImports {
			if regexp.MustCompile(`import` + jsWhitespace + `+['"]` + regexp.QuoteMeta(prefix)).MatchString(event.Content) {
				return Block("Do not import " + prefix + " in app code; go through the project facade."), nil
			}
		}
		if p.forbiddenBroken {
			return Decision{}, errors.New("forbidden_imports holds a value that is not a string")
		}
	}
	return Allow(), nil
}

func isAppLib(path string, p Policy) (bool, error) {
	if !strings.Contains(path, "/lib/") && !strings.HasPrefix(path, "lib/") {
		return false, nil
	}
	for _, pkg := range p.LocalPackages {
		if strings.Contains(path, "/"+pkg+"/lib/") || strings.HasPrefix(path, pkg+"/lib/") {
			return false, nil
		}
	}
	if p.localBroken {
		return false, errors.New("local_packages holds a value that is not a string")
	}
	return true, nil
}
