package proxy

import (
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// StreamPeeker abstracts away the gRPC stream being forwarded so that it can
// be inspected and modified.
type StreamPeeker interface {
	// Peek allows a director to peek one message into the request stream without
	// removing those messages from the stream that will be forwarded to
	// the backend server.
	Peek() (frame *emptypb.Empty, _ error)
}

type partialStream struct {
	frames []*emptypb.Empty // frames encountered in partial stream
}

type peeker struct {
	srcStream      grpc.ServerStream
	consumedStream *partialStream
}

func newPeeker(stream grpc.ServerStream) *peeker {
	return &peeker{
		srcStream:      stream,
		consumedStream: &partialStream{},
	}
}

// ErrInvalidPeekCount indicates the director function requested an invalid
// peek quantity
var ErrInvalidPeekCount = errors.New("peek count must be greater than zero")

func (p peeker) Peek() (*emptypb.Empty, error) {
	payloads, err := p.peek(1)
	if err != nil {
		return nil, err
	}

	if len(payloads) != 1 {
		return nil, errors.New("failed to peek 1 message")
	}

	return payloads[0], nil
}

func (p peeker) peek(n uint) ([]*emptypb.Empty, error) {
	if n < 1 {
		return nil, ErrInvalidPeekCount
	}

	p.consumedStream.frames = make([]*emptypb.Empty, n)
	peekedFrames := make([]*emptypb.Empty, n)

	for i := 0; i < len(p.consumedStream.frames); i++ {
		f := &emptypb.Empty{}
		if err := p.srcStream.RecvMsg(f); err != nil {
			return nil, err
		}
		p.consumedStream.frames[i] = f
		peekedFrames[i] = f
	}

	return peekedFrames, nil
}
