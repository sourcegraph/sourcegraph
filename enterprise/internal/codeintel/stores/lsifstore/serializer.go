package lsifstore

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"io"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

func init() {
	gob.Register(&semantic.DocumentData{})
	gob.Register(&semantic.ResultChunkData{})
	gob.Register(&semantic.LocationData{})
}

type serializer struct {
	readers sync.Pool
	writers sync.Pool
}

func newSerializer() *serializer {
	return &serializer{
		readers: sync.Pool{New: func() interface{} { return new(gzip.Reader) }},
		writers: sync.Pool{New: func() interface{} { return gzip.NewWriter(nil) }},
	}
}

// MarshalDocumentData transforms document data into a string of bytes writable to disk.
func (s *serializer) MarshalDocumentData(document semantic.DocumentData) ([]byte, error) {
	return s.withEncoder(func(encoder *gob.Encoder) error { return encoder.Encode(&document) })
}

// MarshalResultChunkData transforms result chunk data into a string of bytes writable to disk.
func (s *serializer) MarshalResultChunkData(resultChunks semantic.ResultChunkData) ([]byte, error) {
	return s.withEncoder(func(encoder *gob.Encoder) error { return encoder.Encode(&resultChunks) })
}

// MarshalLocations transforms a slice of locations into a string of bytes writable to disk.
func (s *serializer) MarshalLocations(locations []semantic.LocationData) ([]byte, error) {
	return s.withEncoder(func(encoder *gob.Encoder) error { return encoder.Encode(&locations) })
}

// withEncoder creates a gob encoded, calls the given function with it, then compressed the encoded output.
func (s *serializer) withEncoder(f func(encoder *gob.Encoder) error) ([]byte, error) {
	gzipWriter := s.writers.Get().(*gzip.Writer)
	defer s.writers.Put(gzipWriter)

	encodeBuf := new(bytes.Buffer)
	if err := f(gob.NewEncoder(encodeBuf)); err != nil {
		return nil, err
	}

	compressBuf := new(bytes.Buffer)
	gzipWriter.Reset(compressBuf)

	if _, err := io.Copy(gzipWriter, encodeBuf); err != nil {
		return nil, err
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return compressBuf.Bytes(), nil
}

// UnmarshalDocumentData is the inverse of MarshalDocumentData.
func (s *serializer) UnmarshalDocumentData(data []byte) (document semantic.DocumentData, err error) {
	err = s.withDecoder(data, func(decoder *gob.Decoder) error { return decoder.Decode(&document) })
	return document, err
}

// UnmarshalResultChunkData is the inverse of MarshalResultChunkData.
func (s *serializer) UnmarshalResultChunkData(data []byte) (resultChunk semantic.ResultChunkData, err error) {
	err = s.withDecoder(data, func(decoder *gob.Decoder) error { return decoder.Decode(&resultChunk) })
	return resultChunk, err
}

// UnmarshalLocations is the inverse of MarshalLocations.
func (s *serializer) UnmarshalLocations(data []byte) (locations []semantic.LocationData, err error) {
	err = s.withDecoder(data, func(decoder *gob.Decoder) error { return decoder.Decode(&locations) })
	return locations, err
}

// withDecoder creates a gob decoder with the given encoded data and calls the given function with it.
func (s *serializer) withDecoder(data []byte, f func(decoder *gob.Decoder) error) (err error) {
	r := s.readers.Get().(*gzip.Reader)
	defer s.readers.Put(r)

	if err := r.Reset(bytes.NewReader(data)); err != nil {
		return err
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil {
			err = multierror.Append(err, closeErr)
		}
	}()

	return f(gob.NewDecoder(r))
}
