package guard

import (
	"os"
	"path/filepath"

	"github.com/fahnaladitia/dashcrew/internal/omap"
	"github.com/fahnaladitia/dashcrew/internal/pathstyle"
)

// projects picks the policy for each event (P9): a file event in another dashcrew project than the policy's, such
// as a dependency linked with `dashcrew link`, is checked under that project's `.ai/guard.json`. When the hook
// runs in a folder that is no project but holds projects one level down, shell events are checked under each of
// them and blocked when any blocks, and file events under the project holding the path.
type projects struct {
	k    *knowledgeRoots
	home string
	// policy and project are the hook's policy and the project whose `.ai/guard.json` it is; policy is nil in a
	// workspace folder, project is empty for a policy stored elsewhere.
	policy  *Policy
	project string
	// workspace is the folder above several projects; empty unless policy is nil.
	workspace string
	// perPath is false for a policy stored outside any project, which keeps the single-policy check.
	perPath bool
	loaded  map[string]*Policy
}

func newProjects(hookDir, home string) *projects {
	return &projects{k: newKnowledgeRoots(pathstyle.Host, osFS{}, hookDir, "", home, ""), home: home, loaded: map[string]*Policy{}}
}

// evaluateAll blocks when any event is blocked under its policy, with the first reason; see Evaluate for errors.
func (r *projects) evaluateAll(events []Event) (Decision, error) {
	for _, event := range events {
		decision, err := r.evaluate(event)
		if err != nil || decision.Blocked {
			return decision, err
		}
	}
	return Allow(), nil
}

func (r *projects) evaluate(event Event) (Decision, error) {
	if r.perPath && (event.Kind == Write || event.Kind == Edit) && event.FilePath != "" {
		real := r.k.realPath(r.k.expand(event.FilePath, r.k.cwd))
		if owner := r.ownerOf(real); owner != "" && (r.project == "" || !r.k.style.Same(owner, r.k.realPath(r.project))) {
			if p, ok := r.load(owner, owner); ok {
				event.FilePath = real
				return Evaluate(event, *p)
			}
		}
	}
	if r.policy != nil {
		return Evaluate(event, *r.policy)
	}
	if event.Kind == Shell {
		for _, p := range r.below() {
			if decision, err := Evaluate(event, *p); err != nil || decision.Blocked {
				return decision, err
			}
		}
	}
	return Allow(), nil
}

// ownerOf is the nearest folder holding path that has `.ai/guard.json`; empty when none does.
func (r *projects) ownerOf(path string) string {
	s := r.k.style
	for dir := path; ; {
		if exists, _ := r.k.fsys.lstat(s.Join(dir, ".ai", "guard.json")); exists {
			return dir
		}
		parent := s.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// below are the policies of the projects one level under the workspace folder, in name order.
func (r *projects) below() []*Policy {
	if r.workspace == "" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(r.workspace, "*", ".ai", "guard.json"))
	var out []*Policy
	for _, match := range matches {
		if p, ok := r.load(filepath.Dir(filepath.Dir(match)), r.k.cwd); ok {
			out = append(out, p)
		}
	}
	return out
}

// load reads project's `.ai/guard.json`, answering for that project with relative paths read from cwd: the project
// for a file event, the hook's folder for a shell event. False when it cannot be read, which leaves the event to
// the hook's own policy as before P9.
func (r *projects) load(project, cwd string) (*Policy, bool) {
	key := project + "\x00" + cwd
	if p, seen := r.loaded[key]; seen {
		return p, p != nil
	}
	r.loaded[key] = nil
	file, err := os.Open(filepath.Join(project, ".ai", "guard.json"))
	if err != nil {
		return nil, false
	}
	value, err := readJSON(file)
	file.Close()
	if err != nil {
		return nil, false
	}
	if _, ok := value.(*omap.Map[any]); !ok {
		return nil, false
	}
	p, err := LoadPolicy(value)
	if err != nil {
		return nil, false
	}
	p = p.at(cwd, project, r.home)
	r.loaded[key] = &p
	return &p, true
}
