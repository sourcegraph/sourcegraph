package loki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/grafana/regexp"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/bk"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const pushEndpoint = "/loki/api/v1/push"

// On Grafana cloud Loki rejects log entries that are longer that 65536 bytes.
const maxEntrySize = math.MaxUint16

// To point at a custom instance, e.g. one on Grafana Cloud, refer to:
// https://grafana.com/orgs/sourcegraph/hosted-logs/85581#sending-logs
// The URL should have the format https://85581:$TOKEN@logs-prod-us-central1.grafana.net
const DefaultLokiURL = "http://127.0.0.1:3100"

// Stream is the Loki logs equivalent of a metric series.
type Stream struct {
	// Labels map identifying a stream
	Stream StreamLabels `json:"stream"`

	// ["<unix epoch in nanoseconds>"", "<log line>"] value pairs
	Values [][2]string `json:"values"`
}

// StreamLabels is an identifier for a Loki log stream, denoted by a set of labels.
//
// NOTE: bk.JobMeta is very high-cardinality, since we create a new stream for each job.
// Similarly to Prometheus, Loki is not designed to handle very high cardinality log streams.
// However, it is important that each job gets a separate stream, because Loki does not
// permit non-chronologically uploaded logs, so simultaneous jobs logs will collide.
// NewStreamFromJobLogs handles this within a job by merging entries with the same timestamp.
// Possible routes for investigation:
// - https://grafana.com/docs/loki/latest/operations/storage/retention/
// - https://grafana.com/docs/loki/latest/operations/storage/table-manager/
type StreamLabels struct {
	bk.JobMeta

	// Distinguish from other log streams

	App       string `json:"app"`
	Component string `json:"component"`

	// Additional metadata for CI when pushing

	Branch string `json:"branch"`
	Queue  string `json:"queue"`
}

// NewStreamFromJobLogs cleans the given log data, splits it into log entries, merges
// entries with the same timestamp, and returns a Stream that can be pushed to Loki.
func NewStreamFromJobLogs(log *bk.JobLogs) (*Stream, error) {
	stream := StreamLabels{
		JobMeta:   log.JobMeta,
		App:       "buildkite",
		Component: "build-logs",
	}
	cleanedContent := bk.CleanANSI(*log.Content)

	// seems to be some kind of buildkite line separator, followed by a timestamp
	const bkTimestampSeparator = "_bk;"
	if len(cleanedContent) == 0 {
		return &Stream{
			Stream: stream,
			Values: make([][2]string, 0),
		}, nil
	}
	if !strings.Contains(cleanedContent, bkTimestampSeparator) {
		return nil, errors.Newf("log content does not contain Buildkite timestamps, denoted by %q", bkTimestampSeparator)
	}
	lines := strings.Split(cleanedContent, bkTimestampSeparator)

	// parse lines into loki log entries
	values := make([][2]string, 0, len(lines))
	var previousTimestamp string
	timestamp := regexp.MustCompile(`t=(?P<ts>\d{13})`) // 13 digits for unix epoch in nanoseconds
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 3 {
			continue // ignore irrelevant lines
		}

		tsMatches := timestamp.FindStringSubmatch(line)
		if len(tsMatches) == 0 {
			return nil, errors.Newf("no timestamp on line %q", line)
		}

		line = strings.TrimSpace(strings.Replace(line, tsMatches[0], "", 1))
		if len(line) < 3 {
			continue // ignore irrelevant lines
		}

		ts := strings.Replace(tsMatches[0], "t=", "", 1)
		if ts == previousTimestamp {
			value := values[len(values)-1]
			value[1] = value[1] + fmt.Sprintf("\n%s", line)
			// Check that the current entry is not larger than maxEntrySize (65536) in bytes.
			// If it is, we take the entry split into chunks of maxEntrySize bytes.
			//
			// To ensure that each chunked entry doesn't clash with a previous entry in Loki, the nanoseconds of
			// each entry is incremented by 1 for each chunked entry.
			chunkedEntries, err := chunkEntry(value, maxEntrySize)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to split value entry into chunks")
			}

			// replace the value we split into chunks with the first chunk 0, then add the rest
			values[len(values)-1] = chunkedEntries[0]
			if len(chunkedEntries) > 1 {
				values = append(values, chunkedEntries[1:]...)
			}
		} else {
			// buildkite timestamps are in ms, so convert to ns with a lot of zeros
			value := [2]string{ts + "000000", line}
			chunkedEntries, err := chunkEntry(value, maxEntrySize)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to split value entry into chunks")
			}
			values = append(values, chunkedEntries...)
			previousTimestamp = ts
		}

	}

	return &Stream{
		Stream: stream,
		Values: values,
	}, nil
}

func chunkEntry(entry [2]string, chunkSize int) ([][2]string, error) {
	if len(entry[1]) < chunkSize {
		return [][2]string{entry}, nil
	}
	// the first item in an entry is the timestamp
	epoch, err := strconv.ParseInt(entry[0], 10, 64)
	if err != nil {
		return nil, err
	}
	// TODO(burmudar): Use runes instead since with bytes we might split on a UTF-8 char
	chunks := splitIntoChunks([]byte(entry[1]), chunkSize)

	results := make([][2]string, len(chunks))
	for i, c := range chunks {
		ts := fmt.Sprintf("%d", epoch+int64(i))
		results[i] = [2]string{ts, string(c)}
	}

	return results, nil
}

func splitIntoChunks(data []byte, chunkSize int) [][]byte {
	count := math.Ceil(float64(len(data)) / float64(chunkSize))

	if count <= 1 {
		return [][]byte{data}
	}

	chunks := make([][]byte, int(count))

	for i := range int(count) {
		start := i * chunkSize
		end := start + chunkSize

		if end <= len(data) {
			chunks[i] = data[start:end]
		} else {
			chunks[i] = data[start:]
		}
	}

	return chunks
}

// https://grafana.com/docs/loki/latest/api/#post-lokiapiv1push
type jsonPushBody struct {
	Streams []*Stream `json:"streams"`
}

type Client struct {
	lokiURL *url.URL
}

func NewLokiClient(lokiURL *url.URL) *Client {
	return &Client{lokiURL}
}

func (c *Client) PushStreams(ctx context.Context, streams []*Stream) error {
	body, err := json.Marshal(&jsonPushBody{Streams: streams})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, c.lokiURL.String()+pushEndpoint, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()
		// Stream already published
		if strings.Contains(string(b), "entry out of order") {
			return nil
		}
		return errors.Newf("unexpected status code %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
