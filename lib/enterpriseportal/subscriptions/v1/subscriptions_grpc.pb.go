// This file follows the API design guidelines at https://google.aip.dev/, exceptions are otherwise noted.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.3.0
// - protoc             (unknown)
// source: subscriptions.proto

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

const (
	SubscriptionsService_GetEnterpriseSubscription_FullMethodName          = "/enterpriseportal.subscriptions.v1.SubscriptionsService/GetEnterpriseSubscription"
	SubscriptionsService_ListEnterpriseSubscriptions_FullMethodName        = "/enterpriseportal.subscriptions.v1.SubscriptionsService/ListEnterpriseSubscriptions"
	SubscriptionsService_ListEnterpriseSubscriptionLicenses_FullMethodName = "/enterpriseportal.subscriptions.v1.SubscriptionsService/ListEnterpriseSubscriptionLicenses"
	SubscriptionsService_UpdateEnterpriseSubscription_FullMethodName       = "/enterpriseportal.subscriptions.v1.SubscriptionsService/UpdateEnterpriseSubscription"
	SubscriptionsService_UpdateSubscriptionMembership_FullMethodName       = "/enterpriseportal.subscriptions.v1.SubscriptionsService/UpdateSubscriptionMembership"
)

// SubscriptionsServiceClient is the client API for SubscriptionsService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type SubscriptionsServiceClient interface {
	// GetEnterpriseSubscription retrieves an exact match on an Enterprise subscription.
	GetEnterpriseSubscription(ctx context.Context, in *GetEnterpriseSubscriptionRequest, opts ...grpc.CallOption) (*GetEnterpriseSubscriptionResponse, error)
	// ListEnterpriseSubscriptions queries for Enterprise subscriptions.
	ListEnterpriseSubscriptions(ctx context.Context, in *ListEnterpriseSubscriptionsRequest, opts ...grpc.CallOption) (*ListEnterpriseSubscriptionsResponse, error)
	// ListEnterpriseSubscriptionLicenses queries for licenses associated with
	// Enterprise subscription licenses, with the ability to list licenses across
	// all subscriptions, or just a specific subscription.
	//
	// Each subscription owns a collection of licenses, typically a series of
	// licenses with the most recent one being a subscription's active license.
	ListEnterpriseSubscriptionLicenses(ctx context.Context, in *ListEnterpriseSubscriptionLicensesRequest, opts ...grpc.CallOption) (*ListEnterpriseSubscriptionLicensesResponse, error)
	// UpdateEnterpriseSubscription updates an existing Enterprise subscription.
	// Only properties specified by the update_mask are applied.
	UpdateEnterpriseSubscription(ctx context.Context, in *UpdateEnterpriseSubscriptionRequest, opts ...grpc.CallOption) (*UpdateEnterpriseSubscriptionResponse, error)
	// UpdateSubscriptionMembership updates a subscription membership. It creates
	// a new one if it does not exist and allow_missing is set to true.
	UpdateSubscriptionMembership(ctx context.Context, in *UpdateSubscriptionMembershipRequest, opts ...grpc.CallOption) (*UpdateSubscriptionMembershipResponse, error)
}

type subscriptionsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewSubscriptionsServiceClient(cc grpc.ClientConnInterface) SubscriptionsServiceClient {
	return &subscriptionsServiceClient{cc}
}

