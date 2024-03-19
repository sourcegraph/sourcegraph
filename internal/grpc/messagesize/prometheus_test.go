package messagesize

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	newspb "github.com/sourcegraph/sourcegraph/internal/grpc/testprotos/news/v1"
)

var (
	binaryMessage = &newspb.BinaryAttachment{
		Name: "data",
		Data: []byte(strings.Repeat("x", 1*1024*1024)),
	}

	keyValueMessage = &newspb.KeyValueAttachment{
		Name: "data",
		Data: map[string]string{
			"key1": strings.Repeat("x", 1*1024*1024),
			"key2": "value2",
		},
	}

	articleMessage = &newspb.Article{
		Author:  "author",
		Date:    &timestamppb.Timestamp{Seconds: 1234567890},
		Title:   "title",
		Content: "content",
		Status:  newspb.Article_STATUS_PUBLISHED,
		Attachments: []*newspb.Attachment{
			{Contents: &newspb.Attachment_KeyValueAttachment{KeyValueAttachment: keyValueMessage}},
			{Contents: &newspb.Attachment_KeyValueAttachment{KeyValueAttachment: keyValueMessage}},
			{Contents: &newspb.Attachment_BinaryAttachment{BinaryAttachment: binaryMessage}},
			{Contents: &newspb.Attachment_BinaryAttachment{BinaryAttachment: binaryMessage}},
		},
	}
)

func BenchmarkObserverBinary(b *testing.B) {
	o := messageSizeObserver{
		onSingleFunc: func(messageSizeBytes uint64) {},
		onFinishFunc: func(totalSizeBytes uint64) {},
	}

	benchmarkObserver(b, &o, binaryMessage)
}

func BenchmarkObserverKeyValue(b *testing.B) {
	o := messageSizeObserver{
		onSingleFunc: func(messageSizeBytes uint64) {},
		onFinishFunc: func(totalSizeBytes uint64) {},
	}

	benchmarkObserver(b, &o, keyValueMessage)
}

func BenchmarkObserverArticle(b *testing.B) {
	o := messageSizeObserver{
		onSingleFunc: func(messageSizeBytes uint64) {},
		onFinishFunc: func(totalSizeBytes uint64) {},
	}

	benchmarkObserver(b, &o, articleMessage)
}

func benchmarkObserver(b *testing.B, observer *messageSizeObserver, message proto.Message) {
	b.ReportAllocs()

	for range b.N {
		observer.Observe(message)
	}

	observer.FinishRPC()
}

func TestUnaryServerInterceptor(t *testing.T) {
	ctx := context.Background()

	request := &newspb.BinaryAttachment{
		Data: bytes.Repeat([]byte("request"), 3),
	}

	response := &newspb.BinaryAttachment{
		Data: bytes.Repeat([]byte("response"), 7),
	}

	info := &grpc.UnaryServerInfo{
		FullMethod: "news.v1.NewsService/GetArticle",
	}

	sentinelError := errors.New("expected error")

	tests := []struct {
		name           string
		handler        func(ctx context.Context, req any) (any, error)
		expectedError  error
		expectedResult any
		expectedSize   uint64
	}{
		{
			name: "invoker successful - observe response",
			handler: func(ctx context.Context, req any) (any, error) {
				return response, nil
			},
			expectedError:  nil,
			expectedResult: response,
			expectedSize:   uint64(proto.Size(response)),
		},
		{
			name: "invoker error - observe a zero-sized response",
			handler: func(ctx context.Context, req any) (any, error) {
				return nil, sentinelError
			},
			expectedError:  sentinelError,
			expectedResult: nil,
			expectedSize:   uint64(0),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			onFinishCalledCount := 0

			observer := messageSizeObserver{
				onSingleFunc: func(messageSizeBytes uint64) {},
				onFinishFunc: func(totalSizeBytes uint64) {
					onFinishCalledCount++

					if diff := cmp.Diff(totalSizeBytes, test.expectedSize); diff != "" {
						t.Error("totalSizeBytes mismatch (-want +got):\n", diff)
					}
				},
			}

			actualResult, err := unaryServerInterceptor(&observer, request, ctx, info, test.handler)
			if err != test.expectedError {
				t.Errorf("error mismatch (wanted: %q, got: %q)", test.expectedError, err)
			}

			if diff := cmp.Diff(test.expectedResult, actualResult, protocmp.Transform()); diff != "" {
				t.Error("response mismatch (-want +got):\n", diff)
			}

			if diff := cmp.Diff(1, onFinishCalledCount); diff != "" {
				t.Error("onFinishFunc not called expected number of times (-want +got):\n", diff)
			}
		})
	}
}

