//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris
// +build aix darwin dragonfly freebsd linux netbsd openbsd solaris

package output

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func init() {
	// The platforms this file builds on support the SIGWINCH signal, which
	// indicates that the terminal has been resized. When we receive that
	// signal, we can use this to re-detect the terminal capabilities.
	//
	// We won't do any setup until the first time newCapabilityWatcher is
	// invoked, but we do need some shared state to be ready.
	var (
		// chans contains the listening channels that should be notified when
		// capabilities are updated.
		chans []chan capabilities

		// mu guards the chans variable.
		mu sync.RWMutex

		// once guards the lazy initialisation, including installing the signal
		// handler.
		once sync.Once
	)

	newCapabilityWatcher = func(opts OutputOpts) chan capabilities {
		// Lazily initialise the required global state if we haven't already.
		once.Do(func() {
			mu.Lock()
			chans = make([]chan capabilities, 0, 1)
			mu.Unlock()

			// Install the signal handler. To avoid race conditions, we should
			// do this synchronously before spawning the goroutine that will
			// actually listen to the channel.
			c := make(chan os.Signal, 1)
			signal.Notify(c, syscall.SIGWINCH)

			go func() {
				for {
					<-c
					caps, err := detectCapabilities(opts)
					// We won't bother reporting an error here; there's no harm
					// in the previous capabilities being used besides possibly
					// being ugly.
					if err == nil {
						mu.RLock()
						for _, out := range chans {
							go func() {
								select {
								case out <- caps:
									// success
								default:
									// welp
								}
							}()
						}
						mu.RUnlock()
					}
				}
			}()
		})

		// Now we can create and return the actual output channel.
		out := make(chan capabilities)
		mu.Lock()
		defer mu.Unlock()
		chans = append(chans, out)

		return out
	}
}
