package streaming

import (
	"context"
	"io"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api/internalapi"
	"github.com/sourcegraph/sourcegraph/internal/compute/client"
	streamhttp "github.com/sourcegraph/sourcegraph/internal/search/streaming/http"
	"github.com/sourcegraph/sourcegraph/internal/trace"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

// Opts contains the search options supported by Search.
type Opts struct {
	Display int
	Trace   bool
	Json    bool
}

// Search calls the streaming search endpoint and uses decoder to decode the
// response body.
func Search(ctx context.Context, query string, patternType *string, decoder streamhttp.FrontendStreamDecoder) (err error) {
	tr, ctx := trace.New(ctx, "insights.StreamSearch",
		attribute.String("query", query))
	defer tr.EndWithErr(&err)

	req, err := streamhttp.NewRequest(internalapi.Client.URL+"/.internal", query)
	if err != nil {
		return err
	}
	if patternType != nil {
		query := req.URL.Query()
		query.Add("t", *patternType)
		req.URL.RawQuery = query.Encode()
	}
	// to receive chunk matches we must set this url parameter
	rq := req.URL.Query()
	rq.Add("cm", "t")
	req.URL.RawQuery = rq.Encode()

	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", "code-insights-backend")

	cli, err := httpcli.NewInternalClientFactory("insights_searcher").Doer()
	if err != nil {
		return err
	}

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	decErr := decoder.ReadAll(resp.Body)
	if decErr != nil {
		return decErr
	}
	return err
}

func genericComputeStream(ctx context.Context, handler func(io.Reader) error, query, operation string) (err error) {
	tr, ctx := trace.New(ctx, operation)
	defer tr.EndWithErr(&err)

	req, err := client.NewComputeStreamRequest(internalapi.Client.URL+"/.internal", query)
	if err != nil {
		return err
	}
	req = req.WithContext(ctx)
	req.Header.Set("User-Agent", "code-insights-backend")

	cli, err := httpcli.NewInternalClientFactory("insights_compute").Doer()
	if err != nil {
		return err
	}

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handler(resp.Body)
}

func ComputeMatchContextStream(ctx context.Context, query string, decoder client.ComputeMatchContextStreamDecoder) (err error) {
	return genericComputeStream(ctx, decoder.ReadAll, query, "InsightsComputeStreamSearch")
}

func ComputeTextExtraStream(ctx context.Context, query string, decoder client.ComputeTextExtraStreamDecoder) (err error) {
	return genericComputeStream(ctx, decoder.ReadAll, query, "InsightsComputeTextSearch")
}
