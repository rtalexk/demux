package proc

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	gops "github.com/shirou/gopsutil/v3/process"
)

// CWD returns the working directory of the given PID.
func CWD(pid int32) (string, error) {
	p, err := gops.NewProcess(pid)
	if err != nil {
		return "", err
	}
	cwd, err := p.Cwd()
	if err == nil && cwd != "" {
		return cwd, nil
	}
	if runtime.GOOS == "darwin" {
		return cwdLsof(pid)
	}
	return "", fmt.Errorf("cwd not available for pid %d", pid)
}

func cwdLsof(pid int32) (string, error) {
	out, err := exec.Command("lsof", "-p", fmt.Sprint(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimPrefix(line, "n"), nil
		}
	}
	return "", fmt.Errorf("cwd not found in lsof output")
}
