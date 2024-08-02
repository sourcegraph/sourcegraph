// Copyright (c) The go-grpc-middleware Authors.
// Licensed under the Apache License 2.0.

package testpb

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestPingServiceOnWire(t *testing.T) {
	stopped := make(chan error)
	serverListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "must be able to allocate a port for serverListener")

	server := grpc.NewServer()
	RegisterTestServiceServer(server, &TestPingService{})

	go func() {
		defer close(stopped)
		stopped <- server.Serve(serverListener)
	}()
	defer func() {
		server.Stop()
		<-stopped
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This is the point where we hook up the interceptor.
	clientConn, err := grpc.DialContext(
		ctx,
		serverListener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	require.NoError(t, err, "must not error on client Dial")

	testClient := NewTestServiceClient(clientConn)
	select {
	case err := <-stopped:
		t.Fatal("gRPC server stopped prematurely", err)
	default:
	}

	r, err := testClient.PingEmpty(context.Background(), &PingEmptyRequest{})
	require.NoError(t, err)
	require.NotNil(t, r)

	r2, err := testClient.Ping(context.Background(), &PingRequest{Value: "24"})
	require.NoError(t, err)
	require.Equal(t, "24", r2.Value)
	require.Equal(t, int32(0), r2.Counter)

	_, err = testClient.PingError(context.Background(), &PingErrorRequest{
		ErrorCodeReturned: uint32(codes.Internal),
		Value:             "24",
	})
	require.Error(t, err)
	require.Equal(t, codes.Internal, status.Code(err))

	l, err := testClient.PingList(context.Background(), &PingListRequest{Value: "24"})
	require.NoError(t, err)
	for i := 0; i < ListResponseCount; i++ {
		r, err := l.Recv()
		require.NoError(t, err)
		require.Equal(t, "24", r.Value)
		require.Equal(t, int32(i), r.Counter)
	}

	s, err := testClient.PingStream(context.Background())
	require.NoError(t, err)
	for i := 0; i < ListResponseCount; i++ {
		require.NoError(t, s.Send(&PingStreamRequest{Value: fmt.Sprintf("%v", i)}))

		r, err := s.Recv()
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf("%v", i), r.Value)
		require.Equal(t, int32(i), r.Counter)
	}

	select {
	case err := <-stopped:
		t.Fatal("gRPC server stopped prematurely", err)
	default:
	}
}
