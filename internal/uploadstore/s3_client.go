package uploadstore

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sourcegraph/log"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/iterator"
)

type s3Store struct {
	logger       log.Logger
	bucket       string
	manageBucket bool
	client       s3API
	uploader     s3Uploader
	operations   *Operations
}

var _ Store = &s3Store{}

type S3Config struct {
	IsBlobstore     bool
	Region          string
	Endpoint        string
	UsePathStyle    bool
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

// newS3FromConfig creates a new store backed by AWS Simple Storage Service.
func newS3FromConfig(ctx context.Context, config Config, operations *Operations) (Store, error) {
	cfg, err := s3ClientConfig(ctx, config.S3)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(cfg, s3ClientOptions(config.S3))
	api := &s3APIShim{s3Client}
	uploader := &s3UploaderShim{manager.NewUploader(s3Client)}
	return newS3WithClients(api, uploader, config.Bucket, config.ManageBucket, operations), nil
}

func newS3WithClients(client s3API, uploader s3Uploader, bucket string, manageBucket bool, operations *Operations) *s3Store {
	return &s3Store{
		bucket:       bucket,
		manageBucket: manageBucket,
		client:       client,
		uploader:     uploader,
		operations:   operations,
	}
}

func (s *s3Store) Init(ctx context.Context) error {
	if !s.manageBucket {
		return nil
	}

	if err := s.create(ctx); err != nil {
		return errors.Wrap(err, "failed to create bucket")
	}

	return nil
}

// maxZeroReads is the maximum number of no-progress iterations (due to connection reset errors)
// in Get that can occur in a row before returning an error.
const maxZeroReads = 3

// errNoDownloadProgress is returned from Get after multiple connection reset errors occur
// in a row.
var errNoDownloadProgress = errors.New("no download progress")

func (s *s3Store) List(ctx context.Context, prefix string) (_ *iterator.Iterator[string], err error) {
	ctx, _, endObservation := s.operations.List.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("prefix", prefix),
	}})
	defer endObservation(1, observation.Args{})

	listObjectsV2Input := s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	}

	// This may be unnecessary, but we're being extra careful because we don't know
	// how s3 handles a pointer to an empty string.
	if prefix != "" {
		listObjectsV2Input.Prefix = aws.String(prefix)
	}

	// We wrap the client's paginator and just return the keys.
	paginator := s.client.NewListObjectsV2Paginator(&listObjectsV2Input)

	next := func() ([]string, error) {
		if !paginator.HasMorePages() {
			return nil, nil
		}

		nextPage, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		keys := make([]string, 0, len(nextPage.Contents))
		for _, c := range nextPage.Contents {
			if c.Key != nil {
				keys = append(keys, *c.Key)
			}
		}

		return keys, nil
	}

	return iterator.New[string](next), nil
}

func (s *s3Store) Get(ctx context.Context, key string) (_ io.ReadCloser, err error) {
	ctx, _, endObservation := s.operations.Get.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("key", key),
	}})
	done := func() { endObservation(1, observation.Args{}) }

	reader := writeToPipe(func(w io.Writer) error {
		zeroReads := 0
		byteOffset := int64(0)

		for {
			n, err := s.readObjectInto(ctx, w, key, byteOffset)
			if err == nil || !isConnectionResetError(err) {
				return err
			}

			byteOffset += n
			s.operations.Get.Logger.Warn("Transient error while reading payload", log.String("key", key), log.Error(err))

			if n == 0 {
				zeroReads++

				if zeroReads > maxZeroReads {
					return errNoDownloadProgress
				}
			} else {
				zeroReads = 0
			}
		}
	})

	return NewExtraCloser(io.NopCloser(reader), done), nil
}

// ioCopyHook is a pointer to io.Copy. This function is replaced in unit tests so that we can
// easily inject errors when reading from the backing S3 store.
var ioCopyHook = io.Copy

// readObjectInto reads the content of the given key starting at the given byte offset into the
// given writer. The number of bytes read is returned. On successful read, the error value is nil.
func (s *s3Store) readObjectInto(ctx context.Context, w io.Writer, key string, byteOffset int64) (int64, error) {
	var bytesRange *string
	if byteOffset > 0 {
		bytesRange = aws.String(fmt.Sprintf("bytes=%d-", byteOffset))
	} else if byteOffset < 0 {
		bytesRange = aws.String(fmt.Sprintf("bytes=%d", byteOffset))
	}

	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Range:  bytesRange,
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to get object")
	}
	defer resp.Body.Close()

	return ioCopyHook(w, resp.Body)
}

func (s *s3Store) Upload(ctx context.Context, key string, r io.Reader) (_ int64, err error) {
	ctx, _, endObservation := s.operations.Upload.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("key", key),
	}})
	defer endObservation(1, observation.Args{})

	cr := &countingReader{r: r}

	if err := s.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   cr,
	}); err != nil {
		return 0, errors.Wrap(err, "failed to upload object")
	}

	return int64(cr.n), nil
}

