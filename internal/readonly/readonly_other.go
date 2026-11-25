//go:build !linux

package readonly

func ancestorActive() bool { return false }
