package graphqlbackend

import (
	"context"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/stretchr/testify/require"
)

func TestSchemaResolver_SetCodeHostRateLimits_NotASiteAdmin(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbmocks.NewMockDB()
	r := &schemaResolver{logger: logger, db: db}

	usersStore := dbmocks.NewMockUserStore()
	usersStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: false}, nil)
	db.UsersFunc.SetDefaultReturn(usersStore)

	_, err := r.SetCodeHostRateLimits(context.Background(), SetCodeHostRateLimitsArgs{
		Input: SetCodeHostRateLimitsInput{},
	})
	require.NotNil(t, err)
	require.Equal(t, auth.ErrMustBeSiteAdmin, err)
}

func TestSchemaResolver_SetCodeHostRateLimits_InvalidConfigs(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbmocks.NewMockDB()
	r := &schemaResolver{logger: logger, db: db}
	ctx := context.Background()
	wantErr := errors.New("rate limit settings must be positive integers")

	tests := []struct {
		name    string
		args    SetCodeHostRateLimitsArgs
		wantErr error
	}{
		{
			name: "Negative APIQuota",
			args: SetCodeHostRateLimitsArgs{
				Input: SetCodeHostRateLimitsInput{
					CodeHostID: "Q29kZUhvc3Q6MQ==",
					APIQuota:   -1,
				},
			},
			wantErr: wantErr,
		},
		{
			name: "Negative APIReplenishmentIntervalSeconds",
			args: SetCodeHostRateLimitsArgs{
				Input: SetCodeHostRateLimitsInput{
					CodeHostID:                      "Q29kZUhvc3Q6MQ==",
					APIReplenishmentIntervalSeconds: -1,
				},
			},
			wantErr: wantErr,
		},
		{
			name: "Negative GitQuota",
			args: SetCodeHostRateLimitsArgs{
				Input: SetCodeHostRateLimitsInput{
					CodeHostID: "Q29kZUhvc3Q6MQ==",
					GitQuota:   -1,
				},
			},
			wantErr: wantErr,
		},
		{
			name: "Negative GitReplenishmentIntervalSeconds",
			args: SetCodeHostRateLimitsArgs{
				Input: SetCodeHostRateLimitsInput{
					CodeHostID:                      "Q29kZUhvc3Q6MQ==",
					GitReplenishmentIntervalSeconds: -1,
				},
			},
			wantErr: wantErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			usersStore := dbmocks.NewMockUserStore()
			usersStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
			db.UsersFunc.SetDefaultReturn(usersStore)

			_, err := r.SetCodeHostRateLimits(ctx, test.args)
			require.NotNil(t, err)
			require.Equal(t, errCodeHostRateLimitsMustBePositiveIntegers, err)
		})
	}
}

func TestSchemaResolver_SetCodeHostRateLimits_InvalidCodeHostID(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbmocks.NewMockDB()
	r := &schemaResolver{logger: logger, db: db}
	wantErr := errors.New("test error")

	db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
		return f(db)
	})

	usersStore := dbmocks.NewMockUserStore()
	usersStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	db.UsersFunc.SetDefaultReturn(usersStore)

	codeHostStore := dbmocks.NewMockCodeHostStore()
	codeHostStore.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int32) (*types.CodeHost, error) {
		return nil, wantErr
	})
	db.CodeHostsFunc.SetDefaultReturn(codeHostStore)

	_, err := r.SetCodeHostRateLimits(context.Background(), SetCodeHostRateLimitsArgs{
		Input: SetCodeHostRateLimitsInput{CodeHostID: ""},
	})
	require.NotNil(t, err)
	require.Equal(t, "invalid code host id: invalid graphql.ID", err.Error())
}

func TestSchemaResolver_SetCodeHostRateLimits_GetCodeHostByIDError(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbmocks.NewMockDB()
	r := &schemaResolver{logger: logger, db: db}
	wantErr := errors.New("test error")

	db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
		return f(db)
	})

	usersStore := dbmocks.NewMockUserStore()
	usersStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	db.UsersFunc.SetDefaultReturn(usersStore)

	codeHostStore := dbmocks.NewMockCodeHostStore()
	codeHostStore.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int32) (*types.CodeHost, error) {
		return nil, wantErr
	})
	db.CodeHostsFunc.SetDefaultReturn(codeHostStore)

	_, err := r.SetCodeHostRateLimits(context.Background(), SetCodeHostRateLimitsArgs{
		Input: SetCodeHostRateLimitsInput{CodeHostID: "Q29kZUhvc3Q6MQ=="},
	})
	require.NotNil(t, err)
	require.Equal(t, wantErr.Error(), err.Error())
}

