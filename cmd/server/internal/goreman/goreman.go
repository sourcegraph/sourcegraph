// Package goreman implements a process supervisor for a Procfile.
package goreman

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// -- process information structure.
type procInfo struct {
	proc    string
	cmdline string
	stopped bool // true if we stopped it
	cmd     *exec.Cmd
	mu      sync.Mutex
	cond    *sync.Cond
	waitErr error
}

// process informations named with proc.
var procs map[string]*procInfo

var maxProcNameLength int

// read Procfile and parse it.
func readProcfile(content []byte) error {
	procs = map[string]*procInfo{}
	re := regexp.MustCompile(`\$([a-zA-Z]+[a-zA-Z0-9_]+)`)
	for _, line := range strings.Split(string(content), "\n") {
		tokens := strings.SplitN(line, ":", 2)
		if len(tokens) != 2 || tokens[0][0] == '#' {
			continue
		}
		k, v := strings.TrimSpace(tokens[0]), strings.TrimSpace(tokens[1])
		if runtime.GOOS == "windows" {
			v = re.ReplaceAllStringFunc(v, func(s string) string {
				return "%" + s[1:] + "%"
			})
		}
		p := &procInfo{proc: k, cmdline: v}
		p.cond = sync.NewCond(&p.mu)
		procs[k] = p
		if len(k) > maxProcNameLength {
			maxProcNameLength = len(k)
		}
	}
	if len(procs) == 0 {
		return errors.New("No valid entry")
	}
	return nil
}

type Options struct {
	// RPCAddr is the address to listen for Goreman RPCs.
	RPCAddr string
}

// Start starts up the Procfile.
func Start(contents []byte, opts Options) error {
	err := readProcfile(contents)
	if err != nil {
		return err
	}
	if opts.RPCAddr != "" {
		if err := os.Setenv("GOREMAN_RPC_ADDR", opts.RPCAddr); err != nil {
			return err
		}
		if err := startServer(opts.RPCAddr); err != nil {
			return err
		}
	}
	startProcs()
	return waitProcs()
}
