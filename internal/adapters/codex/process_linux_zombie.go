//go:build linux

package codex

import (
	"fmt"
	"os"
	"strings"
)

func processZombieLinux(pid int) bool {
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
