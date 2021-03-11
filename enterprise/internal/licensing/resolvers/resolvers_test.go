package resolvers

import (
	"context"
	"errors"
	"testing"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/batches/resolvers/apitest"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
)

func TestEnterpriseLicenseHasFeature(t *testing.T) {
	r := &LicenseResolver{}
	schema, err := graphqlbackend.NewSchema(nil, nil, nil, nil, nil, nil, r)
	if err != nil {
		t.Fatal(err)
	}
	ctx := backend.WithAuthzBypass(context.Background())

	buildMock := func(allow ...licensing.Feature) func(feature licensing.Feature) error {
		return func(feature licensing.Feature) error {
			for _, allowed := range allow {
				if feature == allowed {
					return nil
				}
			}

			return licensing.NewFeatureNotActivatedError("feature not allowed")
		}
	}
	query := `query HasFeature($feature: String!) { enterpriseLicenseHasFeature(feature: $feature) }`

	for name, tc := range map[string]struct {
		feature string
		mock    func(feature licensing.Feature) error
		want    bool
		wantErr bool
	}{
		"real feature, enabled": {
			feature: string(licensing.FeatureBatchChanges),
			mock:    buildMock(licensing.FeatureBatchChanges),
			want:    true,
			wantErr: false,
		},
		"real feature, disabled": {
			feature: string(licensing.FeatureMonitoring),
			mock:    buildMock(licensing.FeatureBatchChanges),
			want:    false,
			wantErr: false,
		},
		"fake feature, enabled": {
			feature: "foo",
			mock:    buildMock("foo"),
			want:    true,
			wantErr: false,
		},
		"fake feature, disabled": {
			feature: "foo",
			mock:    buildMock("bar"),
			want:    false,
			wantErr: false,
		},
		"error from check": {
			feature: string(licensing.FeatureMonitoring),
			mock: func(feature licensing.Feature) error {
				return errors.New("this is a different error")
			},
			want:    false,
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			oldMock := licensing.MockCheckFeature
			licensing.MockCheckFeature = tc.mock
			defer func() {
				licensing.MockCheckFeature = oldMock
			}()

			var have struct{ EnterpriseLicenseHasFeature bool }
			if err := apitest.Exec(ctx, t, schema, map[string]interface{}{
				"feature": tc.feature,
			}, &have, query); err != nil {
				if !tc.wantErr {
					t.Errorf("got error when no error was expected: %v", err)
				}
			} else if tc.wantErr {
				t.Error("did not get expected error")
			}

			if have.EnterpriseLicenseHasFeature != tc.want {
				t.Errorf("unexpected has feature response: have=%v want=%v", have, tc.want)
			}
		})
	}
}