func TestStreamServerInterceptor(t *testing.T) {
	response1 := &newspb.BinaryAttachment{
		Name: "",
		Data: []byte("response"),
	}
	response2 := &newspb.BinaryAttachment{
		Name: "",
		Data: bytes.Repeat([]byte("response"), 3),
	}
	response3 := &newspb.BinaryAttachment{
		Name: "",
		Data: bytes.Repeat([]byte("response"), 7),
	}

	info := &grpc.StreamServerInfo{
		FullMethod: "news.v1.NewsService/GetArticle",
	}

	sentinelError := errors.New("expected error")

	tests := []struct {
		name string

		mockSendMsg func(m any) error
		handler     func(srv any, stream grpc.ServerStream) error

		expectedError     error
		expectedResponses []any
		expectedSize      uint64
	}{
		{
			name: "invoker successful - observe all 3 responses",

			mockSendMsg: func(m any) error {
				return nil // no error
			},

			handler: func(srv any, stream grpc.ServerStream) error {
				for _, r := range []proto.Message{response1, response2, response3} {
					if err := stream.SendMsg(r); err != nil {
						return err
					}
				}

				return nil
			},

			expectedError:     nil,
			expectedResponses: []any{response1, response2, response3},
			expectedSize:      uint64(proto.Size(response1) + proto.Size(response2) + proto.Size(response3)),
		},

		{
			name: "invoker fails on 3rd response - only observe first 2",

			mockSendMsg: func(m any) error {
				if m == response3 {
					return sentinelError
				}

				return nil
			},
			handler: func(srv any, stream grpc.ServerStream) error {
				for _, r := range []proto.Message{response1, response2, response3} {
					if err := stream.SendMsg(r); err != nil {
						return err
					}
				}

				return nil
			},

			expectedError:     sentinelError,
			expectedResponses: []any{response1, response2, response3},                // response 3 should still be attempted to be sent
			expectedSize:      uint64(proto.Size(response1) + proto.Size(response2)), // response 3 should not be counted since an error occurred while sending it
		},

		{
			name: "invoker fails immediately - should still observe a zero-sized response",

			mockSendMsg: func(m any) error {
				return errors.New("should not be called")
			},

			handler: func(srv any, stream grpc.ServerStream) error {
				return sentinelError
			},

			expectedError:     sentinelError,
			expectedResponses: []any{},   // there are no responses
			expectedSize:      uint64(0), // there are no responses, so the size is 0
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			onFinishCallCount := 0

			observer := messageSizeObserver{
				onSingleFunc: func(messageSizeBytes uint64) {},
				onFinishFunc: func(totalSizeBytes uint64) {
					onFinishCallCount++

					if totalSizeBytes != test.expectedSize {
						t.Errorf("totalSizeBytes mismatch (wanted: %d, got: %d)", test.expectedSize, totalSizeBytes)
					}
				},
			}

			var actualResponses []any

			ss := &mockServerStream{
				mockSendMsg: func(m any) error {
					actualResponses = append(actualResponses, m)

					return test.mockSendMsg(m)
				},
			}

			err := streamServerInterceptor(&observer, nil, ss, info, test.handler)
			if err != test.expectedError {
				t.Errorf("error mismatch (wanted: %q, got: %q)", test.expectedError, err)
			}

			if diff := cmp.Diff(test.expectedResponses, actualResponses, protocmp.Transform(), cmpopts.EquateEmpty()); diff != "" {
				t.Error("responses mismatch (-want +got):\n", diff)
			}

			if diff := cmp.Diff(1, onFinishCallCount); diff != "" {
				t.Error("onFinishFunc not called expected number of times (-want +got):\n", diff)
			}
		})
	}
}

