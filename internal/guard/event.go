package guard

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/yamlutil"
)

// Kind is the kind of tool call the guard sees.
type Kind int

const (
	Shell Kind = iota
	Write
	Edit
	Other
)

// Event is one hook input normalized across agents. Missing text is empty; Role is nil when the hook names no
// subagent, which no role capability matches.
type Event struct {
	Kind     Kind
	Command  string
	FilePath string
	Content  string
	Role     *string
}

// typeError is the Dart guard's cast failure: the call is allowed, as every read or parse failure is.
func typeError(key string, value any) error {
	return fmt.Errorf("%s is %T, not a string", key, value)
}

// optionalString is Dart's `value as String?`.
func optionalString(key string, value any) (*string, error) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case string:
		return &v, nil
	}
	return nil, typeError(key, value)
}

func orEmpty(text *string) string {
	if text == nil {
		return ""
	}
	return *text
}

// objectAt is the object under key, or an empty one when the value is not an object.
func objectAt(json *omap.Map[any], key string) *omap.Map[any] {
	if object, ok := json.Value(key).(*omap.Map[any]); ok {
		return object
	}
	return omap.New[any]()
}

// firstNonNull is Dart's `a ?? b ?? ...` over values of object.
func firstNonNull(object *omap.Map[any], keys ...string) any {
	for _, key := range keys {
		if value := object.Value(key); value != nil {
			return value
		}
	}
	return nil
}

var shellTools = map[string]bool{"Bash": true, "shell": true, "local_shell": true, "exec_command": true, "container.exec": true}

// FromHook reads one Claude Code or Codex hook input: `Bash`, `Write`, `Edit`, `MultiEdit`, `NotebookEdit` and
// `agent_type`; Codex `shell` or `exec_command` with a string or an argv list (`bash -lc <script>` is unwrapped).
// An error is a value of the wrong type, which the guard answers by allowing the call.
func FromHook(json *omap.Map[any]) (Event, error) {
	tool, err := optionalString("tool_name", json.Value("tool_name"))
	if err != nil {
		return Event{}, err
	}
	input := objectAt(json, "tool_input")
	role, err := optionalString("agent_type", json.Value("agent_type"))
	if err != nil {
		return Event{}, err
	}

	switch name := orEmpty(tool); {
	case shellTools[name]:
		return Event{Kind: Shell, Command: shellText(input.Value("command")), Role: role}, nil
	case name == "Write":
		path, err := optionalString("file_path", input.Value("file_path"))
		if err != nil {
			return Event{}, err
		}
		content, err := optionalString("content", input.Value("content"))
		if err != nil {
			return Event{}, err
		}
		return Event{Kind: Write, FilePath: orEmpty(path), Content: orEmpty(content), Role: role}, nil
	case name == "Edit" || name == "MultiEdit" || name == "NotebookEdit":
		var content string
		if edits, ok := input.Value("edits").([]any); ok {
			parts := make([]string, len(edits))
			for i, edit := range edits {
				object, ok := edit.(*omap.Map[any])
				if !ok {
					return Event{}, fmt.Errorf("edit %d is %T, not a map", i, edit)
				}
				if value := object.Value("new_string"); value != nil {
					parts[i] = yamlutil.DartString(value)
				}
			}
			content = strings.Join(parts, "\n")
		} else if value := firstNonNull(input, "new_string", "new_source"); value != nil {
			content = yamlutil.DartString(value)
		}
		path, err := optionalString("file_path", firstNonNull(input, "file_path", "notebook_path"))
		if err != nil {
			return Event{}, err
		}
		return Event{Kind: Edit, FilePath: orEmpty(path), Content: content, Role: role}, nil
	case name == "apply_patch":
		events, err := AllFromHook(json)
		if err != nil {
			return Event{}, err
		}
		if len(events) == 0 {
			return Event{Kind: Edit, Role: role}, nil
		}
		return events[0], nil
	}
	return Event{Kind: Other, Role: role}, nil
}

// shellText is the script of a shell tool's `command`: a string as is, an argv list joined by spaces, and the
// script of `bash|sh|zsh ... -c|-lc <script>`.
func shellText(command any) string {
	switch v := command.(type) {
	case string:
		return v
	case []any:
		argv := make([]string, len(v))
		flag := -1
		for i, item := range v {
			argv[i] = yamlutil.DartString(item)
			if flag < 0 && (argv[i] == "-c" || argv[i] == "-lc") {
				flag = i
			}
		}
		if len(argv) > 0 && flag >= 0 {
			switch argv[0][strings.LastIndexByte(argv[0], '/')+1:] {
			case "bash", "sh", "zsh":
				if flag+1 < len(argv) {
					return argv[flag+1]
				}
				return ""
			}
		}
		return strings.Join(argv, " ")
	}
	return ""
}

