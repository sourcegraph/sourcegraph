package iam

import (
	"testing"

	"github.com/hexops/autogold/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/dev/managedservicesplatform/spec"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestExtractImageGoogleProject(t *testing.T) {
	for _, tc := range []struct {
		name        string
		image       string
		wantProject *string
	}{
		{
			name:        "public image",
			image:       "index.docker.io/sourcegraph/telemetry-gateway",
			wantProject: nil,
		},
		{
			name:        "gcr",
			image:       "us.gcr.io/sourcegraph-dev/abuse-banbot",
			wantProject: pointers.Ptr("sourcegraph-dev"),
		},
		{
			name:        "GCP docker",
			image:       "us-central1-docker.pkg.dev/control-plane-5e9ee072/docker/apiserver",
			wantProject: pointers.Ptr("control-plane-5e9ee072"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := extractImageGoogleProject(tc.image)
			if tc.wantProject == nil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, *tc.wantProject, *got)
			}
		})
	}
}

func TestExtractExternalSecrets(t *testing.T) {
	for _, tc := range []struct {
		name                string
		secretEnv           map[string]string
		secretVolumes       map[string]spec.EnvironmentSecretVolume
		wantExternalSecrets []externalSecret
		wantError           autogold.Value
	}{
		{
			name:      "no external secrets",
			secretEnv: map[string]string{"SEKRET": "SEKRET"},
		},
		{
			name:      "invalid external secret parts",
			secretEnv: map[string]string{"SEKRET": "projects/foo/secret"},
			wantError: autogold.Expect(`invalid secret name "projects/foo/secret": expected 'projects/'-prefixed name to have 4 '/'-delimited parts`),
		},
		{
			name:      "invalid external secret prefix",
			secretEnv: map[string]string{"SEKRET": "project/foo/secrets/BAR"},
			wantError: autogold.Expect(`invalid secret name "project/foo/secrets/BAR": 'project/'-prefixed name provided, did you mean 'projects/'?`),
		},
		{
			name:      "invalid external secret segment'",
			secretEnv: map[string]string{"SEKRET": "projects/foo/secret/BAR"},
			wantError: autogold.Expect(`invalid secret name "projects/foo/secret/BAR": found '/secret/' segment, did you mean '/secrets/'?`),
		},
		{
			name:      "invalid external secret with version",
			secretEnv: map[string]string{"SEKRET": "projects/foo/secrets/BAR/versions/2"},
			wantError: autogold.Expect(`invalid secret name "projects/foo/secrets/BAR/versions/2": secrets should not be versioned with '/version/'`),
		},
		{
			name:      "has external secret",
			secretEnv: map[string]string{"SEKRET": "projects/foo/secrets/BAR", "NOT_EXTERNAL": "SEKRET"},
			wantExternalSecrets: []externalSecret{{
				key:       "sekret",
				projectID: "foo",
				secretID:  "BAR",
			}},
		},
		{
			name: "volumes has external secret",
			secretVolumes: map[string]spec.EnvironmentSecretVolume{
				"secret":  {Secret: "projects/foo/secrets/VOLUME"},
				"secret2": {Secret: "SEKRET_VOLUME"},
			},
			wantExternalSecrets: []externalSecret{{
				key:       "volume_secret",
				projectID: "foo",
				secretID:  "VOLUME",
			}},
		},
		{
			name:      "external secrets from volumes and env",
			secretEnv: map[string]string{"SEKRET": "projects/foo/secrets/BAR", "NOT_EXTERNAL": "SEKRET"},
			secretVolumes: map[string]spec.EnvironmentSecretVolume{
				"secret":  {Secret: "projects/foo/secrets/VOLUME"},
				"secret2": {Secret: "SEKRET_VOLUME"},
			},
			wantExternalSecrets: []externalSecret{{
				key:       "sekret",
				projectID: "foo",
				secretID:  "BAR",
			}, {
				key:       "volume_secret",
				projectID: "foo",
				secretID:  "VOLUME",
			}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractExternalSecrets(tc.secretEnv, tc.secretVolumes)
			if tc.wantError != nil {
				require.Error(t, err)
				tc.wantError.Equal(t, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.wantExternalSecrets, got)
		})
	}
}