func TestUnaryClientInterceptor(t *testing.T) {
	ctx := context.Background()

	request := &newspb.BinaryAttachment{
		Name: "data",
		Data: bytes.Repeat([]byte("request"), 3),
	}

	method := "news.v1.NewsService/GetArticle"

	sentinelError := errors.New("expected error")

	tests := []struct {
		name    string
		invoker grpc.UnaryInvoker

		expectedError   error
		expectedRequest any
		expectedSize    uint64
	}{
		{
			name: "invoker successful - observe request size",
			invoker: func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return nil
			},

			expectedError:   nil,
			expectedRequest: request,
			expectedSize:    uint64(proto.Size(request)),
		},

		{
			name: "invoker error - observe a zero-sized response",
			invoker: func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				return sentinelError
			},

			expectedError:   sentinelError,
			expectedRequest: request,
			expectedSize:    uint64(0),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			onFinishCallCount := 0

			observer := messageSizeObserver{
				onSingleFunc: func(messageSizeBytes uint64) {},
				onFinishFunc: func(totalSizeBytes uint64) {
					onFinishCallCount++

					if diff := cmp.Diff(totalSizeBytes, test.expectedSize); diff != "" {
						t.Error("totalSizeBytes mismatch (-want +got):\n", diff)
					}
				},
			}

			var actualRequest any

			invokerCalled := false
			invoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
				invokerCalled = true

				actualRequest = req
				return test.invoker(ctx, method, req, reply, cc, opts...)
			}

			err := unaryClientInterceptor(&observer, ctx, method, request, nil, nil, invoker)
			if err != test.expectedError {
				t.Errorf("error mismatch (wanted: %q, got: %q)", test.expectedError, err)
			}

			if !invokerCalled {
				t.Fatal("invoker not called")
			}

			if diff := cmp.Diff(test.expectedRequest, actualRequest, protocmp.Transform()); diff != "" {
				t.Error("request mismatch (-want +got):\n", diff)
			}

			if diff := cmp.Diff(1, onFinishCallCount); diff != "" {
				t.Error("onFinishFunc not called expected number of times (-want +got):\n", diff)
			}
		})
	}
}

func TestStreamingClientInterceptor(t *testing.T) {
	ctx := context.Background()

	request1 := &newspb.BinaryAttachment{
		Name: "data",
		Data: bytes.Repeat([]byte("request"), 3),
	}

	request2 := &newspb.BinaryAttachment{
		Name: "data",
		Data: bytes.Repeat([]byte("request"), 7),
	}

	request3 := &newspb.BinaryAttachment{
		Name: "data",
		Data: bytes.Repeat([]byte("request"), 13),
	}

	method := "news.v1.NewsService/GetArticle"

	sentinelError := errors.New("expected error")

	type stepType int

	const (
		stepSend stepType = iota
		stepRecv
		stepCloseSend
	)

	type step struct {
		stepType stepType

		message   any
		streamErr error
	}

	tests := []struct {
		name string

		steps        []step
		expectedSize uint64
	}{
		{
			name: "invoker successful - observe request size",
			steps: []step{
				{
					stepType: stepSend,

					message:   request1,
					streamErr: nil,
				},
				{
					stepType: stepSend,

					message:   request2,
					streamErr: nil,
				},
				{
					stepType: stepSend,

					message:   request3,
					streamErr: nil,
				},
				{
					stepType: stepRecv,

					message:   nil,
					streamErr: io.EOF, // end of stream
				},
			},

			expectedSize: uint64(proto.Size(request1) + proto.Size(request2) + proto.Size(request3)),
		},
		{
			name: "2nd send failed - stream aborts and should only observe first request",
			steps: []step{
				{
					stepType:  stepSend,
					message:   request1,
					streamErr: nil,
				},
				{
					stepType:  stepSend,
					message:   request2,
					streamErr: sentinelError,
				},
			},

			expectedSize: uint64(proto.Size(request1)),
		},
		{
			name: "recv message fails with non io.EOF error - should still observe all requests",
			steps: []step{
				{
					stepType: stepSend,

					message:   request1,
					streamErr: nil,
				},
				{
					stepType: stepSend,

					message:   request2,
					streamErr: nil,
				},
				{
					stepType: stepSend,

					message:   request3,
					streamErr: nil,
				},
				{
					stepType: stepRecv,

					message:   nil,
					streamErr: sentinelError,
				},
			},

			expectedSize: uint64(proto.Size(request1) + proto.Size(request2) + proto.Size(request3)),
		},

		{
			name: "close send called - should  observe all requests",
			steps: []step{
				{
					stepType: stepSend,

					message:   request1,
					streamErr: nil,
				},
				{
					stepType: stepSend,

					message:   request2,
					streamErr: nil,
				},
				{
					stepType: stepSend,

					message:   request3,
					streamErr: nil,
				},
				{
					stepType: stepCloseSend,

					message:   nil,
					streamErr: nil,
				},
			},

			expectedSize: uint64(proto.Size(request1) + proto.Size(request2) + proto.Size(request3)),
		},
		{
			name: "close send called immediately - should observe zero-sized response",
			steps: []step{
				{
					stepType: stepCloseSend,

					message:   nil,
					streamErr: nil,
				},
			},

			expectedSize: uint64(0),
		},
		{
			name: "first send fails - stream should abort and observe zero-sized response",
			steps: []step{
				{
					stepType: stepSend,

					message:   request1,
					streamErr: sentinelError,
				},
			},

			expectedSize: uint64(0),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			onFinishCallCount := 0

			observer := messageSizeObserver{
				onSingleFunc: func(messageSizeBytes uint64) {},
				onFinishFunc: func(totalSizeBytes uint64) {
					onFinishCallCount++

					if diff := cmp.Diff(totalSizeBytes, test.expectedSize); diff != "" {
						t.Error("totalSizeBytes mismatch (-want +got):\n", diff)
					}
				},
			}

			baseStream := &mockClientStream{}
			streamerCalled := false
			streamer := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
				streamerCalled = true

				return baseStream, nil
			}

			ss, err := streamClientInterceptor(&observer, ctx, nil, nil, method, streamer)
			require.NoError(t, err)

			// Run through all the steps, preparing the mockClientStream to return the expected errors
			for _, step := range test.steps {
				baseStreamCalled := false
				var streamErr error

				switch step.stepType {
				case stepSend:
					baseStream.mockSendMsg = func(m any) error {
						baseStreamCalled = true
						return step.streamErr
					}

					streamErr = ss.SendMsg(step.message)
				case stepRecv:
					baseStream.mockRecvMsg = func(_ any) error {
						baseStreamCalled = true
						return step.streamErr
					}

					streamErr = ss.RecvMsg(step.message)

				case stepCloseSend:
					baseStream.mockCloseSend = func() error {
						baseStreamCalled = true
						return step.streamErr
					}

					streamErr = ss.CloseSend()
				default:
					t.Fatalf("unknown step type: %v", step.stepType)
				}

				// ensure that the baseStream was called and errors are propagated
				require.True(t, baseStreamCalled)
				require.Equal(t, step.streamErr, streamErr)
			}

			if !streamerCalled {
				t.Fatal("streamer not called")
			}

			if diff := cmp.Diff(1, onFinishCallCount); diff != "" {
				t.Error("onFinishFunc not called expected number of times (-want +got):\n", diff)
			}
		})
	}
}

