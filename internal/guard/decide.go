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

// Decision is the guard's answer to one tool call; Reason is set when Blocked or Ask. Ask lets the call run only
// once the user approves it, on an agent that can ask (see HookResponse).
type Decision struct {
	Blocked bool
	Ask     bool
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

// ClaudeCodeArgument is the second argument of the Claude Code hook command. It is how the guard knows Claude Code
// called: Codex sends the same `Bash` payload, and Codex runs a call whose hook asked.
const ClaudeCodeArgument = "claude-code"

// HookResponse answers the agent that called: Antigravity (`toolCall` input) reads a JSON decision on stdout, and
// gets `allow` for an allowed call since it fails closed on a missing one (`allow` hands the call on to the next hook
// or its default policy, it approves nothing). Claude Code and Codex read exit code 2 with the reason on stderr, and
// nothing for an allowed call. input may be nil when the hook input did not parse.
//
// An ask goes to Antigravity as `force_ask` and to Claude Code, known by agent ClaudeCodeArgument and a payload of its
// shape, as a JSON ask; every other agent gets a block, since none of them is known to ask. Cursor runs the Claude
// Code hook, argument included, so its payload (`cursor_version`) gets the block too.
func HookResponse(input *omap.Map[any], decision Decision, agent string) Response {
	antigravity := false
	if input != nil {
		_, antigravity = input.Value("toolCall").(*omap.Map[any])
	}
	if decision.Ask && !decision.Blocked {
		reason := "[dashcrew guard] " + decision.Reason
		switch {
		case antigravity:
			text, _ := dartjson.Encode(omap.Of[any]("decision", "force_ask", "reason", reason))
			return Response{Stdout: text}
		case agent == ClaudeCodeArgument && agentOf(input) == "claude-code":
			text, _ := dartjson.Encode(omap.Of[any]("hookSpecificOutput", omap.Of[any]("hookEventName", "PreToolUse",
				"permissionDecision", "ask", "permissionDecisionReason", reason)))
			return Response{Stdout: text}
		}
		decision = Block(decision.Reason + "; this agent's hooks cannot ask for approval, so ask the user to run it")
	}
	if !decision.Blocked {
		if antigravity {
			return Response{Stdout: `{"decision":"allow"}`}
		}
		return Response{}
	}
	reason := "[dashcrew guard] " + decision.Reason
	if antigravity {
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
	if p.KnowledgeClone, err = knowledgeCloneOf(object); err != nil {
		return Policy{}, err
	}
	if err := backendRulesOf(object, &p); err != nil {
		return Policy{}, err
	}
	if p.Push, err = pushRuleOf(object); err != nil {
		return Policy{}, err
	}
	if p.AskGit, err = askGitOf(object); err != nil {
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

// EvaluateAll blocks when any of events is blocked, with the first reason; else it asks when any asks. An error (a
// policy entry of the wrong type reached on the way) means the call is allowed, as in the Dart guard.
func EvaluateAll(events []Event, p Policy) (Decision, error) {
	result := Allow()
	for _, event := range events {
		decision, err := Evaluate(event, p)
		if err != nil || decision.Blocked {
			return decision, err
		}
		if decision.Ask && !result.Ask {
			result = decision
		}
	}
	return result, nil
}

// Evaluate allows, asks about or blocks one tool call under p. A call the rules allow is checked again with the
// AskGit subcommands blocked: what that blocks, in any of the forms the rules unwrap, is asked about instead.
func Evaluate(event Event, p Policy) (Decision, error) {
	decision, err := evaluate(event, p)
	if err != nil || decision.Blocked || len(p.AskGit) == 0 || p.asking {
		return decision, err
	}
	strict := p
	strict.asking = true
	strict.BlockedGit = map[string]bool{}
	for sub := range p.BlockedGit {
		strict.BlockedGit[sub] = true
	}
	for sub := range p.AskGit {
		strict.BlockedGit[sub] = true
	}
	if second, err := evaluate(event, strict); err == nil && second.Blocked {
		return Decision{Ask: true, Reason: second.Reason}, nil
	}
	return decision, nil
}

func evaluate(event Event, p Policy) (Decision, error) {
	switch event.Kind {
	case Shell:
		return evaluateShell(event.Command, event.Argv, p), nil
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

// evaluateShell checks script with the Dart rules, then, to block what they miss, after unwrapping shells,
// keywords and wrappers (G4b). argv is the command when the agent sent an argv list instead of a script.
func evaluateShell(script string, argv []string, p Policy) Decision {
	if decision := evaluateShellAsDart(script, p); decision.Blocked {
		return decision
	}
	if decision := evaluateNested(script, p, 0); decision.Blocked {
		return decision
	}
	if argv != nil {
		if decision := evaluateUnwrapped(argv, p, 0); decision.Blocked {
			return decision
		}
	}
	if decision := evaluateBackend(script, argv, p); decision.Blocked {
		return decision
	}
	if p.knowledge != nil && len(p.knowledge.roots) > 0 {
		return p.knowledge.shellWrites(script, argv)
	}
	return Allow()
}

// evaluateShellAsDart is the Dart guard's shell check.
func evaluateShellAsDart(script string, p Policy) Decision {
	for _, raw := range shell.Commands(script) {
		if decision := checkCommand(stripPrefixes(raw), p); decision.Blocked {
			return decision
		}
	}
	return Allow()
}

// checkCommand applies the shell rules to one command whose wrappers are already stripped.
func checkCommand(argv []string, p Policy) Decision {
	if len(argv) == 0 {
		return Allow()
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
		if at, ok := gitSubcommandAt(argv); ok {
			sub := argv[at]
			if p.asking && p.AskGit[sub] {
				return Block("git " + sub + " needs your approval (policy.agent_" + sub + ": ask)")
			}
			if p.BlockedGit[sub] {
				return Block("git " + sub + " is reserved for the user. Ask them to run it.")
			}
			if sub == "push" && p.Push != nil {
				if reason := p.Push.check(argv[at+1:]); reason != "" {
					return Block(reason)
				}
			}
		}
	}
	if reason := blockedCommand(argv, p); reason != "" {
		return Block(reason)
	}
	if p.BlockFormatters && len(argv) > 1 {
		sub := argv[1]
		writingFormat := program == "dart" && sub == "format" && dartFormatWrites(argv[2:])
		if writingFormat || (program == "dart" && sub == "fix") || (program == "flutter" && sub == "format") {
			return Block(program + " " + sub + " rewrites untouched lines. Fix only the lines your task needs.")
		}
	}
	return Allow()
}

// dartFormatFlags and dartFormatValueOptions are the `dart format` options dartFormatWrites knows; a value option
// given without `=` takes the next word as its value, whatever it looks like.
var (
	dartFormatFlags        = map[string]bool{"--set-exit-if-changed": true, "--follow-links": true, "--fix": true, "-v": true, "--verbose": true}
	dartFormatValueOptions = map[string]bool{"-l": true, "--line-length": true, "--page-width": true, "--indent": true, "--language-version": true, "--enable-experiment": true, "--show": true, "--summary": true, "--trailing-commas": true}
)

// dartFormatWrites reports whether `dart format` with these arguments may write files: true unless its last
// --output (-o) value, read before a `--`, is none, show or json. An option it does not know, or a missing or
// unknown output value, counts as writing, since an unknown option could take the next word as its value.
func dartFormatWrites(args []string) bool {
	output := "write"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, _, hasValue := strings.Cut(arg, "=")
		switch {
		case arg == "--":
			i = len(args)
		case arg == "--output" || arg == "-o" || dartFormatValueOptions[arg]:
			if i+1 >= len(args) {
				return true
			}
			i++
			if arg == "--output" || arg == "-o" {
				output = args[i]
			}
		case name == "--output" && hasValue:
			output = strings.TrimPrefix(arg, "--output=")
		case strings.HasPrefix(arg, "-o") && !strings.HasPrefix(arg, "--"):
			output = strings.TrimPrefix(arg, "-o")
		case strings.HasPrefix(arg, "-l") && !strings.HasPrefix(arg, "--"):
			// -l100: the line length attached to its flag.
		case dartFormatFlags[arg], hasValue && dartFormatValueOptions[name]:
		case strings.HasPrefix(arg, "-"):
			return true
		}
	}
	return output != "none" && output != "show" && output != "json"
}

func stripPrefixes(argv []string) []string {
	i := 0
	for i < len(argv) && (assignment.MatchString(argv[i]) || wrappers[argv[i]]) {
		i++
	}
	return argv[i:]
}

// gitSubcommandAt is the index in argv of git's subcommand, past git's own options.
func gitSubcommandAt(argv []string) (int, bool) {
	for i := 1; i < len(argv); {
		word := argv[i]
		switch {
		case gitOptionsWithValue[word]:
			i += 2
		case strings.HasPrefix(word, "-"):
			i++
		default:
			return i, true
		}
	}
	return 0, false
}

// jsWhitespace is `\s` of a Dart (JavaScript) regular expression, which also matches Unicode spaces.
const jsWhitespace = `[\t\n\v\f\r \x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}\x{feff}]`

// evaluateWrite runs the write rules on the path as written, then, when Run gave the policy a working folder (K2),
// the knowledge rule on its real path and the write rules again on the real path when a link moves it elsewhere
// in the project.
func evaluateWrite(event Event, p Policy) (Decision, error) {
	decision, err := evaluateWritePath(event, p)
	if err != nil || decision.Blocked || p.knowledge == nil || event.FilePath == "" {
		return decision, err
	}
	if reason := p.knowledge.blocks(event.FilePath, p.knowledge.cwd); reason != "" {
		return Block(reason), nil
	}
	if real, moved := p.knowledge.appPath(event.FilePath); moved {
		event.FilePath = real
		return evaluateWritePath(event, p)
	}
	return Allow(), nil
}

// evaluateWritePath is the Dart guard's write check, on the path's text.
func evaluateWritePath(event Event, p Policy) (Decision, error) {
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
