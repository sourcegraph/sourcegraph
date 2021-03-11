package codeintelutils

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
)

type UploadIndexOpts struct {
	Endpoint             string
	Path                 string
	AccessToken          string
	AdditionalHeaders    map[string]string
	Repo                 string
	Commit               string
	Root                 string
	Indexer              string
	GitHubToken          string
	File                 string
	AssociatedIndexID    *int
	MaxPayloadSizeBytes  int
	MaxRetries           int
	RetryInterval        time.Duration
	UploadProgressEvents chan UploadProgressEvent
	Logger               RequestLogger
}

type UploadProgressEvent struct {
	NumParts      int
	Part          int
	Progress      float64
	TotalProgress float64
}

type RequestLogger interface {
	LogRequest(req *http.Request)
	LogResponse(req *http.Request, resp *http.Response, body []byte, elapsed time.Duration)
}

// ErrUnauthorized occurs when the upload endpoint returns a 401 response.
var ErrUnauthorized = errors.New("unauthorized upload")

// UploadIndex uploads the given index file to the upload endpoint. If the upload
// file is large, it may be split into multiple chunks locally and uploaded over
// multiple requests.
func UploadIndex(opts UploadIndexOpts) (int, error) {
	fileInfo, err := os.Stat(opts.File)
	if err != nil {
		return 0, err
	}

	if fileInfo.Size() <= int64(opts.MaxPayloadSizeBytes) {
		id, err := uploadIndex(opts)
		if err != nil {
			return 0, err
		}
		return id, nil
	}

	id, err := uploadMultipartIndex(opts)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// uploadIndex performs a single request to the upload endpoint. The new upload id is returned.
func uploadIndex(opts UploadIndexOpts) (id int, err error) {
	baseURL, err := makeBaseURL(opts)
	if err != nil {
		return 0, err
	}

	retry := makeRetry(opts.MaxRetries, opts.RetryInterval)

	args := requestArgs{
		baseURL:           baseURL,
		accessToken:       opts.AccessToken,
		additionalHeaders: opts.AdditionalHeaders,
		repo:              opts.Repo,
		commit:            opts.Commit,
		root:              opts.Root,
		indexer:           opts.Indexer,
		gitHubToken:       opts.GitHubToken,
		associatedIndexID: opts.AssociatedIndexID,
	}

	if err := retry(func() (_ bool, err error) {
		return uploadFile(args, opts.File, false, &id, 0, 1, opts.UploadProgressEvents, opts.Logger)
	}); err != nil {
		return 0, err
	}

	return id, nil
}

// uploadMultipartIndex splits the index file into chunks small enough to upload, then
// performs a series of request to the upload endpoint. The new upload id is returned.
func uploadMultipartIndex(opts UploadIndexOpts) (id int, err error) {
	baseURL, err := makeBaseURL(opts)
	if err != nil {
		return 0, err
	}

	retry := makeRetry(opts.MaxRetries, opts.RetryInterval)

	compressedFile, err := compressFile(opts.File)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = os.Remove(compressedFile)
	}()

	files, cleanup, err := SplitFile(compressedFile, opts.MaxPayloadSizeBytes)
	if err != nil {
		return 0, err
	}
	defer func() {
		err = cleanup(err)
	}()

	setupArgs := requestArgs{
		baseURL:           baseURL,
		accessToken:       opts.AccessToken,
		additionalHeaders: opts.AdditionalHeaders,
		repo:              opts.Repo,
		commit:            opts.Commit,
		root:              opts.Root,
		indexer:           opts.Indexer,
		gitHubToken:       opts.GitHubToken,
		multiPart:         true,
		numParts:          len(files),
		associatedIndexID: opts.AssociatedIndexID,
	}

	if err := retry(func() (bool, error) { return makeUploadRequest(setupArgs, nil, &id, opts.Logger) }); err != nil {
		return 0, err
	}

	for i, file := range files {
		uploadArgs := requestArgs{
			baseURL:           baseURL,
			accessToken:       opts.AccessToken,
			additionalHeaders: opts.AdditionalHeaders,
			uploadID:          id,
			index:             i,
		}

		if err := retry(func() (_ bool, err error) {
			return uploadFile(uploadArgs, file, true, nil, i, len(files), opts.UploadProgressEvents, opts.Logger)
		}); err != nil {
			return 0, err
		}
	}

	finalizeArgs := requestArgs{
		baseURL:           baseURL,
		accessToken:       opts.AccessToken,
		additionalHeaders: opts.AdditionalHeaders,
		uploadID:          id,
		done:              true,
	}

	if err := retry(func() (bool, error) { return makeUploadRequest(finalizeArgs, nil, nil, opts.Logger) }); err != nil {
		return 0, err
	}

	return id, nil
}

