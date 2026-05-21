package tmuxhooks

import (
	"fmt"
	"strings"
)

// fakeTmux records Run calls and returns scripted Output responses.
type fakeTmux struct {
	outputs map[string]string // key: space-joined args -> stdout
	errs    map[string]error  // key: space-joined args -> error
	runs    [][]string        // every Run call, in order
}

func newFakeTmux() *fakeTmux {
	return &fakeTmux{outputs: map[string]string{}, errs: map[string]error{}}
}

func key(args []string) string { return strings.Join(args, " ") }

func (f *fakeTmux) Run(args ...string) error {
	f.runs = append(f.runs, append([]string(nil), args...))
	if err, ok := f.errs[key(args)]; ok {
		return err
	}
	return nil
}

func (f *fakeTmux) Output(args ...string) (string, error) {
	k := key(args)
	if err, ok := f.errs[k]; ok {
		return "", err
	}
	out, ok := f.outputs[k]
	if !ok {
		return "", fmt.Errorf("fakeTmux: no scripted output for %q", k)
	}
	return out, nil
}

// runStrings returns every recorded Run call as a space-joined string.
func (f *fakeTmux) runStrings() []string {
	var out []string
	for _, r := range f.runs {
		out = append(out, strings.Join(r, " "))
	}
	return out
}

var _ Tmux = (*fakeTmux)(nil)