func (s *s3Store) Compose(ctx context.Context, destination string, sources ...string) (_ int64, err error) {
	ctx, _, endObservation := s.operations.Compose.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("destination", destination),
		attribute.StringSlice("sources", sources),
	}})
	defer endObservation(1, observation.Args{})

	multipartUpload, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(destination),
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to create multipart upload")
	}

	defer func() {
		if err == nil {
			// Delete sources on success
			if err := s.deleteSources(ctx, *multipartUpload.Bucket, sources); err != nil {
				s.operations.Compose.Logger.Error("failed to delete source objects", log.Error(err))
			}
		} else {
			// On failure, try to clean up copied then orphaned parts
			if _, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
				Bucket:   multipartUpload.Bucket,
				Key:      multipartUpload.Key,
				UploadId: multipartUpload.UploadId,
			}); err != nil {
				s.operations.Compose.Logger.Error("Failed to abort multipart upload", log.Error(err))
			}
		}
	}()

	var m sync.Mutex
	etags := map[int]*string{}

	if err := ForEachString(sources, func(index int, source string) error {
		partNumber := index + 1

		copyResult, err := s.client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
			Bucket:     multipartUpload.Bucket,
			Key:        multipartUpload.Key,
			UploadId:   multipartUpload.UploadId,
			PartNumber: int32(partNumber),
			CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, source)),
		})
		if err != nil {
			return errors.Wrap(err, "failed to upload part")
		}

		m.Lock()
		etags[partNumber] = copyResult.CopyPartResult.ETag
		m.Unlock()

		return nil
	}); err != nil {
		return 0, err
	}

	var parts []s3types.CompletedPart
	for i := range len(sources) {
		partNumber := i + 1

		parts = append(parts, s3types.CompletedPart{
			ETag:       etags[partNumber],
			PartNumber: int32(partNumber),
		})
	}

	if _, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:          multipartUpload.Bucket,
		Key:             multipartUpload.Key,
		UploadId:        multipartUpload.UploadId,
		MultipartUpload: &s3types.CompletedMultipartUpload{Parts: parts},
	}); err != nil {
		return 0, errors.Wrap(err, "failed to complete multipart upload")
	}

	obj, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: multipartUpload.Bucket,
		Key:    multipartUpload.Key,
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to stat composed object")
	}

	return obj.ContentLength, nil
}

func (s *s3Store) Delete(ctx context.Context, key string) (err error) {
	ctx, _, endObservation := s.operations.Delete.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("key", key),
	}})
	defer endObservation(1, observation.Args{})

	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})

	return errors.Wrap(err, "failed to delete object")
}

func (s *s3Store) ExpireObjects(ctx context.Context, prefix string, maxAge time.Duration) (err error) {
	ctx, _, endObservation := s.operations.ExpireObjects.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("prefix", prefix),
		attribute.Stringer("maxAge", maxAge),
	}})
	defer endObservation(1, observation.Args{})

	var toDelete []s3types.ObjectIdentifier
	flush := func() {
		_, err = s.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: &s.bucket,
			Delete: &s3types.Delete{
				Objects: toDelete,
			},
		})
		if err != nil {
			s.operations.ExpireObjects.Logger.Error("Failed to delete objects in S3 bucket",
				log.Error(err),
				log.String("bucket", s.bucket))
			return // try again at next flush
		}
		toDelete = toDelete[:0]
	}
	paginator := s.client.NewListObjectsV2Paginator(&s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			s.operations.ExpireObjects.Error("Failed to paginate S3 bucket", log.Error(err))
			break // we'll try again later
		}
		for _, object := range page.Contents {
			if time.Since(*object.LastModified) >= maxAge {
				toDelete = append(toDelete,
					s3types.ObjectIdentifier{
						Key: object.Key,
					})
				if len(toDelete) >= 1000 {
					flush()
				}
			}
		}
	}
	if len(toDelete) > 0 {
		flush()
	}
	return nil
}

func (s *s3Store) create(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s.bucket),
	})

	if errors.HasType(err, &s3types.BucketAlreadyExists{}) || errors.HasType(err, &s3types.BucketAlreadyOwnedByYou{}) {
		return nil
	}

	return err
}

func (s *s3Store) deleteSources(ctx context.Context, bucket string, sources []string) error {
	return ForEachString(sources, func(index int, source string) error {
		if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(source),
		}); err != nil {
			return errors.Wrap(err, "failed to delete source object")
		}

		return nil
	})
}

// countingReader is an io.Reader that counts the number of bytes sent
// back to the caller.
type countingReader struct {
	r io.Reader
	n int
}

func (r *countingReader) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	r.n += n
	return n, err
}

func s3ClientConfig(ctx context.Context, s3config S3Config) (aws.Config, error) {
	if s3config.IsBlobstore {
		// For blobstore, no need to read local credential files or talk to a server
		// to get a role assumption. Instead, we return a simple config with only a static
		// provider and that's it.
		//
		// NOTE: We allow credential free access. In that case we set no
		// provider. Otherwise the s3 client fails to sign requests with empty
		// static credentials.
		cfg := aws.NewConfig()

		if s3config.AccessKeyID != "" || s3config.SecretAccessKey != "" || s3config.SessionToken != "" {
			cfg.Credentials = credentials.NewStaticCredentialsProvider(
				s3config.AccessKeyID,
				s3config.SecretAccessKey,
				s3config.SessionToken,
			)
		}

		cfg.Region = s3config.Region
		return *cfg, nil
	}

	optFns := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(s3config.Region),
	}

	if s3config.AccessKeyID != "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			s3config.AccessKeyID,
			s3config.SecretAccessKey,
			s3config.SessionToken,
		)))
	}

	return awsconfig.LoadDefaultConfig(ctx, optFns...)
}

func s3ClientOptions(config S3Config) func(o *s3.Options) {
	return func(o *s3.Options) {
		if config.Endpoint != "" {
			o.EndpointResolver = s3.EndpointResolverFromURL(config.Endpoint)
		}

		o.UsePathStyle = config.UsePathStyle
	}
}

// writeToPipe invokes the given function with a pipe writer in a goroutine
// and returns the associated pipe reader.
func writeToPipe(fn func(w io.Writer) error) io.Reader {
	pr, pw := io.Pipe()
	go func() { _ = pw.CloseWithError(fn(pw)) }()
	return pr
}

func isConnectionResetError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "read: connection reset by peer")
}