func makeBaseURL(opts UploadIndexOpts) (*url.URL, error) {
	endpointAndPath := opts.Endpoint
	if opts.Path != "" {
		endpointAndPath += opts.Path
	} else {
		endpointAndPath += "/.api/lsif/upload"
	}

	return url.Parse(endpointAndPath)
}

func compressFile(filename string) (_ string, err error) {
	rawFile, err := os.Open(filename)
	if err != nil {
		return "", nil
	}
	defer rawFile.Close()

	compressedFile, err := ioutil.TempFile("", "")
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := compressedFile.Close(); err != nil {
			err = multierror.Append(err, closeErr)
		}
	}()

	gzipWriter := gzip.NewWriter(compressedFile)
	defer func() {
		if closeErr := gzipWriter.Close(); err != nil {
			err = multierror.Append(err, closeErr)
		}
	}()

	if _, err := io.Copy(gzipWriter, rawFile); err != nil {
		return "", nil
	}

	return compressedFile.Name(), nil
}

// requestArgs are a superset of the values that can be supplied in the query string of the
// upload endpoint. The endpoint and access token fields must be set on every request, but the
// remaining fields must be set when appropriate by the caller of makeUploadRequest.
type requestArgs struct {
	baseURL           *url.URL
	accessToken       string
	additionalHeaders map[string]string
	repo              string
	commit            string
	root              string
	indexer           string
	gitHubToken       string
	multiPart         bool
	numParts          int
	uploadID          int
	index             int
	done              bool
	associatedIndexID *int
}

// EncodeQuery constructs a query string from the args.
func (args requestArgs) EncodeQuery() string {
	qs := newQueryValues()
	qs.SetOptionalString("repository", args.repo)
	qs.SetOptionalString("commit", args.commit)
	qs.SetOptionalString("root", args.root)
	qs.SetOptionalString("indexerName", args.indexer)
	qs.SetOptionalString("github_token", args.gitHubToken)
	qs.SetOptionalBool("multiPart", args.multiPart)
	qs.SetOptionalInt("numParts", args.numParts)
	qs.SetOptionalInt("uploadId", args.uploadID)
	qs.SetOptionalBool("done", args.done)

	if args.associatedIndexID != nil {
		qs.SetInt("associatedIndexId", *args.associatedIndexID)
	}

	// Do not set an index of zero unless we're uploading a part
	if args.uploadID != 0 && !args.multiPart && !args.done {
		qs.SetInt("index", args.index)
	}

	return qs.Encode()
}

const ProgressUpdateInterval = time.Millisecond * 100

// uploadFile performs an HTTP POST to the upload endpoint with the content from the given file.
// This method will gzip the content before sending. If target is a non-nil pointer, it will be
// assigned the value of the upload identifier present in the response body. If the events channel
// is non-nil, progress of the upload will be sent to it on a timer. This function returns an error
// as well as a boolean flag indicating if the function can be retried.
func uploadFile(args requestArgs, file string, compressed bool, target *int, part, numParts int, events chan<- UploadProgressEvent, logger RequestLogger) (bool, error) {
	f, err := os.Open(file)
	if err != nil {
		return false, err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = multierror.Append(err, closeErr)
		}
	}()

	var r io.Reader = f

	if events != nil {
		info, err := os.Stat(file)
		if err != nil {
			return false, err
		}

		rateReader := newRateReader(f, info.Size())
		progressPerFile := 1 / float64(numParts)

		t := time.NewTicker(ProgressUpdateInterval)
		defer t.Stop()

		go func() {
			for range t.C {
				p := rateReader.Progress()
				event := UploadProgressEvent{
					NumParts:      numParts,
					Part:          part + 1,
					Progress:      p,
					TotalProgress: float64(part)*progressPerFile + (p * progressPerFile),
				}

				select {
				case events <- event:
				default:
				}
			}
		}()

		r = rateReader
	}

	if !compressed {
		r = Gzip(r)
	}

	return makeUploadRequest(args, r, target, logger)
}

