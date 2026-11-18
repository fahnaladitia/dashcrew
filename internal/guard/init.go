package guard

import (
	"regexp"

	"github.com/fahnaladitia/dashcrew/internal/dartjson"
)

// InitPolicyPath is where `dashcrew guard init` and sync keep the policy.
const InitPolicyPath = ".ai/guard.json"

// InitPolicy is the policy `dashcrew guard init` writes without a profile: what a profile with every `policy`
// default compiles to, without a platform's rules, roles, packs or local packages.
func InitPolicy() Policy {
	blocked := map[string]bool{"commit": true}
	for _, sub := range AlwaysBlockedGit {
		blocked[sub] = true
	}
	return Policy{
		BlockedGit:       blocked,
		BlockFormatters:  true,
		PlansDir:         ".ai/plans",
		RoleCapability:   map[string]string{},
		ForbiddenImports: []string{},
		LocalPackages:    []string{},
	}
}

// InitPolicyText is `.ai/guard.json` as `dashcrew guard init` writes it.
func InitPolicyText() string {
	text, err := dartjson.EncodeIndent(InitPolicy().ToJSON(), "  ")
	if err != nil {
		panic(err) // the policy holds only strings, bools, lists and maps
	}
	return text + "\n"
}

var shimNeed = regexp.MustCompile(`(?m)^need=(\S+)$`)

// ShimMinimum is the oldest dashcrew a shim accepts, from its `need=` line; empty when it has none.
func ShimMinimum(shim string) string {
	if m := shimNeed.FindStringSubmatch(shim); m != nil {
		return m[1]
	}
	return ""
}

// WrittenByInit is true when content is exactly what `dashcrew guard init` writes at path: the default policy, or
// the shim for the version its own `need=` line names. Sync takes such a file over as its own.
func WrittenByInit(path, content string) bool {
	switch path {
	case InitPolicyPath:
		return content == InitPolicyText()
	case ShimPath:
		minimum := ShimMinimum(content)
		return minimum != "" && content == Shim(minimum)
	}
	return false
}
