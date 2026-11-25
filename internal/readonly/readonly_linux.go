//go:build linux

package readonly

import (
	"bytes"
	"os"
	"strconv"
	"strings"
)

const maxAncestorDepth = 64

func ancestorActive() bool {
	pid := os.Getppid()
	for depth := 0; depth < maxAncestorDepth && pid >= 1; depth++ {
		if procEnvTruthy(pid) {
			return true
		}
		if pid == 1 {
			break
		}
		ppid, ok := parentPID(pid)
		if !ok || ppid == pid {
			break
		}
		pid = ppid
	}
	return false
}

func procEnvTruthy(pid int) bool {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/environ")
	if err != nil {
		return false
	}
	prefix := []byte(EnvVar + "=")
	for _, entry := range bytes.Split(raw, []byte{0}) {
		if bytes.HasPrefix(entry, prefix) {
			if isTruthy(string(entry[len(prefix):])) {
				return true
			}
		}
	}
	return false
}

func parentPID(pid int) (int, bool) {
	raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, false
	}
	// Comm may contain spaces or parentheses; the fields after the last ')' start
	// with state (index 0) and ppid (index 1).
	s := string(raw)
	idx := strings.LastIndexByte(s, ')')
	if idx < 0 || idx+2 >= len(s) {
		return 0, false
	}
	fields := strings.Fields(s[idx+2:])
	if len(fields) < 2 {
		return 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil || ppid <= 0 {
		return 0, false
	}
	return ppid, true
}
