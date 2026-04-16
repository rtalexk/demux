//go:build !darwin && !linux

package proc

import "errors"

func environPlatform(_ int32) ([]string, error) {
	return nil, errors.New("env: not supported on this platform")
}
