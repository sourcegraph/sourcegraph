package gsm

import (
	"context"
	"fmt"
	"testing"

	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/stretchr/testify/assert"
)

type mockClient struct {
	AccessFunc func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
	CloseFunc  func() error
}

func (m *mockClient) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	return m.AccessFunc(ctx, req, opts...)
}

func (m *mockClient) Close() error {
	return m.CloseFunc()
}

func TestFetchGSM(t *testing.T) {

	testcases := []struct {
		name     string
		client   *mockClient
		project  string
		secret   string
		value    []byte
		pass     bool
		contains string
	}{
		{
			name: "Test cannot find secret returns empty secret",
			client: &mockClient{
				AccessFunc: func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
					return nil, errors.Newf("rpc error: code = NotFound desc = Secret [%s] not found or has no versions", req.Name)
				},
				CloseFunc: func() error { return nil },
			},
			project:  "foo",
			secret:   "message-signing-secret",
			value:    nil,
			pass:     false,
			contains: "projects/foo/secrets/message-signing-secret/versions/latest] not found",
		},
		{
			name: "Can find secret returns a secret",
			client: &mockClient{
				AccessFunc: func(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
					var secret secretmanagerpb.AccessSecretVersionResponse
					secret.Name = "message-signing-secret"
					secret.Payload = &secretmanagerpb.SecretPayload{}
					secret.Payload.Data = []byte("secret-value")

					return &secret, nil
				},
				CloseFunc: func() error { return nil },
			},
			project:  "foo",
			secret:   "message-signing-secret",
			value:    []byte("secret-value"),
			pass:     true,
			contains: "",
		},
	}

	ctx := context.Background()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			requestedSecrets := []SecretRequest{
				{
					Name:        "message-signing-secret",
					Description: "For signing purposes",
				},
			}

			secrets, err := NewSecretSet(ctx, tc.client, "foo", requestedSecrets)

			for secret := range secrets {
				assert.Equal(t, tc.value, secrets[secret].Value)
				if !tc.pass {
					assert.ErrorContains(t, err, tc.contains)
				}
			}
		})
	}

}