func TestObserver(t *testing.T) {
	testCases := []struct {
		name     string
		messages []proto.Message
	}{
		{
			name: "single message",
			messages: []proto.Message{&newspb.BinaryAttachment{
				Name: "data1",
				Data: []byte("sample data"),
			}},
		},
		{
			name: "multiple messages",
			messages: []proto.Message{
				&newspb.BinaryAttachment{
					Name: "data1",
					Data: []byte("sample data"),
				},
				&newspb.KeyValueAttachment{
					Name: "data2",
					Data: map[string]string{
						"key1": "value1",
						"key2": "value2",
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var singleMessageSizes []uint64
			var totalSize uint64

			// Create a new observer with custom onSingleFunc and onFinishFunc
			obs := &messageSizeObserver{
				onSingleFunc: func(messageSizeBytes uint64) {
					singleMessageSizes = append(singleMessageSizes, messageSizeBytes)
				},
				onFinishFunc: func(totalSizeBytes uint64) {
					totalSize = totalSizeBytes
				},
			}

			// Call ObserveSingle for each message
			for _, msg := range tc.messages {
				obs.Observe(msg)
			}

			// Check that the singleMessageSizes are correct
			for i, msg := range tc.messages {
				expectedSize := uint64(proto.Size(msg))
				require.Equal(t, expectedSize, singleMessageSizes[i])
			}

			// Call FinishRPC
			obs.FinishRPC()

			// Check that the totalSize is correct
			expectedTotalSize := uint64(0)
			for _, size := range singleMessageSizes {
				expectedTotalSize += size
			}
			require.EqualValues(t, expectedTotalSize, totalSize)
		})
	}
}

type mockServerStream struct {
	mockSendMsg func(m any) error

	grpc.ServerStream
}

func (s *mockServerStream) SendMsg(m any) error {
	if s.mockSendMsg != nil {
		return s.mockSendMsg(m)
	}

	return errors.New("send msg not implemented")
}

type mockClientStream struct {
	mockRecvMsg   func(m any) error
	mockSendMsg   func(m any) error
	mockCloseSend func() error

	grpc.ClientStream
}

func (s *mockClientStream) SendMsg(m any) error {
	if s.mockSendMsg != nil {
		return s.mockSendMsg(m)
	}

	return errors.New("send msg not implemented")
}

func (s *mockClientStream) RecvMsg(m any) error {
	if s.mockRecvMsg != nil {
		return s.mockRecvMsg(m)
	}

	return errors.New("recv msg not implemented")
}

func (s *mockClientStream) CloseSend() error {
	if s.mockCloseSend != nil {
		return s.mockCloseSend()
	}

	return errors.New("close send not implemented")
}

var (
	_ grpc.ServerStream = &mockServerStream{}
	_ grpc.ClientStream = &mockClientStream{}
)