// makeUploadRequest performs an HTTP POST to the upload endpoint. The query string of the request
// is constructed from the given request args and the body of the request is the unmodified reader.
// If target is a non-nil pointer, it will be assigned the value of the upload identifier present
// in the response body. This function returns an error as well as a boolean flag indicating if the
// function can be retried.
func makeUploadRequest(args requestArgs, payload io.Reader, target *int, logger RequestLogger) (bool, error) {
	url := args.baseURL
	url.RawQuery = args.EncodeQuery()

	req, err := http.NewRequest("POST", url.String(), payload)
	if err != nil {
		return false, err
	}

	req.Header.Set("Content-Type", "application/x-ndjson+lsif")
	if args.accessToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", args.accessToken))
	}
	for k, v := range args.additionalHeaders {
		req.Header.Set(k, v)
	}

	started := time.Now()
	if logger != nil {
		logger.LogRequest(req)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if logger != nil {
		logger.LogResponse(req, resp, body, time.Since(started))
	}
	if err != nil {
		return true, err
	}

	if resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusUnauthorized {
			return false, ErrUnauthorized
		}

		// Do not retry client errors
		err = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		return resp.StatusCode >= 500, err
	}

	if target != nil {
		var respPayload struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &respPayload); err != nil {
			return false, fmt.Errorf("unexpected response (%s)", err)
		}

		id, err := strconv.Atoi(respPayload.ID)
		if err != nil {
			return false, fmt.Errorf("unexpected response (%s)", err)
		}

		*target = id
	}

	return false, nil
}

// makeRetry returns a function that calls retry with the given max attempt and interval values.
func makeRetry(n int, interval time.Duration) func(f func() (bool, error)) error {
	return func(f func() (bool, error)) error {
		return retry(f, n, interval)
	}
}

// retry will re-invoke the given function until it returns a nil error value, the function returns
// a non-retryable error (as indicated by its boolean return value), or until the maximum number of
// retries have been attempted. The returned error will be the last error to occur.
func retry(f func() (bool, error), n int, interval time.Duration) (err error) {
	var retry bool
	for i := n; i >= 0; i-- {
		if retry, err = f(); err == nil || !retry {
			break
		}

		time.Sleep(interval)
	}

	return err
}

// queryValues is a convenience wrapper around url.Values that adds
// behaviors to set values of non-string types and optionally set
// values that may be a zero-value.
type queryValues struct {
	values url.Values
}

// newQueryValues creates a new queryValues.
func newQueryValues() queryValues {
	return queryValues{values: url.Values{}}
}

// Set adds the given name/string-value pairing to the underlying values.
func (qv queryValues) Set(name string, value string) {
	qv.values[name] = []string{value}
}

// SetInt adds the given name/int-value pairing to the underlying values.
func (qv queryValues) SetInt(name string, value int) {
	qv.Set(name, strconv.FormatInt(int64(value), 10))
}

// SetOptionalString adds the given name/string-value pairing to the underlying values.
// If the value is empty, the underlying values are not modified.
func (qv queryValues) SetOptionalString(name string, value string) {
	if value != "" {
		qv.Set(name, value)
	}
}

// SetOptionalInt adds the given name/int-value pairing to the underlying values.
// If the value is zero, the underlying values are not modified.
func (qv queryValues) SetOptionalInt(name string, value int) {
	if value != 0 {
		qv.SetInt(name, value)
	}
}

// SetOptionalBool adds the given name/bool-value pairing to the underlying values.
// If the value is false, the underlying values are not modified.
func (qv queryValues) SetOptionalBool(name string, value bool) {
	if value {
		qv.Set(name, "true")
	}
}

// Encode encodes the underlying values.
func (qv queryValues) Encode() string {
	return qv.values.Encode()
}
