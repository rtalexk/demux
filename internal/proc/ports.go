package proc

import (
    "fmt"
    "os/exec"
    "runtime"
    "strconv"
    "strings"
)

type PortInfo struct {
    Port int
    PID  int32
}

func ListeningPorts() ([]PortInfo, error) {
    if runtime.GOOS == "darwin" {
        out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-n", "-P").Output()
        if err != nil {
            return nil, fmt.Errorf("lsof: %w", err)
        }
        return ParseLsofPorts(string(out))
    }
    out, err := exec.Command("ss", "-tlnp").Output()
    if err != nil {
        return nil, fmt.Errorf("ss: %w", err)
    }
    return parseSsPorts(string(out))
}

func ParseLsofPorts(raw string) ([]PortInfo, error) {
    var ports []PortInfo
    for i, line := range strings.Split(strings.TrimSpace(raw), "\n") {
        if i == 0 || strings.TrimSpace(line) == "" {
            continue // skip header
        }
        fields := strings.Fields(line)
        if len(fields) < 9 {
            continue
        }
        pid, err := strconv.Atoi(fields[1])
        if err != nil {
            continue
        }
        // NAME field is fields[8]: "*:3000" or "127.0.0.1:3000" — strip trailing " (LISTEN)" if present
        name := fields[8]
        colonIdx := strings.LastIndex(name, ":")
        if colonIdx < 0 {
            continue
        }
        portStr := name[colonIdx+1:]
        port, err := strconv.Atoi(portStr)
        if err != nil {
            continue
        }
        ports = append(ports, PortInfo{Port: port, PID: int32(pid)})
    }
    return ports, nil
}

func parseSsPorts(raw string) ([]PortInfo, error) {
    var ports []PortInfo
    for i, line := range strings.Split(strings.TrimSpace(raw), "\n") {
        if i == 0 || strings.TrimSpace(line) == "" {
            continue
        }
        fields := strings.Fields(line)
        if len(fields) < 4 {
            continue
        }
        addr := fields[3]
        colonIdx := strings.LastIndex(addr, ":")
        if colonIdx < 0 {
            continue
        }
        port, err := strconv.Atoi(addr[colonIdx+1:])
        if err != nil {
            continue
        }
        ports = append(ports, PortInfo{Port: port})
    }
    return ports, nil
}
