package updatecheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestLatestDockerVersionPushed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping due to network request against dockerhub")
	}

	url := fmt.Sprintf("https://index.docker.io/v1/repositories/sourcegraph/server/tags/%s", latestReleaseDockerServerImageBuild.Version)
	resp, err := http.Get(url)
	if err != nil {
		t.Skip("Failed to contact dockerhub", err)
	}
	if resp.StatusCode == 404 {
		t.Fatalf("sourcegraph/server:%s does not exist on dockerhub. %s", latestReleaseDockerServerImageBuild.Version, url)
	}
	if resp.StatusCode != 200 {
		t.Skip("unexpected response from dockerhub", resp.StatusCode)
	}
}

func TestLatestKubernetesVersionPushed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping due to network request")
	}

	url := fmt.Sprintf("https://github.com/sourcegraph/deploy-sourcegraph/releases/tag/v%v", latestReleaseKubernetesBuild.Version)
	resp, err := http.Head(url)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Could not find Kubernetes release %s on GitHub. Response code %s from %s, err: %v", latestReleaseKubernetesBuild.Version, resp.Status, url, err)
	}
}

func TestLatestDockerComposeOrPureDockerVersionPushed(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping due to network request")
	}

	url := fmt.Sprintf("https://github.com/sourcegraph/deploy-sourcegraph-docker/releases/tag/v%v", latestReleaseDockerComposeOrPureDocker.Version)
	resp, err := http.Head(url)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Could not find Docker Compose or Pure Docker release %s on GitHub. Response code %s from %s, err: %v", latestReleaseDockerComposeOrPureDocker.Version, resp.Status, url, err)
	}
}

func TestCanUpdate(t *testing.T) {
	tests := []struct {
		name                string
		now                 time.Time
		clientVersionString string
		latestReleaseBuild  build
		hasUpdate           bool
		err                 error
	}{
		{
			name:                "no version update",
			clientVersionString: "v1.2.3",
			latestReleaseBuild:  newBuild("1.2.3"),
			hasUpdate:           false,
		},
		{
			name:                "version update",
			clientVersionString: "v1.2.3",
			latestReleaseBuild:  newBuild("1.2.4"),
			hasUpdate:           true,
		},
		{
			name:                "no date update clock skew",
			now:                 time.Date(2018, time.August, 1, 0, 0, 0, 0, time.UTC),
			clientVersionString: "19272_2018-08-02_f7dec47",
			latestReleaseBuild:  newBuild("1.2.3"),
			hasUpdate:           false,
		},
		{
			name:                "no date update",
			now:                 time.Date(2018, time.September, 1, 0, 0, 0, 0, time.UTC),
			clientVersionString: "19272_2018-08-01_f7dec47",
			latestReleaseBuild:  newBuild("1.2.3"),
			hasUpdate:           false,
		},
		{
			name:                "date update",
			now:                 time.Date(2018, time.August, 42, 0, 0, 0, 0, time.UTC),
			clientVersionString: "19272_2018-08-01_f7dec47",
			latestReleaseBuild:  newBuild("1.2.3"),
			hasUpdate:           true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Mock the current time for this test.
			timeNow = func() time.Time {
				return test.now
			}
			// Restore the real time after this test is done.
			defer func() {
				timeNow = time.Now
			}()

			hasUpdate, err := canUpdate(test.clientVersionString, test.latestReleaseBuild)
			if err != test.err {
				t.Fatalf("expected error %s; got %s", test.err, err)
			}
			if hasUpdate != test.hasUpdate {
				t.Fatalf("expected hasUpdate=%t; got hasUpdate=%t", test.hasUpdate, hasUpdate)
			}
		})
	}
}

