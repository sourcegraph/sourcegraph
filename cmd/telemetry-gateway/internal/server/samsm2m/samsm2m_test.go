package samsm2m

import (
	"context"
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/sourcegraph/sourcegraph/internal/sams"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type mockSAMSClient struct {
	result *sams.TokenIntrospection
	error  error
}

func (m mockSAMSClient) IntrospectToken(context.Context, string) (*sams.TokenIntrospection, error) {
	return m.result, m.error
}

func TestCheckWriteEventsScope(t *testing.T) {
	for _, tc := range []struct {
		name       string
		metadata   map[string]string
		samsClient sams.Client
		wantErr    autogold.Value
	}{
		{
			name:       "no metadata",
			metadata:   nil,
			samsClient: nil, // will not be used
			wantErr:    autogold.Expect("rpc error: code = Unauthenticated desc = no token header"),
		},
		{
			name:       "no authorization header",
			metadata:   map[string]string{"somethingelse": "foobar"},
			samsClient: nil, // will not be used
			wantErr:    autogold.Expect("rpc error: code = Unauthenticated desc = no token header value"),
		},
		{
			name:       "malformed authorization header",
			metadata:   map[string]string{"authorization": "bearer"},
			samsClient: nil, // will not be used
			wantErr:    autogold.Expect("rpc error: code = Unauthenticated desc = invalid token header: token type missing in Authorization header"),
		},
		{
			name:       "token ok, introspect failed",
			metadata:   map[string]string{"authorization": "bearer foobar"},
			samsClient: mockSAMSClient{error: errors.New("introspection failed")},
			wantErr:    autogold.Expect("rpc error: code = Internal desc = unable to validate token"),
		},
		{
			name:       "token ok, but inactive",
			metadata:   map[string]string{"authorization": "bearer foobar"},
			samsClient: mockSAMSClient{result: &sams.TokenIntrospection{Active: false}},
			wantErr:    autogold.Expect("rpc error: code = PermissionDenied desc = permission denied"),
		},
		{
			name:       "token ok and active, but invalid scope",
			metadata:   map[string]string{"authorization": "bearer foobar"},
			samsClient: mockSAMSClient{result: &sams.TokenIntrospection{Active: true, Scope: "foo bar"}},
			wantErr:    autogold.Expect("rpc error: code = PermissionDenied desc = permission denied"),
		},
		{
			name:     "token ok and active and valid scope",
			metadata: map[string]string{"authorization": "bearer foobar"},
			samsClient: mockSAMSClient{
				result: &sams.TokenIntrospection{
					Active: true,
					Scope:  "foo bar " + requiredSamsScope,
				},
			},
			wantErr: nil, // success
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if len(tc.metadata) > 0 {
				// we mock the ctx of an incoming context
				ctx = metadata.NewIncomingContext(ctx, metadata.New(tc.metadata))
			}

			err := CheckWriteEventsScope(ctx, logtest.Scoped(t), tc.samsClient)
			if tc.wantErr == nil {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				tc.wantErr.Equal(t, err.Error())
			}
		})
	}
}