func TestSchemaResolver_SetCodeHostRateLimits_UpdateCodeHostError(t *testing.T) {
	logger := logtest.Scoped(t)
	db := dbmocks.NewMockDB()
	r := &schemaResolver{logger: logger, db: db}
	wantErr := errors.New("test error")

	db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
		return f(db)
	})

	usersStore := dbmocks.NewMockUserStore()
	usersStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	db.UsersFunc.SetDefaultReturn(usersStore)

	codeHostStore := dbmocks.NewMockCodeHostStore()
	codeHostStore.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int32) (*types.CodeHost, error) {
		require.Equal(t, int32(1), id)
		return &types.CodeHost{ID: 1}, nil
	})
	codeHostStore.UpdateFunc.SetDefaultHook(func(ctx context.Context, host *types.CodeHost) error {
		return wantErr
	})
	db.CodeHostsFunc.SetDefaultReturn(codeHostStore)

	_, err := r.SetCodeHostRateLimits(context.Background(), SetCodeHostRateLimitsArgs{
		Input: SetCodeHostRateLimitsInput{CodeHostID: "Q29kZUhvc3Q6MQ=="},
	})
	require.NotNil(t, err)
	require.Equal(t, wantErr.Error(), err.Error())
}

func TestSchemaResolver_SetCodeHostRateLimits_Success(t *testing.T) {
	db := dbmocks.NewMockDB()
	ctx := context.Background()
	setCodeHostRateLimitsInput := SetCodeHostRateLimitsInput{
		CodeHostID:                      "Q29kZUhvc3Q6MQ==",
		APIQuota:                        1,
		APIReplenishmentIntervalSeconds: 2,
		GitQuota:                        3,
		GitReplenishmentIntervalSeconds: 4,
	}

	db.WithTransactFunc.SetDefaultHook(func(ctx context.Context, f func(database.DB) error) error {
		return f(db)
	})

	usersStore := dbmocks.NewMockUserStore()
	usersStore.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{SiteAdmin: true}, nil)
	db.UsersFunc.SetDefaultReturn(usersStore)

	codeHostStore := dbmocks.NewMockCodeHostStore()
	codeHostStore.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int32) (*types.CodeHost, error) {
		require.Equal(t, int32(1), id)
		return &types.CodeHost{ID: 1}, nil
	})
	codeHostStore.UpdateFunc.SetDefaultHook(func(ctx context.Context, host *types.CodeHost) error {
		require.Equal(t, setCodeHostRateLimitsInput.APIQuota, *(host.APIRateLimitQuota))
		require.Equal(t, setCodeHostRateLimitsInput.APIReplenishmentIntervalSeconds, *(host.APIRateLimitIntervalSeconds))
		require.Equal(t, setCodeHostRateLimitsInput.GitQuota, *(host.GitRateLimitQuota))
		require.Equal(t, setCodeHostRateLimitsInput.GitReplenishmentIntervalSeconds, *(host.GitRateLimitIntervalSeconds))
		return nil
	})
	db.CodeHostsFunc.SetDefaultReturn(codeHostStore)

	variables := map[string]any{
		"input": map[string]any{
			"codeHostID":                      string(setCodeHostRateLimitsInput.CodeHostID),
			"apiQuota":                        setCodeHostRateLimitsInput.APIQuota,
			"apiReplenishmentIntervalSeconds": setCodeHostRateLimitsInput.APIReplenishmentIntervalSeconds,
			"gitQuota":                        setCodeHostRateLimitsInput.GitQuota,
			"gitReplenishmentIntervalSeconds": setCodeHostRateLimitsInput.GitReplenishmentIntervalSeconds,
		},
	}
	RunTest(t, &Test{
		Context:   ctx,
		Schema:    mustParseGraphQLSchema(t, db),
		Variables: variables,
		Query: `mutation setCodeHostRateLimits($input:SetCodeHostRateLimitsInput!) {
		  setCodeHostRateLimits(input:$input) {
			alwaysNil
		  }
		}`,
		ExpectedResult: `{
			"setCodeHostRateLimits": {
			  "alwaysNil": null
			}
		}`,
	})
}
