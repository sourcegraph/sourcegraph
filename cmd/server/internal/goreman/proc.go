package goreman

import (
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	wg      sync.WaitGroup
	signals = make(chan os.Signal, 10)
)

// stop specified proc.
func stopProc(proc string, kill bool) error {
	procM.Lock()
	p, ok := procs[proc]
	procM.Unlock()
	if !ok {
		return errors.New("Unknown proc: " + proc)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil {
		return nil
	}

	p.stopped = true

	if kill {
		err := p.cmd.Process.Kill()
		if err != nil {
			return err
		}
	} else {
		err := terminateProc(proc)
		if err != nil {
			return err
		}
	}

	p.cond.Wait()
	return nil
}

// start specified proc. if proc is started already, return nil.
func startProc(proc string) error {
	procM.Lock()
	p, ok := procs[proc]
	procM.Unlock()
	if !ok {
		return errors.New("Unknown proc: " + proc)
	}

	p.mu.Lock()
	if p.cmd != nil {
		p.mu.Unlock()
		return nil
	}

	wg.Add(1)
	go func() {
		stopped := spawnProc(proc)
		wg.Done()
		p.mu.Unlock()
		if !stopped {
			switch procDiedAction {
			case Shutdown:
				log.Printf("%s died. Shutting down...", proc)
				signals <- syscall.SIGINT
			case Ignore:
				log.Printf("%s died.", proc)
			default:
				log.Fatalf("%s died. Unknown ProcDiedAction %v", proc, procDiedAction)
			}
		}
	}()
	return nil
}

// startProcs starts the processes.
func startProcs() {
	for _, proc := range names() {
		_ = startProc(proc)
	}
}

var waitProcsOnce sync.Once

// waitProcs waits for processes to complete.
func waitProcs() error {
	waitProcsOnce.Do(func() {
		go func() {
			wg.Wait()
			signals <- syscall.SIGINT
		}()
		signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
		<-signals

		stopped := make(chan struct{})
		go func() {
			stopProcs(false)
			close(stopped)
		}()

		// New signal chan to avoid built up buffered signals
		sc2 := make(chan os.Signal, 10)
		signal.Notify(sc2, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

		select {
		case <-sc2:
			// Second signal received, do a hard exit
			stopProcs(true)
		case <-time.NewTimer(10 * time.Second).C:
			// 10 seconds has passed, kill
			stopProcs(true)
		case <-stopped:
			// Happy case, just continue
		}
	})

	return nil
}

func stopProcs(kill bool) {
	// TODO we probably need a well defined order for shutting down, since
	// something may want to finish writing to postgres for example.
	var wg sync.WaitGroup
	for _, proc := range names() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = stopProc(proc, kill)
		}()
	}
	wg.Wait()
}

func names() (names []string) {
	procM.Lock()
	defer procM.Unlock()

	for proc := range procs {
		names = append(names, proc)
	}

	return names
}
