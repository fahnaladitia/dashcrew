package version

import "testing"

func TestCurrentIsUnstampedInTestsSoTheGoldensStayFixed(t *testing.T) {
	if Version != "" || Current() != Unstamped {
		t.Errorf("Version %q, Current %q", Version, Current())
	}
}

func TestCurrentPrefersTheStampedVersion(t *testing.T) {
	saved := Version
	t.Cleanup(func() { Version = saved })
	Version = "v1.2.3"
	if got := Current(); got != "1.2.3" {
		t.Errorf("Current() = %q", got)
	}
}

func TestFromModuleTakesReleaseAndPseudoVersionsOnly(t *testing.T) {
	cases := map[string]string{
		"v0.1.0":                                   "0.1.0",
		"v0.1.1-0.20261101062238-4b1065d35c59":     "0.1.1-0.20261101062238-4b1065d35c59",
		"v0.0.0-20261101062238-4b1065d35c59+dirty": "0.0.0-20261101062238-4b1065d35c59+dirty",
		"(devel)": Unstamped,
		"":        Unstamped,
		"0.1.0":   Unstamped,
		"v1.2":    Unstamped,
	}
	for module, want := range cases {
		if got := fromModule(module); got != want {
			t.Errorf("fromModule(%q) = %q, want %q", module, got, want)
		}
	}
}

func TestGuardMinimumIsNewerThanUnstampedSoTheDartEngineIsRefused(t *testing.T) {
	if Compare(GuardMinimum, Unstamped) <= 0 || !IsSemantic(GuardMinimum) {
		t.Errorf("GuardMinimum %s, Unstamped %s", GuardMinimum, Unstamped)
	}
}

func TestIsSemantic(t *testing.T) {
	for version, want := range map[string]bool{
		"0.1.0": true, "10.20.30": true, "0.1.0-dev.1": true, "0.1.0+abc": true, "0.1.1-0.2026-ab+dirty": true,
		"": false, "1.2": false, "1.2.3.4": false, "1..3": false, "v1.2.3": false, "1.2.x": false, "dashcrew": false,
	} {
		if got := IsSemantic(version); got != want {
			t.Errorf("IsSemantic(%q) = %v", version, got)
		}
	}
}

func TestCompareOrdersByMajorMinorPatchAndIgnoresBuildMetadata(t *testing.T) {
	cases := []struct {
		a, b string
		sign int
	}{
		{"0.1.0", "0.0.9", 1},
		{"1.2.3", "1.10.0", -1},
		{"1.2.3+4", "1.2.3", 0},
		{"2.0.0-dev.1", "2.0.0", -1},
		{"2.0.0", "2.0.0-dev.1", 1},
		{"1", "1.0.0", 0},
		{"x.1", "0.1", 0},
	}
	for _, c := range cases {
		got := Compare(c.a, c.b)
		if (got > 0) != (c.sign > 0) || (got < 0) != (c.sign < 0) {
			t.Errorf("Compare(%q, %q) = %d, want sign %d", c.a, c.b, got, c.sign)
		}
	}
}
