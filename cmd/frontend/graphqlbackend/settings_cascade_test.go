package graphqlbackend

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestMergeSettings(t *testing.T) {
	boolPtr := func(b bool) *bool {
		return &b
	}

	cases := []struct {
		name     string
		left     *schema.Settings
		right    *schema.Settings
		expected *schema.Settings
	}{{
		name:     "nil left",
		left:     nil,
		right:    &schema.Settings{},
		expected: &schema.Settings{},
	}, {
		name: "empty left",
		left: &schema.Settings{},
		right: &schema.Settings{
			AlertsCodeHostIntegrationMessaging: "test",
		},
		expected: &schema.Settings{
			AlertsCodeHostIntegrationMessaging: "test",
		},
	}, {
		name: "merge bool ptr",
		left: &schema.Settings{
			AlertsHideObservabilitySiteAlerts: boolPtr(true),
		},
		right: &schema.Settings{
			AlertsCodeHostIntegrationMessaging: "test",
		},
		expected: &schema.Settings{
			AlertsCodeHostIntegrationMessaging: "test",
			AlertsHideObservabilitySiteAlerts:  boolPtr(true),
		},
	}, {
		name: "merge bool",
		left: &schema.Settings{
			AlertsShowPatchUpdates:              false,
			BasicCodeIntelGlobalSearchesEnabled: true,
		},
		right: &schema.Settings{
			AlertsShowPatchUpdates:              true,
			BasicCodeIntelGlobalSearchesEnabled: false, // This is the zero value, so will not override a previous non-zero value
		},
		expected: &schema.Settings{
			AlertsShowPatchUpdates:              true,
			BasicCodeIntelGlobalSearchesEnabled: true,
		},
	}, {
		name: "merge int",
		left: &schema.Settings{
			SearchContextLines:                        0,
			CodeIntelligenceAutoIndexPopularRepoLimit: 1,
		},
		right: &schema.Settings{
			SearchContextLines:                        1,
			CodeIntelligenceAutoIndexPopularRepoLimit: 0, // This is the zero value, so will not override a previous non-zero value
		},
		expected: &schema.Settings{
			SearchContextLines:                        1,
			CodeIntelligenceAutoIndexPopularRepoLimit: 1, // This is the zero value, so will not override a previous non-zero value
		},
	}, {
		name: "deep merge struct pointer",
		left: &schema.Settings{
			ExperimentalFeatures: &schema.SettingsExperimentalFeatures{
				CodeMonitoringWebHooks: boolPtr(true),
			},
		},
		right: &schema.Settings{
			ExperimentalFeatures: &schema.SettingsExperimentalFeatures{
				ShowMultilineSearchConsole: boolPtr(false),
			},
		},
		expected: &schema.Settings{
			ExperimentalFeatures: &schema.SettingsExperimentalFeatures{
				CodeMonitoringWebHooks:     boolPtr(true),
				ShowMultilineSearchConsole: boolPtr(false),
			},
		},
	}, {
		name: "overwriting merge",
		left: &schema.Settings{
			AlertsHideObservabilitySiteAlerts: boolPtr(true),
		},
		right: &schema.Settings{
			AlertsHideObservabilitySiteAlerts: boolPtr(false),
		},
		expected: &schema.Settings{
			AlertsHideObservabilitySiteAlerts: boolPtr(false),
		},
	}, {
		name: "deep merge slice",
		left: &schema.Settings{
			SearchScopes: []*schema.SearchScope{{Name: "test1"}},
		},
		right: &schema.Settings{
			SearchScopes: []*schema.SearchScope{{Name: "test2"}},
		},
		expected: &schema.Settings{
			SearchScopes: []*schema.SearchScope{{Name: "test1"}, {Name: "test2"}},
		},
	},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := mergeSettingsLeft(tc.left, tc.right)
			require.Equal(t, tc.expected, res)
		})
	}
}

func TestSubjects(t *testing.T) {
	t.Run("Default settings are included", func(t *testing.T) {
		cascade := &settingsCascade{db: database.NewMockDB(), unauthenticatedActor: true}
		subjects, err := cascade.Subjects(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(subjects) < 1 {
			t.Fatal("Expected at least 1 subject")
		}
		if subjects[0].defaultSettings == nil {
			t.Fatal("Expected the first subject to be default settings")
		}
	})
}
