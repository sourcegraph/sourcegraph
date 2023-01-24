// Package blobstore is a service which exposes an S3-compatible API for object storage.
package blobstore

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	sglog "github.com/sourcegraph/log"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Service is the blobstore service. It is an http.Handler.
type Service struct {
	DataDir        string
	Log            log.Logger
	ObservationCtx *observation.Context

	initOnce      sync.Once
	bucketLocksMu sync.Mutex
	bucketLocks   map[string]*sync.RWMutex
}

func (s *Service) init() {
	s.initOnce.Do(func() {
		s.bucketLocks = map[string]*sync.RWMutex{}

		if err := os.MkdirAll(filepath.Join(s.DataDir, "buckets"), os.ModePerm); err != nil {
			s.Log.Fatal("cannot create buckets directory:", sglog.Error(err))
		}
	})
}

// ServeHTTP handles HTTP based search requests
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.init()
	metricRunning.Inc()
	defer metricRunning.Dec()

	err := s.serve(w, r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.Log.Error("serving request", sglog.Error(err))
		fmt.Fprintf(w, "blobstore: error: %v", err)
		return
	}
}

func (s *Service) serve(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	path := strings.FieldsFunc(r.URL.Path, func(r rune) bool { return r == '/' })
	switch r.Method {
	case "PUT":
		switch len(path) {
		case 1:
			// PUT /<bucket>
			// https://docs.aws.amazon.com/AmazonS3/latest/API/API_CreateBucket.html
			if r.ContentLength != 0 {
				return errors.Newf("expected CreateBucket request to have content length 0: %s %s", r.Method, r.URL)
			}
			bucketName := path[0]
			if err := s.createBucket(ctx, bucketName); err != nil {
				if err == ErrBucketAlreadyExists {
					return writeS3Error(w, s3ErrorBucketAlreadyOwnedByYou, bucketName, err, http.StatusConflict)
				}
				return errors.Wrap(err, "createBucket")
			}
			w.WriteHeader(http.StatusOK)
			return nil
		case 2:
			// PUT /<bucket>/<object>
			// https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
			bucketName := path[0]
			objectName := path[1]
			if err := s.putObject(ctx, bucketName, objectName, r.Body); err != nil {
				if err == ErrNoSuchBucket {
					return writeS3Error(w, s3ErrorNoSuchBucket, bucketName, err, http.StatusNotFound)
				}
				return errors.Wrap(err, "putObject")
			}
			return nil
		default:
			return errors.Newf("unsupported method: PUT request: %s", r.URL)
		}
	case "GET":
		if len(path) == 2 && r.URL.Query().Get("x-id") == "GetObject" {
			// GET /<bucket>/<object>?x-id=GetObject
			// https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html
			bucketName := path[0]
			objectName := path[1]

			reader, err := s.getObject(ctx, bucketName, objectName)
			if err != nil {
				if err == ErrNoSuchKey {
					return writeS3Error(w, s3ErrorNoSuchKey, bucketName, err, http.StatusNotFound)
				}
				return errors.Wrap(err, "getObject")
			}
			defer reader.Close()
			w.Header().Set("Content-Type", "application/x-www-form-urlencoded")
			w.WriteHeader(http.StatusOK)
			_, err = io.Copy(w, reader)
			return errors.Wrap(err, "Copy")
		}
		return errors.Newf("unsupported method: unexpected GET request: %s", r.URL)
	default:
		return errors.Newf("unsupported method: unexpected request: %s %s", r.Method, r.URL)
	}
}

var (
	ErrBucketAlreadyExists = errors.New("bucket already exists")
	ErrNoSuchBucket        = errors.New("no such bucket")
	ErrNoSuchKey           = errors.New("no such key")
)

func (s *Service) createBucket(ctx context.Context, name string) error {
	_ = ctx

	// Lock the bucket so nobody can read or write to the same bucket while we create it.
	bucketLock := s.bucketLock(name)
	bucketLock.Lock()
	defer bucketLock.Unlock()

	// Create the bucket storage directory.
	bucketDir := s.bucketDir(name)
	if _, err := os.Stat(bucketDir); err == nil {
		return ErrBucketAlreadyExists
	}

	defer s.Log.Info("created bucket", sglog.String("name", name), sglog.String("dir", bucketDir))
	if err := os.Mkdir(bucketDir, os.ModePerm); err != nil {
		return errors.Wrap(err, "MkdirAll")
	}
	return nil
}

