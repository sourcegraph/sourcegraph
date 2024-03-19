package reader

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"runtime"
	"sync"
)

type Pair struct {
	Element Element
	Err     error
}

// Read reads the given content as line-separated JSON objects and returns a channel of Pair values for each
// non-empty line.
func Read(ctx context.Context, r io.Reader) <-chan Pair {
	interner := NewInterner()

	return readLines(ctx, r, func(line []byte) (Element, error) {
		return unmarshalElement(interner, line)
	})
}

// LineBufferSize is the maximum size of the buffer used to read each line of a raw LSIF index. Lines in
// LSIF can get very long as it include escaped hover text (package documentation), as well as large edges
// such as the contains edge of large documents.
//
// This corresponds a 10MB buffer that can accommodate 10 million characters.
const LineBufferSize = 1e7

// ChannelBufferSize is the number sources lines that can be read ahead of the correlator.
const ChannelBufferSize = 512

// NumUnmarshalGoRoutines is the number of goroutines launched to unmarshal individual lines.
var NumUnmarshalGoRoutines = runtime.GOMAXPROCS(0)

// readLines reads the given content as line-separated objects which are unmarshallable by the given function
// and returns a channel of Pair values for each non-empty line.
func readLines(ctx context.Context, r io.Reader, unmarshal func(line []byte) (Element, error)) <-chan Pair {
	scanner := bufio.NewScanner(r)
	scanner.Split(bufio.ScanLines)
	scanner.Buffer(make([]byte, LineBufferSize), LineBufferSize)

	// Pool of buffers used to transfer copies of the scanner slice to unmarshal workers
	pool := sync.Pool{New: func() any { return new(bytes.Buffer) }}

	// Read the document in a separate go-routine.
	lineCh := make(chan *bytes.Buffer, ChannelBufferSize)
	go func() {
		defer close(lineCh)

		for scanner.Scan() {
			if line := scanner.Bytes(); len(line) != 0 {
				buf := pool.Get().(*bytes.Buffer)
				_, _ = buf.Write(line)

				select {
				case lineCh <- buf:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	pairCh := make(chan Pair, ChannelBufferSize)
	go func() {
		defer close(pairCh)

		// Unmarshal workers receive work assignments as indices into a shared
		// slice and put the result into the same index in a second shared slice.
		work := make(chan int, NumUnmarshalGoRoutines)
		defer close(work)

		// Each unmarshal worker sends a zero-length value on this channel
		// to signal completion of a unit of work.
		signal := make(chan struct{}, NumUnmarshalGoRoutines)
		defer close(signal)

		// The input slice
		lines := make([]*bytes.Buffer, NumUnmarshalGoRoutines)

		// The result slice
		pairs := make([]Pair, NumUnmarshalGoRoutines)

		for range NumUnmarshalGoRoutines {
			go func() {
				for idx := range work {
					element, err := unmarshal(lines[idx].Bytes())
					pairs[idx].Element = element
					pairs[idx].Err = err
					signal <- struct{}{}
				}
			}()
		}

		done := false
		for !done {
			i := 0

			// Read a new "batch" of lines from the reader routine and fill the
			// shared array. Each index that receives a new value is queued in
			// the unmarshal worker channel and can be immediately processed.
			for i < NumUnmarshalGoRoutines {
				line, ok := <-lineCh
				if !ok {
					done = true
					break
				}

				lines[i] = line
				work <- i
				i++
			}

			// Wait until the current batch has been completely unmarshalled
			for range i {
				<-signal
			}

			// Return each buffer to the pool for reuse
			for j := range i {
				lines[j].Reset()
				pool.Put(lines[j])
			}

			// Read the result array in order. If the caller context has completed,
			// we'll abandon any additional values we were going to send on this
			// channel (as well as any additional errors from the scanner).
			for j := range i {
				select {
				case pairCh <- pairs[j]:
				case <-ctx.Done():
					return
				}
			}
		}

		// If there was an error reading from the source, output it here
		if err := scanner.Err(); err != nil {
			pairCh <- Pair{Err: err}
		}
	}()

	return pairCh
}
