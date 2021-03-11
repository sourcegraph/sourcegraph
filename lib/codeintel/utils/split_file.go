package codeintelutils

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"

	"github.com/hashicorp/go-multierror"
)

// SplitFile writes the contents of the given file into a series of temporary files, each of which
// are gzipped and are no larger than the given max payload size. The file names are returned in the
// order in which they were written. The cleanup function removes all temporary files, and wraps the
// error argument with any additional errors that happen during cleanup.
func SplitFile(filename string, maxPayloadSize int) (files []string, _ func(error) error, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	return SplitReaderIntoFiles(f, maxPayloadSize)
}

// SplitReaderIntoFiles writes the contents of the given reader into a series of temporary files, each of which
// are gzipped and are no larger than the given max payload size. The file names are returned in the
// order in which they were written. The cleanup function removes all temporary files, and wraps the
// error argument with any additional errors that happen during cleanup.
func SplitReaderIntoFiles(r io.ReaderAt, maxPayloadSize int) (files []string, _ func(error) error, err error) {
	cleanup := func(err error) error {
		for _, file := range files {
			if removeErr := os.Remove(file); removeErr != nil {
				err = multierror.Append(err, removeErr)
			}
		}

		return err
	}
	defer func() {
		if err != nil {
			err = cleanup(err)
		}
	}()

	// Create a function that returns a reader of the "next" chunk on each invocation.
	makeNextReader := SplitReader(r, maxPayloadSize)

	for {
		partFile, err := ioutil.TempFile("", "")
		if err != nil {
			return nil, nil, err
		}
		defer partFile.Close()

		n, err := io.Copy(partFile, makeNextReader())
		if err != nil {
			return nil, nil, err
		}

		if n == 0 {
			// File must be closed before it's removed (on Windows), so
			// don't wait for the defer above. Double closing a file is
			// fine, but the second close will return an ErrClosed value,
			// which we aren't checking anyway.
			_ = partFile.Close()

			// Edge case: previous io.CopyN call returned err=nil and
			// n=maxPayloadSize. Nothing written to this file so we
			// can just undo its creation and fall-through to the break.
			if err := os.Remove(partFile.Name()); err != nil {
				return nil, nil, err
			}

			// Reader was empty, nothing left to do in this loop
			break
		}

		files = append(files, partFile.Name())
	}

	return files, cleanup, nil
}

// SplitReader returns a function that returns readers that return a slice of the reader
// at maxPayloadSize bytes long. Each reader returned will operate on the next sequential
// slice from the source reader.
func SplitReader(r io.ReaderAt, maxPayloadSize int) func() io.Reader {
	offset := 0

	return func() io.Reader {
		pr, pw := io.Pipe()

		go func() {
			n, err := readAtN(pw, r, offset, maxPayloadSize)
			offset += n
			pw.CloseWithError(err)
		}()

		return pr
	}
}

// readAtN reads n bytes (or until EOF) from the given ReaderAt starting at the given offset.
// Each chunk read from the reader is written to the writer. Errors are forwarded from both
// the reader and writer.
func readAtN(w io.Writer, r io.ReaderAt, offset, n int) (int, error) {
	buf := make([]byte, 32*1024) // same as used by io.Copy/copyBuffer

	totalRead := 0
	for n > 0 {
		if n < len(buf) {
			buf = buf[:n]
		}

		read, readErr := r.ReadAt(buf, int64(offset+totalRead))
		if readErr != nil && readErr != io.EOF {
			return totalRead, readErr
		}

		n -= read
		totalRead += read

		if _, writeErr := io.Copy(w, bytes.NewReader(buf[:read])); writeErr != nil {
			return totalRead, writeErr
		}
		if readErr != nil {
			return totalRead, readErr
		}
	}

	return totalRead, nil
}
