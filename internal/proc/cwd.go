package proc

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
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

// CWDAll returns a map of PID -> CWD for all accessible processes using a
// single bulk lsof call on darwin, /proc/{pid}/cwd readlinks on linux,
// falling back to per-process gopsutil on other platforms.
// Use this instead of calling CWD in a loop.
func CWDAll() (map[int32]string, error) {
	switch runtime.GOOS {
	case "darwin":
		return cwdAllLsof()
	case "linux":
		return cwdAllProc()
	default:
		return cwdAllGops()
	}
}

func cwdAllLsof() (map[int32]string, error) {
	out, err := exec.Command("lsof", "-d", "cwd", "-Fpn").Output()
	if err != nil {
		return nil, fmt.Errorf("lsof: %w", err)
	}
	m := make(map[int32]string)
	var currentPID int32
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "p") {
			pid, err := strconv.ParseInt(strings.TrimPrefix(line, "p"), 10, 32)
			if err == nil {
				currentPID = int32(pid)
			}
		} else if strings.HasPrefix(line, "n") && currentPID != 0 {
			m[currentPID] = strings.TrimPrefix(line, "n")
		}
	}
	return m, nil
}

func cwdAllGops() (map[int32]string, error) {
	pids, err := gops.Pids()
	if err != nil {
		return nil, fmt.Errorf("pids: %w", err)
	}
	m := make(map[int32]string, len(pids))
	for _, pid := range pids {
		p, err := gops.NewProcess(pid)
		if err != nil {
			continue
		}
		cwd, err := p.Cwd()
		if err == nil && cwd != "" {
			m[pid] = cwd
		}
	}
	return m, nil
}

// cwdAllProc reads /proc/{pid}/cwd symlinks directly — Linux only.
// Faster than cwdAllGops: no gopsutil allocations, just readdir + readlink.
func cwdAllProc() (map[int32]string, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("readdir /proc: %w", err)
	}
	m := make(map[int32]string, len(entries))
	for _, e := range entries {
		pid, err := strconv.ParseInt(e.Name(), 10, 32)
		if err != nil {
			continue // skip non-numeric entries (e.g. "self", "net")
		}
		link, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
		if err != nil {
			continue // process may have exited between readdir and readlink
		}
		m[int32(pid)] = link
	}
	return m, nil
}