func (c *subscriptionsServiceClient) GetEnterpriseSubscription(ctx context.Context, in *GetEnterpriseSubscriptionRequest, opts ...grpc.CallOption) (*GetEnterpriseSubscriptionResponse, error) {
	out := new(GetEnterpriseSubscriptionResponse)
	err := c.cc.Invoke(ctx, SubscriptionsService_GetEnterpriseSubscription_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *subscriptionsServiceClient) ListEnterpriseSubscriptions(ctx context.Context, in *ListEnterpriseSubscriptionsRequest, opts ...grpc.CallOption) (*ListEnterpriseSubscriptionsResponse, error) {
	out := new(ListEnterpriseSubscriptionsResponse)
	err := c.cc.Invoke(ctx, SubscriptionsService_ListEnterpriseSubscriptions_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *subscriptionsServiceClient) ListEnterpriseSubscriptionLicenses(ctx context.Context, in *ListEnterpriseSubscriptionLicensesRequest, opts ...grpc.CallOption) (*ListEnterpriseSubscriptionLicensesResponse, error) {
	out := new(ListEnterpriseSubscriptionLicensesResponse)
	err := c.cc.Invoke(ctx, SubscriptionsService_ListEnterpriseSubscriptionLicenses_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *subscriptionsServiceClient) UpdateEnterpriseSubscription(ctx context.Context, in *UpdateEnterpriseSubscriptionRequest, opts ...grpc.CallOption) (*UpdateEnterpriseSubscriptionResponse, error) {
	out := new(UpdateEnterpriseSubscriptionResponse)
	err := c.cc.Invoke(ctx, SubscriptionsService_UpdateEnterpriseSubscription_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *subscriptionsServiceClient) UpdateSubscriptionMembership(ctx context.Context, in *UpdateSubscriptionMembershipRequest, opts ...grpc.CallOption) (*UpdateSubscriptionMembershipResponse, error) {
	out := new(UpdateSubscriptionMembershipResponse)
	err := c.cc.Invoke(ctx, SubscriptionsService_UpdateSubscriptionMembership_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// SubscriptionsServiceServer is the server API for SubscriptionsService service.
// All implementations must embed UnimplementedSubscriptionsServiceServer
// for forward compatibility
type SubscriptionsServiceServer interface {
	// GetEnterpriseSubscription retrieves an exact match on an Enterprise subscription.
	GetEnterpriseSubscription(context.Context, *GetEnterpriseSubscriptionRequest) (*GetEnterpriseSubscriptionResponse, error)
	// ListEnterpriseSubscriptions queries for Enterprise subscriptions.
	ListEnterpriseSubscriptions(context.Context, *ListEnterpriseSubscriptionsRequest) (*ListEnterpriseSubscriptionsResponse, error)
	// ListEnterpriseSubscriptionLicenses queries for licenses associated with
	// Enterprise subscription licenses, with the ability to list licenses across
	// all subscriptions, or just a specific subscription.
	//
	// Each subscription owns a collection of licenses, typically a series of
	// licenses with the most recent one being a subscription's active license.
	ListEnterpriseSubscriptionLicenses(context.Context, *ListEnterpriseSubscriptionLicensesRequest) (*ListEnterpriseSubscriptionLicensesResponse, error)
	// UpdateEnterpriseSubscription updates an existing Enterprise subscription.
	// Only properties specified by the update_mask are applied.
	UpdateEnterpriseSubscription(context.Context, *UpdateEnterpriseSubscriptionRequest) (*UpdateEnterpriseSubscriptionResponse, error)
	// UpdateSubscriptionMembership updates a subscription membership. It creates
	// a new one if it does not exist and allow_missing is set to true.
	UpdateSubscriptionMembership(context.Context, *UpdateSubscriptionMembershipRequest) (*UpdateSubscriptionMembershipResponse, error)
	mustEmbedUnimplementedSubscriptionsServiceServer()
}

// UnimplementedSubscriptionsServiceServer must be embedded to have forward compatible implementations.
type UnimplementedSubscriptionsServiceServer struct {
}

func (UnimplementedSubscriptionsServiceServer) GetEnterpriseSubscription(context.Context, *GetEnterpriseSubscriptionRequest) (*GetEnterpriseSubscriptionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetEnterpriseSubscription not implemented")
}
func (UnimplementedSubscriptionsServiceServer) ListEnterpriseSubscriptions(context.Context, *ListEnterpriseSubscriptionsRequest) (*ListEnterpriseSubscriptionsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListEnterpriseSubscriptions not implemented")
}
func (UnimplementedSubscriptionsServiceServer) ListEnterpriseSubscriptionLicenses(context.Context, *ListEnterpriseSubscriptionLicensesRequest) (*ListEnterpriseSubscriptionLicensesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListEnterpriseSubscriptionLicenses not implemented")
}
func (UnimplementedSubscriptionsServiceServer) UpdateEnterpriseSubscription(context.Context, *UpdateEnterpriseSubscriptionRequest) (*UpdateEnterpriseSubscriptionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateEnterpriseSubscription not implemented")
}
func (UnimplementedSubscriptionsServiceServer) UpdateSubscriptionMembership(context.Context, *UpdateSubscriptionMembershipRequest) (*UpdateSubscriptionMembershipResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateSubscriptionMembership not implemented")
}
func (UnimplementedSubscriptionsServiceServer) mustEmbedUnimplementedSubscriptionsServiceServer() {}

// UnsafeSubscriptionsServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to SubscriptionsServiceServer will
// result in compilation errors.
type UnsafeSubscriptionsServiceServer interface {
	mustEmbedUnimplementedSubscriptionsServiceServer()
}

func RegisterSubscriptionsServiceServer(s grpc.ServiceRegistrar, srv SubscriptionsServiceServer) {
	s.RegisterService(&SubscriptionsService_ServiceDesc, srv)
}

func _SubscriptionsService_GetEnterpriseSubscription_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetEnterpriseSubscriptionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SubscriptionsServiceServer).GetEnterpriseSubscription(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SubscriptionsService_GetEnterpriseSubscription_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SubscriptionsServiceServer).GetEnterpriseSubscription(ctx, req.(*GetEnterpriseSubscriptionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SubscriptionsService_ListEnterpriseSubscriptions_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListEnterpriseSubscriptionsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SubscriptionsServiceServer).ListEnterpriseSubscriptions(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SubscriptionsService_ListEnterpriseSubscriptions_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SubscriptionsServiceServer).ListEnterpriseSubscriptions(ctx, req.(*ListEnterpriseSubscriptionsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SubscriptionsService_ListEnterpriseSubscriptionLicenses_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListEnterpriseSubscriptionLicensesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SubscriptionsServiceServer).ListEnterpriseSubscriptionLicenses(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SubscriptionsService_ListEnterpriseSubscriptionLicenses_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SubscriptionsServiceServer).ListEnterpriseSubscriptionLicenses(ctx, req.(*ListEnterpriseSubscriptionLicensesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SubscriptionsService_UpdateEnterpriseSubscription_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateEnterpriseSubscriptionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SubscriptionsServiceServer).UpdateEnterpriseSubscription(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SubscriptionsService_UpdateEnterpriseSubscription_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SubscriptionsServiceServer).UpdateEnterpriseSubscription(ctx, req.(*UpdateEnterpriseSubscriptionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _SubscriptionsService_UpdateSubscriptionMembership_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateSubscriptionMembershipRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(SubscriptionsServiceServer).UpdateSubscriptionMembership(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: SubscriptionsService_UpdateSubscriptionMembership_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(SubscriptionsServiceServer).UpdateSubscriptionMembership(ctx, req.(*UpdateSubscriptionMembershipRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// SubscriptionsService_ServiceDesc is the grpc.ServiceDesc for SubscriptionsService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var SubscriptionsService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "enterpriseportal.subscriptions.v1.SubscriptionsService",
	HandlerType: (*SubscriptionsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetEnterpriseSubscription",
			Handler:    _SubscriptionsService_GetEnterpriseSubscription_Handler,
		},
		{
			MethodName: "ListEnterpriseSubscriptions",
			Handler:    _SubscriptionsService_ListEnterpriseSubscriptions_Handler,
		},
		{
			MethodName: "ListEnterpriseSubscriptionLicenses",
			Handler:    _SubscriptionsService_ListEnterpriseSubscriptionLicenses_Handler,
		},
		{
			MethodName: "UpdateEnterpriseSubscription",
			Handler:    _SubscriptionsService_UpdateEnterpriseSubscription_Handler,
		},
		{
			MethodName: "UpdateSubscriptionMembership",
			Handler:    _SubscriptionsService_UpdateSubscriptionMembership_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "subscriptions.proto",
}
