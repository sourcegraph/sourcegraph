package codeintelutils

import (
	"io"
	"sync/atomic"
)

type rateReader struct {
	reader io.Reader
	read   int64
	size   int64
}

func newRateReader(r io.Reader, size int64) *rateReader {
	if r == nil {
		return nil
	}
	return &rateReader{
		reader: r,
		size:   size,
	}
}

func (r *rateReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	atomic.AddInt64(&r.read, int64(n))
	return n, err
}

func (r *rateReader) Progress() float64 {
	return float64(atomic.LoadInt64(&r.read)) / float64(r.size)
}
