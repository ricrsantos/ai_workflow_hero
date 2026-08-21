//go:build !linux

package codex

import "syscall"

func parentDeathSignalAttr() *syscall.SysProcAttr {
	return nil
}

func processZombie(pid int) bool {
	_ = pid
	return false
}
