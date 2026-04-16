//go:build linux

package proc

import (
	"fmt"
	"os"
	"strings"
)

// environPlatform reads the environment variables of a process on Linux from
// /proc/<pid>/environ (null-separated KEY=VALUE entries).
func environPlatform(pid int32) ([]string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var envs []string
	for _, s := range strings.Split(strings.TrimRight(string(data), "\x00"), "\x00") {
		if s != "" {
			envs = append(envs, s)
		}
	}
	return envs, nil
}
