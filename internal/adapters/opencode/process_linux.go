//go:build linux

package opencode

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func parentDeathSignalAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
}

func processZombie(pid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	i := strings.LastIndex(string(data), ")")
	if i < 0 || i+2 >= len(data) {
		return false
	}
	return data[i+2] == 'Z'
}

func listOpenCodeServePIDs() []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if IsManagedOpenCodeServe(pid) {
			pids = append(pids, pid)
		}
	}
	return pids
}

func processListenPort(pid, port int) bool {
	if pid <= 0 || port <= 0 {
		return false
	}
	inodes, err := tcpListenInodes(port)
	if err != nil || len(inodes) == 0 {
		return false
	}
	fdDir := fmt.Sprintf("/proc/%d/fd", pid)
	fds, err := os.ReadDir(fdDir)
	if err != nil {
		return false
	}
	for _, fd := range fds {
		target, err := os.Readlink(fmt.Sprintf("%s/%s", fdDir, fd.Name()))
		if err != nil {
			continue
		}
		if !strings.HasPrefix(target, "socket:[") {
			continue
		}
		inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
		if inodes[inode] {
			return true
		}
	}
	return false
}

func tcpListenInodes(port int) (map[string]bool, error) {
	want := fmt.Sprintf("%04X", port)
	out := make(map[string]bool)
	f, err := os.Open("/proc/net/tcp")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		local := fields[1]
		state := fields[3]
		inode := fields[9]
		if state != "0A" { // LISTEN
			continue
		}
		parts := strings.Split(local, ":")
		if len(parts) != 2 || parts[1] != want {
			continue
		}
		out[inode] = true
	}
	return out, scanner.Err()
}