// The Dart patterns run without the multiline flag, so `.` stops at \r, U+2028 and U+2029 as well as \n, and `$`
// is the end of the line text: a header line ending in \r is not a header.
var (
	patchHeader = regexp.MustCompile(`^\*\*\* (Add|Update|Delete) File: ([^\n\r\x{2028}\x{2029}]+)$`)
	patchMoveTo = regexp.MustCompile(`^\*\*\* Move to: ([^\n\r\x{2028}\x{2029}]+)$`)
)

// AllFromHook is every event of one hook call: a Codex `apply_patch` gives one per file, an Antigravity
// `toolCall` and any other tool one. A patch file's content is its added lines; a moved file is checked at its new path.
func AllFromHook(json *omap.Map[any]) ([]Event, error) {
	if toolCall, ok := json.Value("toolCall").(*omap.Map[any]); ok {
		event, err := antigravity(toolCall)
		if err != nil {
			return nil, err
		}
		return []Event{event}, nil
	}
	if json.Value("tool_name") != "apply_patch" {
		event, err := FromHook(json)
		if err != nil {
			return nil, err
		}
		return []Event{event}, nil
	}

	input := objectAt(json, "tool_input")
	patch := ""
	if value := firstNonNull(input, "command", "input", "patch"); value != nil {
		patch = yamlutil.DartString(value)
	}
	role, err := optionalString("agent_type", json.Value("agent_type"))
	if err != nil {
		return nil, err
	}

	var events []Event
	var path *string
	var kind *Kind
	var added []string
	flush := func() error {
		if path != nil {
			if kind == nil {
				return fmt.Errorf("patch moves %s before naming a file", *path)
			}
			events = append(events, Event{Kind: *kind, FilePath: *path, Content: strings.Join(added, "\n"), Role: role})
		}
		added = nil
		return nil
	}
	for _, line := range strings.Split(patch, "\n") {
		if file := patchHeader.FindStringSubmatch(line); file != nil {
			if err := flush(); err != nil {
				return nil, err
			}
			k := Edit
			if file[1] == "Add" {
				k = Write
			}
			kind = &k
			p := dartTrim(file[2])
			path = &p
		} else if move := patchMoveTo.FindStringSubmatch(line); move != nil {
			p := dartTrim(move[1])
			path = &p
		} else if strings.HasPrefix(line, "+") && path != nil {
			added = append(added, line[1:])
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return events, nil
}

// dartTrim is Dart's String.trim: Unicode White_Space and the byte order mark.
func dartTrim(text string) string {
	return strings.TrimFunc(text, func(r rune) bool {
		switch {
		case r >= 0x09 && r <= 0x0D, r == 0x20, r == 0x85, r == 0xA0, r == 0x1680, r >= 0x2000 && r <= 0x200A,
			r == 0x2028, r == 0x2029, r == 0x202F, r == 0x205F, r == 0x3000, r == 0xFEFF:
			return true
		}
		return false
	})
}

// antigravity reads a `toolCall`: `run_command`, `write_to_file`, `replace_file_content` and
// `multi_replace_file_content`. Its hooks carry no subagent identity, so the role stays nil.
func antigravity(toolCall *omap.Map[any]) (Event, error) {
	args := objectAt(toolCall, "args")
	path, err := optionalString("TargetFile", args.Value("TargetFile"))
	if err != nil {
		return Event{}, err
	}
	text := func(key string) (string, error) {
		value, err := optionalString(key, args.Value(key))
		return orEmpty(value), err
	}
	switch toolCall.Value("name") {
	case "run_command":
		command, err := text("CommandLine")
		return Event{Kind: Shell, Command: command}, err
	case "write_to_file":
		content, err := text("CodeContent")
		return Event{Kind: Write, FilePath: orEmpty(path), Content: content}, err
	case "replace_file_content":
		content, err := text("ReplacementContent")
		return Event{Kind: Edit, FilePath: orEmpty(path), Content: content}, err
	case "multi_replace_file_content":
		var replacements []string
		chunks, _ := args.Value("ReplacementChunks").([]any)
		for _, chunk := range chunks {
			object, ok := chunk.(*omap.Map[any])
			if !ok {
				continue
			}
			for _, key := range object.Keys() {
				if value, ok := object.Value(key).(string); ok && strings.HasPrefix(key, "Replacement") {
					replacements = append(replacements, value)
				}
			}
		}
		return Event{Kind: Edit, FilePath: orEmpty(path), Content: strings.Join(replacements, "\n")}, nil
	}
	return Event{Kind: Other}, nil
}
