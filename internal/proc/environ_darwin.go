//go:build darwin

package proc

import (
	"bytes"
	"encoding/binary"
	"strings"

	"golang.org/x/sys/unix"
)

// environPlatform reads the environment variables of a process on macOS by
// parsing the kern.procargs2 sysctl output. The format is:
//
//	argc (int32) | exe_path\0 | argv[1]\0 ... argv[argc-1]\0 | env[0]\0 ... env[m]\0
func environPlatform(pid int32) ([]string, error) {
	data, err := unix.SysctlRaw("kern.procargs2", int(pid))
	if err != nil {
		return nil, err
	}
	if len(data) < 4 {
		return nil, nil
	}

	nargs := int(binary.LittleEndian.Uint32(data[:4]))
	// Split the remainder on null bytes.
	parts := bytes.Split(data[4:], []byte{0})

	// parts[0] = executable path (skip)
	// parts[1..nargs-1] = argv[1..argc-1] (skip); parts[nargs] = null separator (absorbed by i<=nargs)
	// parts[nargs+1..] = environment variables
	var envs []string
	for i, part := range parts {
		if i == 0 || i <= nargs {
			continue // skip exe path and argv
		}
		s := string(part)
		if s == "" || !strings.Contains(s, "=") {
			continue // skip empty separators and non-env entries
		}
		envs = append(envs, s)
	}
	return envs, nil
}
