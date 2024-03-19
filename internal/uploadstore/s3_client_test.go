package uploadstore

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/go-cmp/cmp"
	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestS3Init(t *testing.T) {
	s3Client := NewMockS3API()
	client := testS3Client(s3Client, nil)
	if err := client.Init(context.Background()); err != nil {
		t.Fatalf("unexpected error initializing client: %s", err)
	}

	if calls := s3Client.CreateBucketFunc.History(); len(calls) != 1 {
		t.Fatalf("unexpected number of CreateBucket calls. want=%d have=%d", 1, len(calls))
	} else if value := *calls[0].Arg1.Bucket; value != "test-bucket" {
		t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
	}
}

func TestS3InitBucketExists(t *testing.T) {
	for _, err := range []error{&s3types.BucketAlreadyExists{}, &s3types.BucketAlreadyOwnedByYou{}} {
		s3Client := NewMockS3API()
		s3Client.CreateBucketFunc.SetDefaultReturn(nil, err)

		client := testS3Client(s3Client, nil)
		if err := client.Init(context.Background()); err != nil {
			t.Fatalf("unexpected error initializing client: %s", err)
		}

		if calls := s3Client.CreateBucketFunc.History(); len(calls) != 1 {
			t.Fatalf("unexpected number of CreateBucket calls. want=%d have=%d", 1, len(calls))
		} else if value := *calls[0].Arg1.Bucket; value != "test-bucket" {
			t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
		}
	}
}

func TestS3UnmanagedInit(t *testing.T) {
	s3Client := NewMockS3API()
	client := newS3WithClients(s3Client, nil, "test-bucket", false, NewOperations(&observation.TestContext, "test", "brittleStore"))
	if err := client.Init(context.Background()); err != nil {
		t.Fatalf("unexpected error initializing client: %s", err)
	}

	if calls := s3Client.CreateBucketFunc.History(); len(calls) != 0 {
		t.Fatalf("unexpected number of CreateBucket calls. want=%d have=%d", 0, len(calls))
	}
}

func TestS3Get(t *testing.T) {
	s3Client := NewMockS3API()
	s3Client.GetObjectFunc.SetDefaultReturn(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("TEST PAYLOAD"))),
	}, nil)

	client := newS3WithClients(s3Client, nil, "test-bucket", false, NewOperations(&observation.TestContext, "test", "brittleStore"))
	rc, err := client.Get(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("unexpected error getting key: %s", err)
	}
	defer rc.Close()

	contents, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("unexpected error reading object: %s", err)
	}

	if string(contents) != "TEST PAYLOAD" {
		t.Fatalf("unexpected contents. want=%s have=%s", "TEST PAYLOAD", contents)
	}

	if calls := s3Client.GetObjectFunc.History(); len(calls) != 1 {
		t.Fatalf("unexpected number of GetObject calls. want=%d have=%d", 1, len(calls))
	} else if value := *calls[0].Arg1.Bucket; value != "test-bucket" {
		t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
	} else if value := *calls[0].Arg1.Key; value != "test-key" {
		t.Errorf("unexpected key argument. want=%s have=%s", "test-key", value)
	} else if value := calls[0].Arg1.Range; value != nil {
		t.Errorf("unexpected range argument. want=%v have=%v", nil, value)
	}
}

var bytesPattern = regexp.MustCompile(`bytes=(\d+)-`)

func TestS3GetTransientErrors(t *testing.T) {
	// read 50 bytes then return a connection reset error
	ioCopyHook = func(w io.Writer, r io.Reader) (int64, error) {
		var buf bytes.Buffer
		_, readErr := io.CopyN(&buf, r, 50)
		if readErr != nil && readErr != io.EOF {
			return 0, readErr
		}

		n, writeErr := io.Copy(w, bytes.NewReader(buf.Bytes()))
		if writeErr != nil {
			return 0, writeErr
		}

		if readErr == io.EOF {
			readErr = nil
		} else {
			readErr = errors.New("read: connection reset by peer")
		}
		return n, readErr
	}

	s3Client := fullContentsS3API()
	client := newS3WithClients(s3Client, nil, "test-bucket", false, NewOperations(&observation.TestContext, "test", "brittleStore"))
	rc, err := client.Get(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("unexpected error getting key: %s", err)
	}
	defer rc.Close()

	contents, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("unexpected error reading object: %s", err)
	}

	if diff := cmp.Diff(fullContents, contents); diff != "" {
		t.Errorf("unexpected payload (-want +got):\n%s", diff)
	}

	expectedGetObjectCalls := len(fullContents)/50 + 1
	if calls := s3Client.GetObjectFunc.History(); len(calls) != expectedGetObjectCalls {
		t.Fatalf("unexpected number of GetObject calls. want=%d have=%d", expectedGetObjectCalls, len(calls))
	}
}