func (s *Service) putObject(ctx context.Context, bucketName, objectName string, data io.ReadCloser) error {
	defer data.Close()
	_ = ctx

	// Ensure the bucket cannot be created/deleted while we look at it.
	bucketLock := s.bucketLock(bucketName)
	bucketLock.RLock()
	defer bucketLock.RUnlock()

	// Does the bucket exist?
	bucketDir := s.bucketDir(bucketName)
	if _, err := os.Stat(bucketDir); err != nil {
		return ErrNoSuchBucket
	}

	// Write the object, relying on an atomic filesystem rename operation to prevent any parallel read/write issues.
	tmpFile, err := os.CreateTemp(bucketDir, "*-"+strip(objectName))
	if err != nil {
		return errors.Wrap(err, "creating tmp file")
	}
	defer os.Remove(tmpFile.Name())
	if _, err := io.Copy(tmpFile, data); err != nil {
		return errors.Wrap(err, "copying data into tmp file")
	}
	objectFile := s.objectFile(bucketName, objectName)
	if err := os.Rename(tmpFile.Name(), objectFile); err != nil {
		return errors.Wrap(err, "renaming object file")
	}
	s.Log.Debug("put object", sglog.String("key", bucketName+"/"+objectName))
	return nil
}

func (s *Service) getObject(ctx context.Context, bucketName, objectName string) (io.ReadCloser, error) {
	_ = ctx

	// Ensure the bucket cannot be created/deleted while we look at it.
	bucketLock := s.bucketLock(bucketName)
	bucketLock.RLock()
	defer bucketLock.RUnlock()

	// Read the object
	objectFile := s.objectFile(bucketName, objectName)
	f, err := os.Open(objectFile)
	if err != nil {
		s.Log.Debug("get object", sglog.String("key", bucketName+"/"+objectName), sglog.Error(err))
		if os.IsNotExist(err) {
			return nil, ErrNoSuchKey
		}
		return nil, errors.Wrap(err, "Open")
	}
	s.Log.Debug("get object", sglog.String("key", bucketName+"/"+objectName))
	return f, nil
}

// Returns a bucket-level lock
//
// When locked for reading, you have shared access to the bucket, for reading/writing objects to it.
// The bucket cannot be created or deleted while you hold a read lock.
//
// When locked for writing, you have exclusive ownership of the entire bucket.
func (s *Service) bucketLock(name string) *sync.RWMutex {
	s.bucketLocksMu.Lock()
	defer s.bucketLocksMu.Unlock()

	lock, ok := s.bucketLocks[name]
	if !ok {
		lock = &sync.RWMutex{}
		s.bucketLocks[name] = lock
	}
	return lock
}

func (s *Service) bucketDir(name string) string {
	return filepath.Join(s.DataDir, "buckets", name)
}

func (s *Service) objectFile(bucketName, objectName string) string {
	// An object name may not be a valid file path. As a result, we use an md5sum of the object name
	// suffixed with valid filepath characters for readability in case someone wants to inspect the bucket
	// dir manually.
	md5Sum := md5.Sum([]byte(objectName))
	objectNameHash := hex.EncodeToString(md5Sum[:]) + "-" + strip(objectName)
	return filepath.Join(s.DataDir, "buckets", bucketName, objectNameHash)
}

// Replaces "/" with "--" and then strips any byte not in [^a-zA-Z0-9\-].
func strip(s string) string {
	s = strings.ReplaceAll(s, "/", "--")
	var result strings.Builder
	result.Grow(len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if ('a' <= b && b <= 'z') ||
			('A' <= b && b <= 'Z') ||
			('0' <= b && b <= '9') ||
			b == '-' {
			result.WriteByte(b)
		}
	}
	return result.String()
}

var (
	metricRunning = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "blobstore_service_running",
		Help: "Number of running blobstore requests.",
	})
)
