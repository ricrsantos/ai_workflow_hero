//go:build !linux

package opencode

import "syscall"

func parentDeathSignalAttr() *syscall.SysProcAttr {
	return nil
}

func processZombie(pid int) bool {
	return false
}

func listOpenCodeServePIDs() []int {
	return nil
}

func processListenPort(pid, port int) bool {
	_ = pid
	_ = port
	return true
}