func TestS3GetReadNothingLoop(t *testing.T) {
	// read nothing then return a connection reset error
	ioCopyHook = func(w io.Writer, r io.Reader) (int64, error) {
		return 0, errors.New("read: connection reset by peer")
	}

	s3Client := fullContentsS3API()
	client := newS3WithClients(s3Client, nil, "test-bucket", false, NewOperations(&observation.TestContext, "test", "brittleStore"))
	rc, err := client.Get(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("unexpected error getting key: %s", err)
	}
	defer rc.Close()

	if _, err := io.ReadAll(rc); err != errNoDownloadProgress {
		t.Fatalf("unexpected error reading object. want=%q have=%q", errNoDownloadProgress, err)
	}
}

var fullContents = func() []byte {
	var fullContents []byte
	for i := range 1000 {
		fullContents = append(fullContents, []byte(fmt.Sprintf("payload %d\n", i))...)
	}

	return fullContents
}()

func fullContentsS3API() *MockS3API {
	s3Client := NewMockS3API()
	s3Client.GetObjectFunc.SetDefaultHook(func(ctx context.Context, input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
		offset := 0
		if input.Range != nil {
			match := bytesPattern.FindStringSubmatch(*input.Range)
			if len(match) != 0 {
				offset, _ = strconv.Atoi(match[1])
			}
		}

		out := &s3.GetObjectOutput{
			Body: io.NopCloser(bytes.NewReader(fullContents[offset:])),
		}

		return out, nil
	})

	return s3Client
}

func TestS3Upload(t *testing.T) {
	s3Client := NewMockS3API()
	uploaderClient := NewMockS3Uploader()
	uploaderClient.UploadFunc.SetDefaultHook(func(ctx context.Context, input *s3.PutObjectInput) error {
		// Synchronously read the reader so that we trigger the
		// counting reader inside the Upload method and test the
		// count.
		contents, err := io.ReadAll(input.Body)
		if err != nil {
			return err
		}

		if string(contents) != "TEST PAYLOAD" {
			t.Fatalf("unexpected contents. want=%s have=%s", "TEST PAYLOAD", contents)
		}

		return nil
	})

	client := testS3Client(s3Client, uploaderClient)

	size, err := client.Upload(context.Background(), "test-key", bytes.NewReader([]byte("TEST PAYLOAD")))
	if err != nil {
		t.Fatalf("unexpected error getting key: %s", err)
	} else if size != 12 {
		t.Errorf("unexpected size. want=%d have=%d", 12, size)
	}

	if calls := uploaderClient.UploadFunc.History(); len(calls) != 1 {
		t.Fatalf("unexpected number of Upload calls. want=%d have=%d", 1, len(calls))
	} else if value := *calls[0].Arg1.Bucket; value != "test-bucket" {
		t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
	} else if value := *calls[0].Arg1.Key; value != "test-key" {
		t.Errorf("unexpected key argument. want=%s have=%s", "test-key", value)
	}
}

