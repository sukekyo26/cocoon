package release

import (
	"strconv"
	"strings"
)

// Compare returns 1 if a > b, -1 if a < b, 0 if equal, treating a/b as
// dotted-number versions with an optional leading "v" and an optional
// "-<prerelease>" suffix (e.g. "1.2.3", "v1.2.3", "1.2.3-rc1"). A
// pre-release version compares lower than the same version without one;
// among two pre-releases the suffixes are compared lexicographically.
// Non-numeric numeric components compare as 0, so malformed inputs
// degrade to equal rather than panicking — callers that need strict
// validation should pre-check the inputs.
func Compare(a, b string) int {
	aNums, aPre := parseSemver(a)
	bNums, bPre := parseSemver(b)
	for i := 0; i < len(aNums) || i < len(bNums); i++ {
		var av, bv int
		if i < len(aNums) {
			av = aNums[i]
		}
		if i < len(bNums) {
			bv = bNums[i]
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	// Numeric parts equal — a pre-release is lower than its release.
	switch {
	case aPre == "" && bPre == "":
		return 0
	case aPre == "" && bPre != "":
		return 1
	case aPre != "" && bPre == "":
		return -1
	case aPre > bPre:
		return 1
	case aPre < bPre:
		return -1
	}
	return 0
}

func parseSemver(s string) ([]int, string) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	pre := ""
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, _ := strconv.Atoi(p) //nolint:errcheck // non-numeric → 0 per Compare's contract
		nums = append(nums, n)
	}
	return nums, pre
}
