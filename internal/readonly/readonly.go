// Package readonly enforces TELE_READ_ONLY across the process tree on Linux so
// a child shell cannot bypass read-only mode by unsetting the variable.
package readonly

import (
	"os"
	"strings"
)

const EnvVar = "TELE_READ_ONLY"

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

// Active reports whether TELE_READ_ONLY is enabled in the current process or,
// on Linux, in any ancestor process environment.
func Active() bool {
	if isTruthy(os.Getenv(EnvVar)) {
		return true
	}
	return ancestorActive()
}