func TestS3Combine(t *testing.T) {
	s3Client := NewMockS3API()
	s3Client.CreateMultipartUploadFunc.SetDefaultReturn(&s3.CreateMultipartUploadOutput{
		Bucket:   aws.String("test-bucket"),
		Key:      aws.String("test-key"),
		UploadId: aws.String("uid"),
	}, nil)

	s3Client.UploadPartCopyFunc.SetDefaultHook(func(ctx context.Context, input *s3.UploadPartCopyInput) (*s3.UploadPartCopyOutput, error) {
		return &s3.UploadPartCopyOutput{
			CopyPartResult: &s3types.CopyPartResult{
				ETag: aws.String(fmt.Sprintf("etag-%s", *input.CopySource)),
			},
		}, nil
	})

	s3Client.HeadObjectFunc.SetDefaultReturn(&s3.HeadObjectOutput{ContentLength: int64(42)}, nil)

	client := testS3Client(s3Client, nil)

	size, err := client.Compose(context.Background(), "test-key", "test-src1", "test-src2", "test-src3")
	if err != nil {
		t.Fatalf("unexpected error getting key: %s", err)
	} else if size != 42 {
		t.Errorf("unexpected size. want=%d have=%d", 42, size)
	}

	if calls := s3Client.UploadPartCopyFunc.History(); len(calls) != 3 {
		t.Fatalf("unexpected number of UploadPartCopy calls. want=%d have=%d", 3, len(calls))
	} else {
		parts := map[int32]string{}
		for _, call := range calls {
			if value := *call.Arg1.Bucket; value != "test-bucket" {
				t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
			}
			if value := *call.Arg1.Key; value != "test-key" {
				t.Errorf("unexpected key argument. want=%s have=%s", "test-key", value)
			}
			if value := *call.Arg1.UploadId; value != "uid" {
				t.Errorf("unexpected key argument. want=%s have=%s", "uid", value)
			}

			parts[call.Arg1.PartNumber] = *call.Arg1.CopySource
		}

		expectedParts := map[int32]string{
			1: "test-bucket/test-src1",
			2: "test-bucket/test-src2",
			3: "test-bucket/test-src3",
		}
		if diff := cmp.Diff(expectedParts, parts); diff != "" {
			t.Fatalf("unexpected parts payloads (-want, +got):\n%s", diff)
		}
	}

	if calls := s3Client.CreateMultipartUploadFunc.History(); len(calls) != 1 {
		t.Fatalf("unexpected number of CreateMultipartUpload calls. want=%d have=%d", 1, len(calls))
	} else if value := *calls[0].Arg1.Bucket; value != "test-bucket" {
		t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
	} else if value := *calls[0].Arg1.Key; value != "test-key" {
		t.Errorf("unexpected key argument. want=%s have=%s", "test-key", value)
	}

	if calls := s3Client.CompleteMultipartUploadFunc.History(); len(calls) != 1 {
		t.Fatalf("unexpected number of CompleteMultipartUpload calls. want=%d have=%d", 1, len(calls))
	} else if value := *calls[0].Arg1.Bucket; value != "test-bucket" {
		t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
	} else if value := *calls[0].Arg1.Key; value != "test-key" {
		t.Errorf("unexpected key argument. want=%s have=%s", "test-key", value)
	} else if value := *calls[0].Arg1.UploadId; value != "uid" {
		t.Errorf("unexpected uploadId argument. want=%s have=%s", "uid", value)
	} else {
		parts := map[int32]string{}
		for _, part := range calls[0].Arg1.MultipartUpload.Parts {
			parts[part.PartNumber] = *part.ETag
		}

		expectedParts := map[int32]string{
			1: "etag-test-bucket/test-src1",
			2: "etag-test-bucket/test-src2",
			3: "etag-test-bucket/test-src3",
		}
		if diff := cmp.Diff(expectedParts, parts); diff != "" {
			t.Fatalf("unexpected parts payloads (-want, +got):\n%s", diff)
		}
	}

	if calls := s3Client.AbortMultipartUploadFunc.History(); len(calls) != 0 {
		t.Fatalf("unexpected number of AbortMultipartUpload calls. want=%d have=%d", 0, len(calls))
	}

	if calls := s3Client.DeleteObjectFunc.History(); len(calls) != 3 {
		t.Fatalf("unexpected number of DeleteObject calls. want=%d have=%d", 3, len(calls))
	} else {
		var keys []string
		for _, call := range calls {
			if value := *call.Arg1.Bucket; value != "test-bucket" {
				t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
			}
			keys = append(keys, *call.Arg1.Key)
		}
		sort.Strings(keys)

		expectedKeys := []string{
			"test-src1",
			"test-src2",
			"test-src3",
		}
		if diff := cmp.Diff(expectedKeys, keys); diff != "" {
			t.Fatalf("unexpected keys (-want, +got):\n%s", diff)
		}
	}
}

func TestS3Delete(t *testing.T) {
	s3Client := NewMockS3API()
	s3Client.GetObjectFunc.SetDefaultReturn(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader([]byte("TEST PAYLOAD"))),
	}, nil)

	client := testS3Client(s3Client, nil)
	if err := client.Delete(context.Background(), "test-key"); err != nil {
		t.Fatalf("unexpected error getting key: %s", err)
	}

	if calls := s3Client.DeleteObjectFunc.History(); len(calls) != 1 {
		t.Fatalf("unexpected number of DeleteObject calls. want=%d have=%d", 1, len(calls))
	} else if value := *calls[0].Arg1.Bucket; value != "test-bucket" {
		t.Errorf("unexpected bucket argument. want=%s have=%s", "test-bucket", value)
	} else if value := *calls[0].Arg1.Key; value != "test-key" {
		t.Errorf("unexpected key argument. want=%s have=%s", "test-key", value)
	}
}

func testS3Client(client s3API, uploader s3Uploader) Store {
	return newLazyStore(rawS3Client(client, uploader))
}

func rawS3Client(client s3API, uploader s3Uploader) *s3Store {
	return newS3WithClients(client, uploader, "test-bucket", true, NewOperations(&observation.TestContext, "test", "brittleStore"))
}
