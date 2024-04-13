package v1

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (r *GitserverRepository) Validate() error {
	if r == nil {
		return status.New(codes.InvalidArgument, "target_repository must be specified").Err()
	}
	if r.GetUid() == "" {
		return status.New(codes.InvalidArgument, "target_repository.uid must be specified").Err()
	}
	if r.GetName() == "" {
		return status.New(codes.InvalidArgument, "target_repository.name must be specified").Err()
	}
	if r.GetPath() == "" {
		return status.New(codes.InvalidArgument, "target_repository.path must be specified").Err()
	}
	return nil
}
