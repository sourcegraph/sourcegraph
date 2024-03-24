package graphqlbackend

import (
	"context"

	"github.com/graph-gophers/graphql-go"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var errCodeHostRateLimitsMustBePositiveIntegers = errors.New("rate limit settings must be positive integers")

type codeHostResolver struct {
	ch *types.CodeHost
	db database.DB
}

type SetCodeHostRateLimitsArgs struct {
	Input SetCodeHostRateLimitsInput
}

type SetCodeHostRateLimitsInput struct {
	CodeHostID                      graphql.ID
	APIQuota                        int32
	APIReplenishmentIntervalSeconds int32
	GitQuota                        int32
	GitReplenishmentIntervalSeconds int32
}

func (r *codeHostResolver) ID() graphql.ID {
	return MarshalCodeHostID(r.ch.ID)
}

func (r *codeHostResolver) Kind() string {
	return r.ch.Kind
}

func (r *codeHostResolver) URL() string {
	return r.ch.URL
}

func (r *codeHostResolver) APIRateLimitQuota() *int32 {
	return r.ch.APIRateLimitQuota
}

func (r *codeHostResolver) APIRateLimitIntervalSeconds() *int32 {
	return r.ch.APIRateLimitIntervalSeconds
}

func (r *codeHostResolver) GitRateLimitQuota() *int32 {
	return r.ch.GitRateLimitQuota
}

func (r *codeHostResolver) GitRateLimitIntervalSeconds() *int32 {
	return r.ch.GitRateLimitIntervalSeconds
}

type CodeHostExternalServicesArgs struct {
	First int32
	After *string
}

func (r *codeHostResolver) ExternalServices(args *CodeHostExternalServicesArgs) (*externalServiceConnectionResolver, error) {
	// 🚨 SECURITY: This may only be returned to site-admins, but code host is
	// only accessible to admins anyways.

	var afterID int64
	if args.After != nil {
		var err error
		afterID, err = UnmarshalExternalServiceID(graphql.ID(*args.After))
		if err != nil {
			return nil, err
		}
	}

	opt := database.ExternalServicesListOptions{
		// Only return services of this code host.
		CodeHostID:  r.ch.ID,
		AfterID:     afterID,
		LimitOffset: &database.LimitOffset{Limit: int(args.First)},
	}
	return &externalServiceConnectionResolver{db: r.db, opt: opt}, nil
}

func (r *schemaResolver) SetCodeHostRateLimits(ctx context.Context, args SetCodeHostRateLimitsArgs) (*EmptyResponse, error) {
	// Security 🚨: Code Hosts may only be updated by site admins.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	input := args.Input
	if input.APIQuota < 0 || input.GitQuota < 0 || input.APIReplenishmentIntervalSeconds < 0 || input.GitReplenishmentIntervalSeconds < 0 {
		return nil, errCodeHostRateLimitsMustBePositiveIntegers
	}

	codeHostID, err := UnmarshalCodeHostID(args.Input.CodeHostID)
	if err != nil {
		return nil, errors.Wrap(err, "invalid code host id")
	}

	err = r.db.WithTransact(ctx, func(tx database.DB) (err error) {
		codeHost, err := tx.CodeHosts().GetByID(ctx, codeHostID)
		if err != nil {
			return err
		}
		codeHost.APIRateLimitQuota = &input.APIQuota
		codeHost.APIRateLimitIntervalSeconds = &input.APIReplenishmentIntervalSeconds
		codeHost.GitRateLimitQuota = &input.GitQuota
		codeHost.GitRateLimitIntervalSeconds = &input.GitReplenishmentIntervalSeconds

		err = tx.CodeHosts().Update(ctx, codeHost)
		if err != nil {
			return err
		}
		return nil
	})

	return &EmptyResponse{}, err
}
