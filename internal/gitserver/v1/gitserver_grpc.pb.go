// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             (unknown)
// source: gitserver.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// GitserverServiceClient is the client API for GitserverService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type GitserverServiceClient interface {
	Exec(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (GitserverService_ExecClient, error)
	Search(ctx context.Context, in *SearchRequest, opts ...grpc.CallOption) (GitserverService_SearchClient, error)
}

type gitserverServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewGitserverServiceClient(cc grpc.ClientConnInterface) GitserverServiceClient {
	return &gitserverServiceClient{cc}
}

func (c *gitserverServiceClient) Exec(ctx context.Context, in *ExecRequest, opts ...grpc.CallOption) (GitserverService_ExecClient, error) {
	stream, err := c.cc.NewStream(ctx, &GitserverService_ServiceDesc.Streams[0], "/gitserver.v1.GitserverService/Exec", opts...)
	if err != nil {
		return nil, err
	}
	x := &gitserverServiceExecClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type GitserverService_ExecClient interface {
	Recv() (*ExecResponse, error)
	grpc.ClientStream
}

type gitserverServiceExecClient struct {
	grpc.ClientStream
}

func (x *gitserverServiceExecClient) Recv() (*ExecResponse, error) {
	m := new(ExecResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *gitserverServiceClient) Search(ctx context.Context, in *SearchRequest, opts ...grpc.CallOption) (GitserverService_SearchClient, error) {
	stream, err := c.cc.NewStream(ctx, &GitserverService_ServiceDesc.Streams[1], "/gitserver.v1.GitserverService/Search", opts...)
	if err != nil {
		return nil, err
	}
	x := &gitserverServiceSearchClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type GitserverService_SearchClient interface {
	Recv() (*SearchResponse, error)
	grpc.ClientStream
}

type gitserverServiceSearchClient struct {
	grpc.ClientStream
}

func (x *gitserverServiceSearchClient) Recv() (*SearchResponse, error) {
	m := new(SearchResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GitserverServiceServer is the server API for GitserverService service.
// All implementations must embed UnimplementedGitserverServiceServer
// for forward compatibility
type GitserverServiceServer interface {
	Exec(*ExecRequest, GitserverService_ExecServer) error
	Search(*SearchRequest, GitserverService_SearchServer) error
	mustEmbedUnimplementedGitserverServiceServer()
}

// UnimplementedGitserverServiceServer must be embedded to have forward compatible implementations.
type UnimplementedGitserverServiceServer struct {
}

func (UnimplementedGitserverServiceServer) Exec(*ExecRequest, GitserverService_ExecServer) error {
	return status.Errorf(codes.Unimplemented, "method Exec not implemented")
}
func (UnimplementedGitserverServiceServer) Search(*SearchRequest, GitserverService_SearchServer) error {
	return status.Errorf(codes.Unimplemented, "method Search not implemented")
}
func (UnimplementedGitserverServiceServer) mustEmbedUnimplementedGitserverServiceServer() {}

// UnsafeGitserverServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GitserverServiceServer will
// result in compilation errors.
type UnsafeGitserverServiceServer interface {
	mustEmbedUnimplementedGitserverServiceServer()
}

func RegisterGitserverServiceServer(s grpc.ServiceRegistrar, srv GitserverServiceServer) {
	s.RegisterService(&GitserverService_ServiceDesc, srv)
}

func _GitserverService_Exec_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ExecRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(GitserverServiceServer).Exec(m, &gitserverServiceExecServer{stream})
}

type GitserverService_ExecServer interface {
	Send(*ExecResponse) error
	grpc.ServerStream
}

type gitserverServiceExecServer struct {
	grpc.ServerStream
}

func (x *gitserverServiceExecServer) Send(m *ExecResponse) error {
	return x.ServerStream.SendMsg(m)
}

func _GitserverService_Search_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SearchRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(GitserverServiceServer).Search(m, &gitserverServiceSearchServer{stream})
}

type GitserverService_SearchServer interface {
	Send(*SearchResponse) error
	grpc.ServerStream
}

type gitserverServiceSearchServer struct {
	grpc.ServerStream
}

func (x *gitserverServiceSearchServer) Send(m *SearchResponse) error {
	return x.ServerStream.SendMsg(m)
}

// GitserverService_ServiceDesc is the grpc.ServiceDesc for GitserverService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var GitserverService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "gitserver.v1.GitserverService",
	HandlerType: (*GitserverServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Exec",
			Handler:       _GitserverService_Exec_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "Search",
			Handler:       _GitserverService_Search_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "gitserver.proto",
}
