package doctor

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fahnaladitia/dashcrew/internal/adapters"
	"github.com/fahnaladitia/dashcrew/internal/guard"
	"github.com/fahnaladitia/dashcrew/internal/version"
)

// guardOnly is a project after `dashcrew guard init --agents antigravity`: no profile, no lock.
func guardOnly(t *testing.T) *fixture {
	t.Helper()
	f := setup(t, "")
	f.delete(".ai/project.yaml")
	f.write(guard.InitPolicyPath, guard.InitPolicyText())
	f.write(guard.ShimPath, guard.Shim(version.AntigravityGuardMinimum))
	f.makeExecutable(guard.ShimPath)
	if _, _, err := adapters.MergeGuardHook("antigravity", f.project); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestAGuardOnlyProjectRunsOnlyTheGuardChecks(t *testing.T) {
	f := guardOnly(t)
	f.installDashcrew(version.AntigravityGuardMinimum, 2)

	checks := f.doctor()

	var ids []string
	for id, check := range checks {
		ids = append(ids, id)
		if check.Status == Fail || check.Status == Warn {
			t.Errorf("%s: %s %s", id, check.Status, check.Detail)
		}
	}
	sort.Strings(ids)
	if got, want := strings.Join(ids, " "), "antigravity-hook gitignore guard-binary guard-policy toolkit"; got != want {
		t.Errorf("checks %s, want %s", got, want)
	}
	expect(t, checks, "toolkit", Info, "guard only (dashcrew guard init)", "dashcrew init and dashcrew sync")
}

func TestAGuardOnlyProjectReadsTheMinimumFromItsShim(t *testing.T) {
	f := guardOnly(t)
	f.installGuard()

	expect(t, f.doctor(), "guard-binary", Warn, "older than "+version.AntigravityGuardMinimum)
}

func TestAGuardOnlyProjectPointsAtGuardInit(t *testing.T) {
	f := guardOnly(t)
	f.installDashcrew(version.AntigravityGuardMinimum, 2)
	f.write(".agents/hooks.json", "{}\n")

	expect(t, f.doctor(), "antigravity-hook", Fail, "dashcrew guard init")
	if detail := f.doctor()["antigravity-hook"].Detail; strings.Contains(detail, "dashcrew sync") {
		t.Errorf("a guard-only project is told to run sync: %s", detail)
	}
}

func TestAnEditedPolicyIsNotGuardOnly(t *testing.T) {
	f := guardOnly(t)
	f.write(guard.InitPolicyPath, strings.Replace(guard.InitPolicyText(), `"merge",`, "", 1))

	expect(t, f.doctor(), "profile", Fail)
}

func (f *fixture) makeExecutable(relative string) {
	f.t.Helper()
	if err := os.Chmod(filepath.Join(f.project, filepath.FromSlash(relative)), 0o755); err != nil {
		f.t.Fatal(err)
	}
}