func TestSerializeBasic(t *testing.T) {
	pr := &pingRequest{
		ClientSiteID:         "0101-0101",
		LicenseKey:           "mylicense",
		DeployType:           "server",
		ClientVersionString:  "3.12.6",
		AuthProviders:        []string{"foo", "bar"},
		ExternalServices:     []string{extsvc.KindGitHub, extsvc.KindGitLab},
		BuiltinSignupAllowed: true,
		HasExtURL:            false,
		UniqueUsers:          123,
		Activity:             json.RawMessage([]byte(`{"foo":"bar"}`)),
		BatchChangesUsage:    nil,
		CodeIntelUsage:       nil,
		SearchUsage:          nil,
		GrowthStatistics:     nil,
		SavedSearches:        nil,
		HomepagePanels:       nil,
		SearchOnboarding:     nil,
		InitialAdminEmail:    "test@sourcegraph.com",
		TotalUsers:           234,
		HasRepos:             true,
		EverSearched:         false,
		EverFindRefs:         true,
		RetentionStatistics:  nil,
	}

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"saved_searches": null,
		"search_onboarding": null,
		"homepage_panels": null,
		"repositories": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"builtin_signup_allowed": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeFromQuery(t *testing.T) {
	pr, err := readPingRequestFromQuery(url.Values{
		"site":       []string{"0101-0101"},
		"deployType": []string{"server"},
		"version":    []string{"3.12.6"},
		"auth":       []string{"foo,bar"},
		"extsvcs":    []string{"GITHUB,GITLAB"},
		"signup":     []string{"true"},
		"hasExtURL":  []string{"false"},
		"u":          []string{"123"},
		"act":        []string{`{"foo": "bar"}`},
		"initAdmin":  []string{"test@sourcegraph.com"},
		"totalUsers": []string{"234"},
		"repos":      []string{"true"},
		"searched":   []string{"false"},
		"refs":       []string{"true"},
	})
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"remote_site_id": "0101-0101",
		"license_key": "",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"saved_searches": null,
		"homepage_panels": null,
		"search_onboarding": null,
		"repositories": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"builtin_signup_allowed": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeAutomationUsage(t *testing.T) {
	pr := &pingRequest{
		ClientSiteID:         "0101-0101",
		DeployType:           "server",
		ClientVersionString:  "3.12.6",
		AuthProviders:        []string{"foo", "bar"},
		ExternalServices:     []string{extsvc.KindGitHub, extsvc.KindGitLab},
		BuiltinSignupAllowed: true,
		HasExtURL:            false,
		UniqueUsers:          123,
		Activity:             json.RawMessage([]byte(`{"foo":"bar"}`)),
		BatchChangesUsage:    json.RawMessage([]byte(`{"baz":"bonk"}`)),
		CodeIntelUsage:       nil,
		NewCodeIntelUsage:    nil,
		SearchUsage:          nil,
		GrowthStatistics:     nil,
		SavedSearches:        nil,
		HomepagePanels:       nil,
		SearchOnboarding:     nil,
		InitialAdminEmail:    "test@sourcegraph.com",
		TotalUsers:           234,
		HasRepos:             true,
		EverSearched:         false,
		EverFindRefs:         true,
		RetentionStatistics:  nil,
	}

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"remote_site_id": "0101-0101",
		"license_key": "",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": {"baz":"bonk"},
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"saved_searches": null,
		"homepage_panels": null,
		"search_onboarding": null,
		"repositories": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"builtin_signup_allowed": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeCodeIntelUsage(t *testing.T) {
	now := time.Unix(1587396557, 0).UTC()
	waus1 := int32(25)
	waus2 := int32(10)
	waus3 := int32(40)

	testUsage, err := json.Marshal(types.NewCodeIntelUsageStatistics{
		StartOfWeek:                now,
		WAUs:                       &waus1,
		SearchBasedWAUs:            &waus2,
		PreciseCrossRepositoryWAUs: &waus3,
		EventSummaries: []types.CodeIntelEventSummary{
			{
				Action:          types.HoverAction,
				Source:          types.PreciseSource,
				LanguageID:      "go",
				CrossRepository: false,
				WAUs:            1,
				TotalActions:    1,
			},
			{
				Action:          types.HoverAction,
				Source:          types.SearchSource,
				LanguageID:      "",
				CrossRepository: true,
				WAUs:            2,
				TotalActions:    2,
			},
			{
				Action:          types.DefinitionsAction,
				Source:          types.PreciseSource,
				LanguageID:      "go",
				CrossRepository: true,
				WAUs:            3,
				TotalActions:    3,
			},
			{
				Action:          types.DefinitionsAction,
				Source:          types.SearchSource,
				LanguageID:      "go",
				CrossRepository: false,
				WAUs:            4,
				TotalActions:    4,
			},
			{
				Action:          types.ReferencesAction,
				Source:          types.PreciseSource,
				LanguageID:      "",
				CrossRepository: false,
				WAUs:            5,
				TotalActions:    1,
			},
			{
				Action:          types.ReferencesAction,
				Source:          types.SearchSource,
				LanguageID:      "typescript",
				CrossRepository: false,
				WAUs:            6,
				TotalActions:    3,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	pr := &pingRequest{
		ClientSiteID:         "0101-0101",
		DeployType:           "server",
		ClientVersionString:  "3.12.6",
		AuthProviders:        []string{"foo", "bar"},
		ExternalServices:     []string{extsvc.KindGitHub, extsvc.KindGitLab},
		BuiltinSignupAllowed: true,
		HasExtURL:            false,
		UniqueUsers:          123,
		Activity:             json.RawMessage([]byte(`{"foo":"bar"}`)),
		BatchChangesUsage:    nil,
		CodeIntelUsage:       nil,
		NewCodeIntelUsage:    testUsage,
		SearchUsage:          nil,
		GrowthStatistics:     nil,
		SavedSearches:        nil,
		HomepagePanels:       nil,
		SearchOnboarding:     nil,
		InitialAdminEmail:    "test@sourcegraph.com",
		TotalUsers:           234,
		HasRepos:             true,
		EverSearched:         false,
		EverFindRefs:         true,
		RetentionStatistics:  nil,
	}

	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"remote_site_id": "0101-0101",
		"license_key": "",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": {
			"start_time": "2020-04-20T15:29:17Z",
			"waus": 25,
			"precise_waus": null,
			"search_waus": 10,
			"xrepo_waus": null,
			"precise_xrepo_waus": 40,
			"search_xrepo_waus": null,
			"event_summaries": [
				{
					"action": "hover",
					"source": "precise",
					"language_id": "go",
					"cross_repository": false,
					"waus": 1,
					"total_actions": 1
				},
				{
					"action": "hover",
					"source": "search",
					"language_id": "",
					"cross_repository": true,
					"waus": 2,
					"total_actions": 2
				},
				{
					"action": "definitions",
					"source": "precise",
					"language_id": "go",
					"cross_repository": true,
					"waus": 3,
					"total_actions": 3
				},
				{
					"action": "definitions",
					"source": "search",
					"language_id": "go",
					"cross_repository": false,
					"waus": 4,
					"total_actions": 4
				},
				{
					"action": "references",
					"source": "precise",
					"language_id": "",
					"cross_repository": false,
					"waus": 5,
					"total_actions": 1
				},
				{
					"action": "references",
					"source": "search",
					"language_id": "typescript",
					"cross_repository": false,
					"waus": 6,
					"total_actions": 3
				}
			]
		},
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"saved_searches": null,
		"homepage_panels": null,
		"search_onboarding": null,
		"repositories": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"builtin_signup_allowed": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeOldCodeIntelUsage(t *testing.T) {
	v1 := int32(1)
	v2 := int32(2)
	v3 := int32(3)
	v4 := int32(4)
	now := time.Unix(1587396557, 0).UTC()

	testPeriod, err := json.Marshal(&types.OldCodeIntelUsagePeriod{
		StartTime: now,
		Hover: &types.OldCodeIntelEventCategoryStatistics{
			LSIF:   &types.OldCodeIntelEventStatistics{UsersCount: 1, EventsCount: &v1},
			Search: &types.OldCodeIntelEventStatistics{UsersCount: 2, EventsCount: &v2},
		},
		Definitions: &types.OldCodeIntelEventCategoryStatistics{
			LSIF:   &types.OldCodeIntelEventStatistics{UsersCount: 3, EventsCount: &v3},
			Search: &types.OldCodeIntelEventStatistics{UsersCount: 4, EventsCount: &v4},
		},
		References: &types.OldCodeIntelEventCategoryStatistics{
			LSIF:   &types.OldCodeIntelEventStatistics{UsersCount: 5, EventsCount: &v1},
			Search: &types.OldCodeIntelEventStatistics{UsersCount: 6, EventsCount: &v3},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	period := string(testPeriod)

	pr := &pingRequest{
		ClientSiteID:         "0101-0101",
		DeployType:           "server",
		ClientVersionString:  "3.12.6",
		AuthProviders:        []string{"foo", "bar"},
		ExternalServices:     []string{extsvc.KindGitHub, extsvc.KindGitLab},
		BuiltinSignupAllowed: true,
		HasExtURL:            false,
		UniqueUsers:          123,
		Activity:             json.RawMessage([]byte(`{"foo":"bar"}`)),
		BatchChangesUsage:    nil,
		CodeIntelUsage:       json.RawMessage([]byte(`{"Weekly": [` + period + `]}`)),
		NewCodeIntelUsage:    nil,
		SearchUsage:          nil,
		GrowthStatistics:     nil,
		SavedSearches:        nil,
		HomepagePanels:       nil,
		SearchOnboarding:     nil,
		InitialAdminEmail:    "test@sourcegraph.com",
		TotalUsers:           234,
		HasRepos:             true,
		EverSearched:         false,
		EverFindRefs:         true,
		RetentionStatistics:  nil,
	}

	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"remote_site_id": "0101-0101",
		"license_key": "",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": {
			"start_time": "2020-04-20T15:29:17Z",
			"waus": null,
			"precise_waus": null,
			"search_waus": null,
			"xrepo_waus": null,
			"precise_xrepo_waus": null,
			"search_xrepo_waus": null,
			"event_summaries": [
				{
					"action": "hover",
					"source": "precise",
					"language_id": "",
					"cross_repository": false,
					"waus": 1,
					"total_actions": 1
				},
				{
					"action": "hover",
					"source": "search",
					"language_id": "",
					"cross_repository": false,
					"waus": 2,
					"total_actions": 2
				},
				{
					"action": "definitions",
					"source": "precise",
					"language_id": "",
					"cross_repository": false,
					"waus": 3,
					"total_actions": 3
				},
				{
					"action": "definitions",
					"source": "search",
					"language_id": "",
					"cross_repository": false,
					"waus": 4,
					"total_actions": 4
				},
				{
					"action": "references",
					"source": "precise",
					"language_id": "",
					"cross_repository": false,
					"waus": 5,
					"total_actions": 1
				},
				{
					"action": "references",
					"source": "search",
					"language_id": "",
					"cross_repository": false,
					"waus": 6,
					"total_actions": 3
				}
			]
		},
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"saved_searches": null,
		"homepage_panels": null,
		"search_onboarding": null,
		"repositories": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"builtin_signup_allowed": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func compareJSON(t *testing.T, actual []byte, expected string) {
	var o1 interface{}
	if err := json.Unmarshal(actual, &o1); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	var o2 interface{}
	if err := json.Unmarshal([]byte(expected), &o2); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	if diff := cmp.Diff(o2, o1); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}
