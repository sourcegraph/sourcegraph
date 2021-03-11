package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

type WriterStat struct {
	Event    string
	Bytes    int
	Duration time.Duration
	Error    error
}

type Writer struct {
	w     io.Writer
	flush func()

	StatHook func(WriterStat)
}

func NewWriter(w http.ResponseWriter) (*Writer, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("http flushing not supported")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// This informs nginx to not buffer. With buffering search responses will
	// be delayed until buffers get full, leading to worst case latency of the
	// full time a search takes to complete.
	w.Header().Set("X-Accel-Buffering", "no")

	return &Writer{
		w:     w,
		flush: flusher.Flush,
	}, nil
}

// Event writes event with data json marshalled.
func (e *Writer) Event(event string, data interface{}) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return e.EventBytes(event, encoded)
}

// EventBytes writes dataLine as an event. dataLine is not allowed to contain
// a newline.
func (e *Writer) EventBytes(event string, dataLine []byte) (err error) {
	// write is a helper to avoid error handling. Additionally it counts the
	// number of bytes written.
	start := time.Now()
	bytes := 0
	write := func(b []byte) {
		if err != nil {
			return
		}
		var n int
		n, err = e.w.Write(b)
		bytes += n
	}

	defer func() {
		if hook := e.StatHook; hook != nil {
			hook(WriterStat{
				Event:    event,
				Bytes:    bytes,
				Duration: time.Since(start),
				Error:    err,
			})
		}
	}()

	if event != "" {
		// event: $event\n
		write([]byte("event: "))
		write([]byte(event))
		write([]byte("\n"))
	}

	// data: json($data)\n\n
	write([]byte("data: "))
	write(dataLine)
	write([]byte("\n\n"))

	e.flush()

	return err
}
