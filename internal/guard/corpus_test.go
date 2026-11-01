package guard

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
	"github.com/fahnaladitia/dashcrew/internal/omap"
)

// corpusCase is one `dashcrew guard` invocation. In stdin, args and env values, `{project}` is a project folder
// whose `.ai/guard.json` is the default policy, and `{policies}` the folder of the named policies in corpusPolicies.
type corpusCase struct {
	name  string
	stdin string
	args  []string
	env   map[string]string
	// inProject runs the process in `{project}` instead of a folder without a policy.
	inProject bool
}

// corpusPolicies are written to `{policies}/<name>.json`; "default" is also `{project}/.ai/guard.json`.
var corpusPolicies = map[string]string{
	"default":      `{"schema":1,"blocked_git":["cherry-pick","commit","merge","push","rebase","reset","revert","stash","tag"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{"engineer":"write-code","explorer":"read","planner":"write-plans","reviewer":"read"},"forbidden_imports":["package:vendor_sdk/"],"local_packages":["core"]}`,
	"relaxed":      `{"schema":1,"blocked_git":["cherry-pick","merge","push","rebase","reset","revert","stash","tag"],"block_formatters":false,"plans_dir":"docs/plans/","role_capability":{"planner":"write-plans","reviewer":"read"},"forbidden_imports":["package:a.b/","package:x"],"local_packages":["core","packages/ui"]}`,
	"plan-gate":    `{"schema":1,"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{"reviewer":"read"},"forbidden_imports":[],"local_packages":[],"plan_gate":true}`,
	"lazy-role":    `{"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{"reviewer":"read","odd":7,"none":null},"forbidden_imports":["package:vendor_sdk/"],"local_packages":["core"]}`,
	"lazy-import":  `{"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{},"forbidden_imports":["package:vendor_sdk/",3,"package:late/"],"local_packages":["core"]}`,
	"lazy-local":   `{"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{},"forbidden_imports":["package:vendor_sdk/"],"local_packages":["core",false,"other"]}`,
	"missing-key":  `{"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{},"forbidden_imports":[]}`,
	"bad-git":      `{"blocked_git":["push",1],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{},"forbidden_imports":[],"local_packages":[]}`,
	"not-object":   `["push"]`,
	"not-json":     `{"blocked_git":`,
	"empty":        ``,
	"bom":          "\ufeff" + `{"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{},"forbidden_imports":[],"local_packages":[]}`,
	"bad-utf8":     `{"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/pl` + "\xff" + `ans","role_capability":{},"forbidden_imports":[],"local_packages":[]}`,
	"regex-prefix": `{"blocked_git":["push"],"block_formatters":true,"plans_dir":".ai/plans","role_capability":{},"forbidden_imports":["package:a+b/(x)","package:[v]"],"local_packages":["c.re"]}`,
}

// seedCommands are the shell commands of the ported tests, the base of every mutation.
func seedCommands() []string {
	var seeds []string
	for _, entry := range blockedShell {
		seeds = append(seeds, entry[0])
	}
	for _, entry := range allowedShell {
		seeds = append(seeds, entry[0])
	}
	return append(seeds,
		"git -C app push",
		"git commit -m x",
		`gh issue comment 42 --body "/approve plan"`,
		`gh api repos/a/b/issues/42/comments -f body='/approve plan'`,
		`rtk gh pr comment 3 --body "/approve plan"`,
		"gh issue view 42",
		`echo "/approve plan"`,
		"git status && flutter test test/a_test.dart; ls | wc -l",
		`echo 'a; git push'`,
		`echo "$(git push origin)"`,
		"x=`git stash`",
		"cat > notes.md <<'EOF'\nrun git commit later\ndart format is banned\nEOF\nls",
		"cat <<-EOF > a.md\n\tgit push\n\tEOF\necho done",
		"bash <<EOF\ngit push\nEOF\nls",
		"cat <<'EOF' | /bin/sh\ndart format lib\nEOF",
		"flutter test > out.txt 2>&1",
		"(cd app && git commit -m x)",
		`echo a\;b`,
		"git --git-dir=.git push",
		"git --work-tree . push",
		"git -c a=b -C x --no-pager tag v2",
		"env FOO=1 BAR=2 git merge main",
		"nohup git rebase main",
		"time git reset --hard",
		"command git stash pop",
		"fvm dart format .",
		"fvm flutter format lib",
		"/usr/bin/git push",
		"dart fix",
		"dart",
		"git",
		"",
	)
}

