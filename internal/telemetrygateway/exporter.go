package telemetrygateway

import (
	"context"
	"io"
	"net/url"

	"google.golang.org/grpc"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/grpc/chunk"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	telemetrygatewayv1 "github.com/sourcegraph/sourcegraph/internal/telemetrygateway/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Exporter interface {
	ExportEvents(context.Context, []*telemetrygatewayv1.Event) ([]string, error)
	Close() error
}

func NewExporter(ctx context.Context, logger log.Logger, c conftypes.SiteConfigQuerier, exportAddress string) (Exporter, error) {
	u, err := url.Parse(exportAddress)
	if err != nil {
		return nil, errors.Wrap(err, "invalid export address")
	}

	insecureTarget := u.Scheme != "https"
	if insecureTarget && !env.InsecureDev {
		return nil, errors.Wrap(err, "insecure export address used outside of dev mode")
	}

	var opts []grpc.DialOption
	if insecureTarget {
		opts = defaults.DialOptions(logger)
	} else {
		opts = defaults.ExternalDialOptions(logger)
	}
	conn, err := grpc.DialContext(ctx, u.Host, opts...)
	if err != nil {
		return nil, errors.Wrap(err, "dialing telemetry gateway")
	}

	return &exporter{
		client: telemetrygatewayv1.NewTelemeteryGatewayServiceClient(conn),
		conf:   c,
		conn:   conn,
	}, nil
}

type exporter struct {
	client telemetrygatewayv1.TelemeteryGatewayServiceClient
	conf   conftypes.SiteConfigQuerier
	conn   *grpc.ClientConn
}

func (e *exporter) ExportEvents(ctx context.Context, events []*telemetrygatewayv1.Event) ([]string, error) {
	// Start the stream
	stream, err := e.client.RecordEvents(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "start export")
	}

	// Send initial metadata
	if err := stream.Send(&telemetrygatewayv1.RecordEventsRequest{
		Payload: &telemetrygatewayv1.RecordEventsRequest_Metadata{
			Metadata: &telemetrygatewayv1.RecordEventsRequestMetadata{
				LicenseKey: e.conf.SiteConfig().LicenseKey,
			},
		},
	}); err != nil {
		return nil, errors.Wrap(err, "send initial metadata")
	}

	// Set up a callback that makes sure we pick up all responses from the
	// server.
	collectResults := func() ([]string, error) {
		// We're collecting results now - end the request send stream. From here,
		// the server will eventually get io.EOF and return, then we will eventually
		// get an io.EOF and return. Discard the error because we don't really
		// care - in examples, the error gets discarded as well:
		// https://github.com/grpc/grpc-go/blob/130bc4281c39ac1ed287ec988364d36322d3cd34/examples/route_guide/client/client.go#L145
		//
		// If anything goes wrong stream.Recv() will let us know.
		_ = stream.CloseSend()

		succeededEvents := make([]string, 0, len(events))
		var errs error
		for {
			resp, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return succeededEvents, err
			}
			for _, e := range resp.GetSucceededEvents() {
				succeededEvents = append(succeededEvents, e.GetEventId())
			}
			for _, e := range resp.GetFailedEvents() {
				errs = errors.Append(errs, errors.Newf("%s (event %s)",
					e.GetError(), e.GetEventId()))
			}
		}
		return succeededEvents, errs
	}

	// Start streaming our set of events, chunking them based on message size
	// as determined internally by chunk.Chunker.
	chunker := chunk.New(func(chunkedEvents []*telemetrygatewayv1.Event) error {
		return stream.Send(&telemetrygatewayv1.RecordEventsRequest{
			Payload: &telemetrygatewayv1.RecordEventsRequest_Events{
				Events: &telemetrygatewayv1.RecordEventsRequest_EventsPayload{
					Events: chunkedEvents,
				},
			},
		})
	})
	if err := chunker.Send(events...); err != nil {
		succeeded, _ := collectResults()
		return succeeded, errors.Wrap(err, "chunk and send events")
	}
	if err := chunker.Flush(); err != nil {
		succeeded, _ := collectResults()
		return succeeded, errors.Wrap(err, "flush events")
	}

	return collectResults()
}

func (e *exporter) Close() error { return e.conn.Close() }
