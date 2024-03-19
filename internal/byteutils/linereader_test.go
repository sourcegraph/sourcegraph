package byteutils_test

import (
	"bytes"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/byteutils"
)

func TestNewLineReader(t *testing.T) {
	data := []byte("hello\nworld\n")
	reader := byteutils.NewLineReader(data)

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	if got, want := reader.Line(), []byte("hello"); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	if got, want := reader.Line(), []byte("world"); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if reader.Scan() {
		t.Error("expected scan to fail, no more lines")
	}
}

func TestNewLineReaderNoFinalNewline(t *testing.T) {
	data := []byte("hello world\nhello sourcegraph")
	reader := byteutils.NewLineReader(data)

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	if got, want := reader.Line(), []byte("hello world"); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	if got, want := reader.Line(), []byte("hello sourcegraph"); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if reader.Scan() {
		t.Error("expected scan to fail, no more lines")
	}
}

func TestNewLineReaderEmptyLines(t *testing.T) {
	data := []byte("\n\n\n")
	reader := byteutils.NewLineReader(data)

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	if got, want := reader.Line(), []byte(""); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	if got, want := reader.Line(), []byte(""); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	if got, want := reader.Line(), []byte(""); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if reader.Scan() {
		t.Error("expected scan to fail, no more lines")
	}
}

func TestLineReaderNoCopy(t *testing.T) {
	data := []byte("hello world\n")
	reader := byteutils.NewLineReader(data)

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	got, want := reader.Line(), []byte("hello world")
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if reader.Scan() {
		t.Error("expected scan to fail, no more lines")
	}

	// Test that modifying the data in the array backing the scanned line does
	// modify the original data, just like with bytes.Split. We do _not_ copy
	// the data, just create a subslice.
	got[1] = 'a'

	if got, want := data, []byte("hallo world\n"); !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLineReaderConsecutiveScans(t *testing.T) {
	data := []byte("hello\nworld\n")
	reader := byteutils.NewLineReader(data)

	if !reader.Scan() {
		t.Error("expected scan to succeed")
	}
	got, want := reader.Line(), []byte("hello")
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	if !reader.Scan() {
		t.Error("expected scan to pass")
	}

	// Check that got is unmodified, it should still point to the old line.
	if !bytes.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func BenchmarkNewLineReader(b *testing.B) {
	data := []byte("hello\nworld\nhello\nworld\nhello\nworld\n")
	for range b.N {
		reader := byteutils.NewLineReader(data)
		for reader.Scan() {
			l := reader.Line()
			_ = l
		}
	}
	b.ReportAllocs()
}

func BenchmarkBytesSplit(b *testing.B) {
	data := []byte("hello\nworld\nhello\nworld\nhello\nworld\n")
	for range b.N {
		b := bytes.Split(data, []byte("\n"))
		_ = b
	}
	b.ReportAllocs()
}

func BenchmarkNewLineReaderLongLine(b *testing.B) {
	data := make([]byte, 0, 10000*12)
	for range 10000 {
		data = append(data, []byte("hello world")...)
	}
	b.ResetTimer()
	for range b.N {
		reader := byteutils.NewLineReader(data)
		for reader.Scan() {
			l := reader.Line()
			_ = l
		}
	}
	b.ReportAllocs()
}

func BenchmarkBytesSplitLongLine(b *testing.B) {
	data := make([]byte, 0, 10000*12)
	for range 10000 {
		data = append(data, []byte("hello world")...)
	}
	b.ResetTimer()
	for range b.N {
		b := bytes.Split(data, []byte("\n"))
		_ = b
	}
	b.ReportAllocs()
}

func BenchmarkNewLineReaderManyLines(b *testing.B) {
	data := make([]byte, 0, 10000*12)
	for range 10000 {
		data = append(data, []byte("hello world")...)
		data = append(data, []byte("\n")...)
	}
	b.ResetTimer()
	for range b.N {
		reader := byteutils.NewLineReader(data)
		for reader.Scan() {
			l := reader.Line()
			_ = l
		}
	}
	b.ReportAllocs()
}

func BenchmarkBytesSplitManyLines(b *testing.B) {
	data := make([]byte, 0, 10000*12)
	for range 10000 {
		data = append(data, []byte("hello world")...)
		data = append(data, []byte("\n")...)
	}
	b.ResetTimer()
	for range b.N {
		b := bytes.Split(data, []byte("\n"))
		_ = b
	}
	b.ReportAllocs()
}