// shellMutations wrap or rewrite one command: quoting, separators, subshells, prefixes, shells, escapes,
// unicode, whitespace and redirects. Many of them change the meaning on purpose; only equal answers matter.
var shellMutations = []func(string) string{
	func(c string) string { return c },
	func(c string) string { return "ls && " + c },
	func(c string) string { return c + " && ls" },
	func(c string) string { return "ls; " + c },
	func(c string) string { return c + ";ls" },
	func(c string) string { return "ls | " + c },
	func(c string) string { return c + " | tee out" },
	func(c string) string { return "true || " + c },
	func(c string) string { return "sleep 1 & " + c },
	func(c string) string { return "(" + c + ")" },
	func(c string) string { return "{ " + c + "; }" },
	func(c string) string { return "$(" + c + ")" },
	func(c string) string { return "echo $(" + c + ")" },
	func(c string) string { return `echo "$(` + c + `)"` },
	func(c string) string { return "echo `" + c + "`" },
	func(c string) string { return `echo "` + "`" + c + "`" + `"` },
	func(c string) string { return "x=$(echo $(" + c + "))" },
	func(c string) string { return "FOO=1 " + c },
	func(c string) string { return "A=1 B='x y' " + c },
	func(c string) string { return "env " + c },
	func(c string) string { return "sudo " + c },
	func(c string) string { return "sudo -u root " + c },
	func(c string) string { return "rtk " + c },
	func(c string) string { return "sh -c '" + c + "'" },
	func(c string) string { return `bash -lc "` + c + `"` },
	func(c string) string { return "eval '" + c + "'" },
	func(c string) string { return "bash <<EOF\n" + c + "\nEOF" },
	func(c string) string { return "bash <<'EOF'\n" + c + "\nEOF\necho after" },
	func(c string) string { return "cat <<EOF\n" + c + "\nEOF" },
	func(c string) string { return "cat <<-EOF | sh\n\t" + c + "\n\tEOF" },
	func(c string) string { return "ssh host <<EOF\n" + c + "\nEOF" },
	func(c string) string { return "python3 - <<'PY'\n" + c + "\nPY" },
	func(c string) string { return "sh <<< '" + c + "'" },
	func(c string) string { return "'" + c + "'" },
	func(c string) string { return `"` + c + `"` },
	func(c string) string { return quoteFirstWord(c, "'") },
	func(c string) string { return quoteFirstWord(c, `"`) },
	func(c string) string { return strings.Replace(c, "git", `g"i"t`, 1) },
	func(c string) string { return strings.Replace(c, "git", `g\it`, 1) },
	func(c string) string { return strings.Replace(c, "git", "g\\\nit", 1) },
	func(c string) string { return strings.Replace(c, " ", `\ `, 1) },
	func(c string) string { return strings.Replace(c, " ", " \\\n ", 1) },
	func(c string) string { return c + ` \` },
	func(c string) string { return strings.ReplaceAll(c, " ", "\t") },
	func(c string) string { return strings.ReplaceAll(c, " ", "  ") },
	func(c string) string { return strings.ReplaceAll(c, " ", "\u00a0") },
	func(c string) string { return strings.Replace(c, "git", "g\u0456t", 1) },
	func(c string) string { return strings.Replace(c, "git", "ｇｉｔ", 1) },
	func(c string) string { return "echo é; " + c + " # ünïcode ✓ 𝔤" },
	func(c string) string { return "\n" + c + "\n" },
	func(c string) string { return "ls\n" + c },
	func(c string) string { return "ls\r\n" + c + "\r\n" },
	func(c string) string { return "# " + c },
	func(c string) string { return "ls #" + c },
	func(c string) string { return "ls a#b " + c },
	func(c string) string { return c + " > out.txt 2>&1" },
	func(c string) string { return "2>/dev/null " + c },
	func(c string) string { return c + " &>/dev/null" },
	func(c string) string { return "1>&2 " + c + " <in.txt" },
	func(c string) string { return c + " >| out 2>&-" },
	func(c string) string { return strings.Replace(c, "git", "/usr/bin/git", 1) },
	func(c string) string { return strings.Replace(c, "git", "../../bin/git", 1) },
	func(c string) string { return strings.Replace(c, "git", "./tools/../git", 1) },
	func(c string) string { return strings.Replace(c, "git", "/opt/link/./bin//git", 1) },
	func(c string) string { return strings.Replace(c, "git", "git.exe", 1) },
	func(c string) string { return "if true; then " + c + "; fi" },
	func(c string) string { return "for f in a b; do " + c + "; done" },
	func(c string) string { return "xargs " + c },
	func(c string) string { return "echo '" + c },
	func(c string) string { return `echo "` + c },
	func(c string) string { return "echo $(" + c },
	func(c string) string { return "echo `" + c },
	func(c string) string { return c + ")" },
	func(c string) string { return "$((1+2)) " + c },
	func(c string) string { return `echo "\"$(` + c + `)\""` },
	func(c string) string { return "echo $( echo ')' ; " + c + " )" },
	func(c string) string { return "cat <<A <<B\na\nA\n" + c + "\nB\n" + c },
	func(c string) string { return "bash <<A; cat <<B\n" + c + "\nA\n" + c + "\nB" },
	func(c string) string { return "cat << \"E O\"\n" + c + "\nE O\nls" },
}

func quoteFirstWord(command, quote string) string {
	word, rest, found := strings.Cut(command, " ")
	if !found {
		return quote + command + quote
	}
	return quote + word + quote + " " + rest
}

func jsonOf(value any) string {
	text, err := dartjson.Encode(value)
	if err != nil {
		panic(err)
	}
	return text
}

// shellPayloads wrap one script the way each agent sends it.
func shellPayloads(script, role string) []string {
	agentType := ""
	if role != "" {
		agentType = `,"agent_type":` + jsonOf(role)
	}
	return []string{
		`{"tool_name":"Bash","tool_input":{"command":` + jsonOf(script) + `}` + agentType + `}`,
		`{"tool_name":"shell","tool_input":{"command":["bash","-lc",` + jsonOf(script) + `]}}`,
		`{"tool_name":"exec_command","tool_input":{"command":` + jsonOf(script) + `}}`,
		`{"conversationId":"c1","workspacePaths":["/repo"],"toolCall":{"name":"run_command","args":{"CommandLine":` + jsonOf(script) + `,"Cwd":"/repo"}}}`,
	}
}

var (
	corpusPaths = []string{
		"/repo/lib/feature/a.dart", "lib/a.dart", "/repo/core/lib/a.dart", "core/lib/a.dart", "/repo/test/a_test.dart",
		"/repo/.ai/plans/x/plan.md", ".ai/plans/plan.md", "/repo/.ai/plans/../../lib/a.dart", "/repo/lib/../.ai/plans/a.md",
		`C:\repo\lib\a.dart`, `C:\repo\.ai\plans\a.md`, "/repo/link/lib/a.dart", "/repo/packages/ui/lib/a.dart",
		"/repo/docs/plans/a.md", "docs/plans/a.md", "/repo/lib/a.dart.bak", "/repo/lib/a.DART", "/repo/libx/a.dart",
		"/repo/c.re/lib/a.dart", "/repo/cXre/lib/a.dart", "/repo/.ai/plansx/a.md", "", "/repo/lib/ünï.dart",
	}
	corpusRoles    = []string{"", "planner", "reviewer", "engineer", "explorer", "Explore", "odd", "none", "null"}
	corpusContents = []string{
		"x", vendorImport, "import\t'package:vendor_sdk/a.dart';", "import\u00a0'package:vendor_sdk/a.dart';",
		"import\u2028\"package:vendor_sdk/a.dart\";", "import\u180e'package:vendor_sdk/a.dart';", "import\u000b'package:vendor_sdk/a.dart';",
		"import\ufeff'package:vendor_sdk/a.dart';", "import'package:vendor_sdk/a.dart';", "// import 'package:vendor_sdk/a.dart';",
		"export 'package:vendor_sdk/a.dart';", "import 'package:a.b/x.dart';", "import 'package:aXb/x.dart';", "import \"package:x\";",
		"import 'package:a+b/(x)';", "import 'package:[v]';", "import 'package:late/a.dart';", "a\nimport  \n 'package:vendor_sdk/'",
	}
)

// writePayloads are the write and edit calls of every agent for one path, role and content.
func writePayloads(path, role, content string) []string {
	agentType := ""
	if role != "" {
		agentType = `,"agent_type":` + jsonOf(role)
	}
	p, c := jsonOf(path), jsonOf(content)
	return []string{
		`{"tool_name":"Write","tool_input":{"file_path":` + p + `,"content":` + c + `}` + agentType + `}`,
		`{"tool_name":"Edit","tool_input":{"file_path":` + p + `,"old_string":"a","new_string":` + c + `}` + agentType + `}`,
		`{"tool_name":"MultiEdit","tool_input":{"file_path":` + p + `,"edits":[{"old_string":"a","new_string":"b"},{"new_string":` + c + `}]}` + agentType + `}`,
		`{"tool_name":"NotebookEdit","tool_input":{"notebook_path":` + p + `,"new_source":` + c + `}` + agentType + `}`,
		`{"tool_name":"apply_patch","tool_input":{"command":` + jsonOf("*** Begin Patch\n*** Add File: "+path+"\n+"+strings.ReplaceAll(content, "\n", "\n+")+"\n*** End Patch\n") + `}` + agentType + `}`,
		`{"toolCall":{"name":"write_to_file","args":{"TargetFile":` + p + `,"CodeContent":` + c + `}}}`,
		`{"toolCall":{"name":"replace_file_content","args":{"TargetFile":` + p + `,"TargetContent":"a","ReplacementContent":` + c + `}}}`,
		`{"toolCall":{"name":"multi_replace_file_content","args":{"TargetFile":` + p + `,"ReplacementChunks":[{"TargetContent":` + c + `,"ReplacementContent":"b"},{"ReplacementContent":` + c + `}]}}}`,
	}
}

// oddPayloads are hook inputs with values of the wrong type, malformed JSON and unusual encodings.
func oddPayloads() []string {
	return []string{
		``, `   `, `null`, `[]`, `"git push"`, `42`, `{`, `{"tool_name":"Bash"} trailing`, "\ufeff" + `{"tool_name":"Bash","tool_input":{"command":"git push"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push"},"x":"` + "\xff" + `"}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push"},"x":"` + "\xed\xa0\x80" + `"}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push\ud800"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push \ud800"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"git \u0070ush"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push"},"tool_name":"Read"}`,
		`{"tool_name":"Read","tool_input":{"command":"git push"},"tool_name":"Bash"}`,
		`{"tool_name":7,"tool_input":{"command":"git push"}}`,
		`{"tool_name":null,"tool_input":{"command":"git push"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push"},"agent_type":5}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push"},"agent_type":null}`,
		`{"tool_name":"Bash","tool_input":"git push"}`,
		`{"tool_name":"Bash","tool_input":{"command":7}}`,
		`{"tool_name":"Bash","tool_input":{"command":null}}`,
		`{"tool_name":"Bash","tool_input":{"command":{"a":"git push"}}}`,
		`{"tool_name":"shell","tool_input":{"command":["git","push"]}}`,
		`{"tool_name":"shell","tool_input":{"command":["/bin/zsh","-c","git push"]}}`,
		`{"tool_name":"shell","tool_input":{"command":["bash","-lc"]}}`,
		`{"tool_name":"shell","tool_input":{"command":["bash","x.sh","-c","git push"]}}`,
		`{"tool_name":"shell","tool_input":{"command":["fish","-c","git push"]}}`,
		`{"tool_name":"shell","tool_input":{"command":["git",1,2.5,1e21,-0.0,1e-7,null,true,["push"],{"a":1}]}}`,
		`{"tool_name":"shell","tool_input":{"command":["git",12345678901234567890,"push"]}}`,
		`{"tool_name":"shell","tool_input":{"command":[]}}`,
		`{"tool_name":"local_shell","tool_input":{"command":["sh","-c","dart format ."]}}`,
		`{"tool_name":"container.exec","tool_input":{"command":"git tag v1"}}`,
		`{"tool_name":"Write","tool_input":{"file_path":7,"content":"x"}}`,
		`{"tool_name":"Write","tool_input":{"file_path":"/repo/lib/a.dart","content":7}}`,
		`{"tool_name":"Write","tool_input":{"file_path":"/repo/lib/a.dart"},"agent_type":"reviewer"}`,
		`{"tool_name":"Edit","tool_input":{"file_path":"/repo/lib/a.dart","new_string":7},"agent_type":"reviewer"}`,
		`{"tool_name":"Edit","tool_input":{"file_path":"/repo/lib/a.dart","new_string":["import 'package:vendor_sdk/x'"]}}`,
		`{"tool_name":"Edit","tool_input":{"file_path":"/repo/lib/a.dart","new_string":{"k":"import 'package:vendor_sdk/x'"}}}`,
		`{"tool_name":"MultiEdit","tool_input":{"file_path":"/repo/lib/a.dart","edits":["import 'package:vendor_sdk/x'"]}}`,
		`{"tool_name":"MultiEdit","tool_input":{"file_path":"/repo/lib/a.dart","edits":[{"new_string":null},{"new_string":12}]}}`,
		`{"tool_name":"MultiEdit","tool_input":{"file_path":"/repo/lib/a.dart","edits":{"new_string":"import 'package:vendor_sdk/x'"}}}`,
		`{"tool_name":"NotebookEdit","tool_input":{"notebook_path":"/repo/lib/a.dart","file_path":null,"new_source":"import 'package:vendor_sdk/x'"}}`,
		`{"tool_name":"Edit","tool_input":{"file_path":"/repo/lib/a.dart","new_string":null,"new_source":"import 'package:vendor_sdk/x'"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Move to: lib/a.dart\n+import 'package:vendor_sdk/x'\n"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File: lib/a.dart\r\n+import 'package:vendor_sdk/x'\r\n"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File:  lib/a.dart \u3000\n+import 'package:vendor_sdk/x'\n"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File: \ufefflib/a.dart\ufeff\n+import 'package:vendor_sdk/x'\n"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File: lib/a.dart\u180e\n+import 'package:vendor_sdk/x'\n"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File: lib/a\u2028.dart\n+import 'package:vendor_sdk/x'\n"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File: \n+import 'package:vendor_sdk/x'\n"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Delete File: lib/a.dart\n*** Add File: lib/b.dart\n+import 'package:vendor_sdk/x'"},"agent_type":"planner"}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Update File: lib/a.dart\n*** Move to: test/a.dart\n+import 'package:vendor_sdk/x'"}}`,
		`{"tool_name":"apply_patch","tool_input":{"input":"*** Add File: lib/a.dart\n+import 'package:vendor_sdk/x'"}}`,
		`{"tool_name":"apply_patch","tool_input":{"patch":"*** Add File: lib/a.dart\n+import 'package:vendor_sdk/x'"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":null,"patch":"*** Add File: lib/a.dart\n+import 'package:vendor_sdk/x'"}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":["a\n*** Add File: lib/x.dart\n+import 'package:vendor_sdk/x'"]}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":{"k":"\n*** Add File: lib/x.dart\n+import 'package:vendor_sdk/x'"}}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":12}}`,
		`{"tool_name":"apply_patch","tool_input":{}}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Add File: lib/a.dart\n+x"},"agent_type":3}`,
		`{"tool_name":"apply_patch","tool_input":{"command":"*** Add File: lib/a.dart\n+x"},"agent_type":"reviewer"}`,
		`{"toolCall":"run_command","tool_name":"Bash","tool_input":{"command":"git push"}}`,
		`{"toolCall":{"name":"run_command","args":{"CommandLine":7}}}`,
		`{"toolCall":{"name":"run_command","args":{"CommandLine":"git push","TargetFile":7}}}`,
		`{"toolCall":{"name":"run_command","args":"git push"}}`,
		`{"toolCall":{"name":"run_command"}}`,
		`{"toolCall":{"name":7,"args":{"CommandLine":"git push"}}}`,
		`{"toolCall":{"args":{"CommandLine":"git push"}}}`,
		`{"toolCall":{"name":"write_to_file","args":{"TargetFile":"lib/a.dart","CodeContent":null}}}`,
		`{"toolCall":{"name":"write_to_file","args":{"TargetFile":"lib/a.dart","CodeContent":["import 'package:vendor_sdk/x'"]}}}`,
		`{"toolCall":{"name":"multi_replace_file_content","args":{"TargetFile":"lib/a.dart","ReplacementChunks":["import 'package:vendor_sdk/x'",{"ReplacementX":"import 'package:vendor_sdk/x'"}]}}}`,
		`{"toolCall":{"name":"multi_replace_file_content","args":{"TargetFile":"lib/a.dart","ReplacementChunks":[{"Replacement":"a","Replacement":"import 'package:vendor_sdk/x'"}]}}}`,
		`{"toolCall":{"name":"multi_replace_file_content","args":{"TargetFile":"lib/a.dart","ReplacementChunks":{"ReplacementContent":"import 'package:vendor_sdk/x'"}}}}`,
		`{"toolCall":{"name":"multi_replace_file_content","args":{"TargetFile":"lib/a.dart","ReplacementChunks":[{"ReplacementContent":7}]}}}`,
		`{"toolCall":{"name":"multi_replace_file_content","args":{"TargetFile":"lib/a.dart"}}}`,
		`{"toolCall":{"name":"view_file","args":{"TargetFile":7}}}`,
		`{"tool_name":"WebFetch","tool_input":{}}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push"},"cwd":7}`,
		`{"tool_name":"Bash","tool_input":{"command":"git push"},"workspacePaths":[7,"/x"]}`,
		`{"tool_name":"Bash","tool_input":{"command":"` + strings.Repeat("$(", 200) + `git push` + strings.Repeat(")", 200) + `"}}`,
		`{"tool_name":"Bash","tool_input":{"command":"` + strings.Repeat(`a\\b \"c\" 'd' `, 300) + `; git push"}}`,
	}
}

// guardCorpus is every differential case: the ported tests as payloads, each seed command under every mutation
// and agent, every write path, role and content under every agent, odd payloads, and seeded random combinations,
// against the policies and the four ways to find a policy.
func guardCorpus() []corpusCase {
	var cases []corpusCase
	add := func(name, stdin string, args ...string) {
		cases = append(cases, corpusCase{name: name, stdin: stdin, args: args})
	}
	policy := func(name string) string { return "{policies}/" + name + ".json" }

	for i, seed := range seedCommands() {
		for _, role := range []string{"", "reviewer"} {
			for j, payload := range shellPayloads(seed, role) {
				add(fmt.Sprintf("seed %d/%s/%d", i, role, j), payload, policy("default"))
				add(fmt.Sprintf("seed %d/%s/%d relaxed", i, role, j), payload, policy("relaxed"))
			}
		}
	}
	for i, seed := range seedCommands() {
		for m, mutate := range shellMutations {
			payloads := shellPayloads(mutate(seed), "")
			add(fmt.Sprintf("mutation %d of seed %d", m, i), payloads[(i+m)%len(payloads)], policy("default"))
		}
	}
	for _, path := range corpusPaths {
		for _, r := range corpusRoles {
			for k, content := range []string{"x", vendorImport} {
				for j, payload := range writePayloads(path, r, content) {
					add(fmt.Sprintf("write %s/%s/%d/%d", path, r, k, j), payload, policy("default"))
				}
			}
		}
		for k, content := range corpusContents {
			payloads := writePayloads(path, "", content)
			add(fmt.Sprintf("content %s/%d", path, k), payloads[k%len(payloads)], policy("default"))
			add(fmt.Sprintf("content %s/%d relaxed", path, k), payloads[(k+1)%len(payloads)], policy("relaxed"))
			add(fmt.Sprintf("content %s/%d regex", path, k), payloads[(k+2)%len(payloads)], policy("regex-prefix"))
		}
	}
	for i, payload := range oddPayloads() {
		add(fmt.Sprintf("odd %d", i), payload, policy("default"))
	}
	for name := range corpusPolicies {
		for i, payload := range []string{
			`{"tool_name":"Bash","tool_input":{"command":"git push"}}`,
			`{"tool_name":"Bash","tool_input":{"command":"git commit -m x && dart format ."}}`,
			`{"tool_name":"Write","tool_input":{"file_path":"/repo/lib/a.dart","content":"import 'package:vendor_sdk/a.dart';"},"agent_type":"odd"}`,
			`{"tool_name":"Write","tool_input":{"file_path":"/repo/lib/a.dart","content":"import 'package:vendor_sdk/a.dart';"},"agent_type":"none"}`,
			`{"tool_name":"Write","tool_input":{"file_path":"/repo/lib/a.dart","content":"import 'package:late/a.dart';"},"agent_type":"reviewer"}`,
			`{"tool_name":"Write","tool_input":{"file_path":"/repo/lib/a.dart","content":"import 'package:late/a.dart';"}}`,
			`{"tool_name":"Write","tool_input":{"file_path":"/repo/other/lib/a.dart","content":"x"}}`,
			`{"tool_name":"Write","tool_input":{"file_path":"/repo/core/lib/a.dart","content":"x"}}`,
			`{"tool_name":"Write","tool_input":{"file_path":"/repo/.ai/plans/a.md","content":"x"},"agent_type":"planner"}`,
			`{"toolCall":{"name":"run_command","args":{"CommandLine":"git push"}}}`,
		} {
			add(fmt.Sprintf("policy %s/%d", name, i), payload, policy(name))
		}
	}

	push := `"tool_input":{"command":"git push"}`
	cases = append(cases,
		corpusCase{name: "policy from env", stdin: `{"tool_name":"Bash",` + push + `}`, env: map[string]string{"DASHCREW_GUARD_POLICY": policy("default")}},
		corpusCase{name: "policy from empty env", stdin: `{"tool_name":"Bash",` + push + `}`, env: map[string]string{"DASHCREW_GUARD_POLICY": ""}, inProject: true},
		corpusCase{name: "argument beats env", stdin: `{"tool_name":"Bash",` + push + `}`, args: []string{policy("relaxed")}, env: map[string]string{"DASHCREW_GUARD_POLICY": policy("not-json")}},
		corpusCase{name: "policy from hook cwd", stdin: `{"tool_name":"Bash",` + push + `,"cwd":"{project}"}`},
		corpusCase{name: "policy from hook cwd with no policy", stdin: `{"tool_name":"Bash",` + push + `,"cwd":"{project}/lib"}`, inProject: true},
		corpusCase{name: "policy from null cwd and workspace", stdin: `{"tool_name":"Bash",` + push + `,"cwd":null,"workspacePaths":["{project}"]}`},
		corpusCase{name: "policy from workspace", stdin: `{"toolCall":{"name":"run_command","args":{"CommandLine":"git push"}},"workspacePaths":["{project}","/x"]}`},
		corpusCase{name: "policy from empty workspace and process dir", stdin: `{"toolCall":{"name":"run_command","args":{"CommandLine":"git push"}},"workspacePaths":[]}`, inProject: true},
		corpusCase{name: "policy from process dir", stdin: `{"tool_name":"Bash",` + push + `}`, inProject: true},
		corpusCase{name: "no policy in process dir", stdin: `{"tool_name":"Bash",` + push + `}`},
		corpusCase{name: "policy path is a folder", stdin: `{"tool_name":"Bash",` + push + `}`, args: []string{"{policies}"}},
		corpusCase{name: "policy path missing", stdin: `{"tool_name":"Bash",` + push + `}`, args: []string{"{policies}/missing.json"}},
		corpusCase{name: "cwd of wrong type with policy argument", stdin: `{"tool_name":"Bash",` + push + `,"cwd":7}`, args: []string{policy("default")}},
		corpusCase{name: "cwd of wrong type", stdin: `{"tool_name":"Bash",` + push + `,"cwd":7}`, inProject: true},
		corpusCase{name: "extra arguments", stdin: `{"tool_name":"Bash",` + push + `}`, args: []string{policy("default"), "extra"}},
	)

	random := rand.New(rand.NewSource(4))
	seeds := seedCommands()
	for i := 0; i < 600; i++ {
		script := seeds[random.Intn(len(seeds))]
		for n := random.Intn(3) + 1; n > 0; n-- {
			script = shellMutations[random.Intn(len(shellMutations))](script)
		}
		payloads := shellPayloads(script, corpusRoles[random.Intn(len(corpusRoles))])
		add(fmt.Sprintf("random %d", i), payloads[random.Intn(len(payloads))], policy([]string{"default", "relaxed"}[random.Intn(2)]))
	}
	return cases
}

// hookObject decodes a corpus payload for in-process checks; nil when it is not a JSON object.
func hookObject(stdin string) *omap.Map[any] {
	value, err := dartjson.Decode(stdin)
	if err != nil {
		return nil
	}
	object, _ := value.(*omap.Map[any])
	return object
}
