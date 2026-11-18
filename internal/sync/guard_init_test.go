package sync

import (
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// guardInitFiles writes what `dashcrew guard init --agents antigravity` leaves: the default policy, the shim and the hook.
func guardInitFiles(t *testing.T, project string) {
	t.Helper()
	write(t, project, guard.InitPolicyPath, guard.InitPolicyText())
	write(t, project, guard.ShimPath, guard.Shim(version.AntigravityGuardMinimum))
	if _, _, err := adapters.MergeGuardHook("antigravity", project); err != nil {
		t.Fatal(err)
	}
}

func TestSyncTakesOverTheFilesOfGuardInit(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	guardInitFiles(t, project)

	run(t, s, false, false)

	if got := read(t, project, guard.ShimPath); !strings.Contains(got, "need="+version.GuardMinimum+"\n") {
		t.Errorf("the shim was not rewritten for this profile:\n%s", got)
	}
	if got := read(t, project, guard.InitPolicyPath); got == guard.InitPolicyText() {
		t.Error("the policy was not rewritten from the profile")
	}
	if !strings.Contains(read(t, project, ".agents/hooks.json"), adapters.AntigravityGuardKey) {
		t.Error("the hook file guard init merged was dropped")
	}
}

func TestAnEditedGuardInitFileIsStillAConflict(t *testing.T) {
	s, project := newProject(t, defaultProfile)
	guardInitFiles(t, project)
	write(t, project, guard.InitPolicyPath, strings.Replace(guard.InitPolicyText(), `"merge",`, "", 1))

	_, err := s.Run(false, false)
	if err == nil || err.Error() != "Files changed outside dashcrew (rerun with --force to overwrite):\n  "+guard.InitPolicyPath {
		t.Errorf("got %v", err)
	}
}
