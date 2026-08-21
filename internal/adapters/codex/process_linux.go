//go:build linux

package codex

import (
	"syscall"
)

func parentDeathSignalAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
}

func processZombie(pid int) bool {
	return processZombieLinux(pid)
}
