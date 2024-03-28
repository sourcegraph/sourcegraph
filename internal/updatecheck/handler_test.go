package updatecheck

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

func TestLatestDockerVersionPushed(t *testing.T) {
	// We cannot perform external network requests in Bazel tests, it breaks the sandbox.
	if testing.Short() || os.Getenv("BAZEL_TEST") == "1" {
		t.Skip("Skipping due to network request against dockerhub")
	}

	urlStr := fmt.Sprintf("https://index.docker.io/v1/repositories/sourcegraph/server/tags/%s", latestReleaseDockerServerImageBuild.Version)
	resp, err := http.Get(urlStr)
	if err != nil {
		t.Skip("Failed to contact dockerhub", err)
	}
	if resp.StatusCode == 404 {
		t.Fatalf("sourcegraph/server:%s does not exist on dockerhub. %s", latestReleaseDockerServerImageBuild.Version, urlStr)
	}
	if resp.StatusCode != 200 {
		t.Skip("unexpected response from dockerhub", resp.StatusCode)
	}
}

func TestLatestKubernetesVersionPushed(t *testing.T) {
	// We cannot perform external network requests in Bazel tests, it breaks the sandbox.
	if testing.Short() || os.Getenv("BAZEL_TEST") == "1" {
		t.Skip("Skipping due to network request")
	}

	urlStr := fmt.Sprintf("https://github.com/sourcegraph/deploy-sourcegraph/releases/tag/v%v", latestReleaseKubernetesBuild.Version)
	resp, err := http.Head(urlStr)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Could not find Kubernetes release %s on GitHub. Response code %s from %s, err: %v", latestReleaseKubernetesBuild.Version, resp.Status, urlStr, err)
	}
}

func TestLatestDockerComposeOrPureDockerVersionPushed(t *testing.T) {
	// We cannot perform external network requests in Bazel tests, it breaks the sandbox.
	if testing.Short() || os.Getenv("BAZEL_TEST") == "1" {
		t.Skip("Skipping due to network request")
	}

	urlStr := fmt.Sprintf("https://github.com/sourcegraph/deploy-sourcegraph-docker/releases/tag/v%v", latestReleaseDockerComposeOrPureDocker.Version)
	resp, err := http.Head(urlStr)
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != 200 {
		t.Errorf("Could not find Docker Compose or Pure Docker release %s on GitHub. Response code %s from %s, err: %v", latestReleaseDockerComposeOrPureDocker.Version, resp.Status, urlStr, err)
	}
}

func TestCanUpdate(t *testing.T) {
	tests := []struct {
		name                string
		now                 time.Time
		clientVersionString string
		latestReleaseBuild  pingResponse
		hasUpdate           bool
		err                 error
	}{
		{
			name:                "no version update",
			clientVersionString: "v1.2.3",
			latestReleaseBuild:  newPingResponse("1.2.3"),
			hasUpdate:           false,
		},
		{
			name:                "version update",
			clientVersionString: "v1.2.3",
			latestReleaseBuild:  newPingResponse("1.2.4"),
			hasUpdate:           true,
		},
		{
			name:                "no date update clock skew",
			now:                 time.Date(2018, time.August, 1, 0, 0, 0, 0, time.UTC),
			clientVersionString: "19272_2018-08-02_f7dec47",
			latestReleaseBuild:  newPingResponse("1.2.3"),
			hasUpdate:           false,
		},
		{
			name:                "no date update",
			now:                 time.Date(2018, time.September, 1, 0, 0, 0, 0, time.UTC),
			clientVersionString: "19272_2018-08-01_f7dec47",
			latestReleaseBuild:  newPingResponse("1.2.3"),
			hasUpdate:           false,
		},
		{
			name:                "date update",
			now:                 time.Date(2018, time.August, 42, 0, 0, 0, 0, time.UTC),
			clientVersionString: "19272_2018-08-01_f7dec47",
			latestReleaseBuild:  newPingResponse("1.2.3"),
			hasUpdate:           true,
		},
		{
			name:                "app version update",
			clientVersionString: "2023.03.23+205275.dd37e7",
			latestReleaseBuild:  newPingResponse("2023.03.24+205301.ca3646"),
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

func makeDefaultPingRequest(t *testing.T) *pingRequest {
	t.Helper()

	return &pingRequest{
		ClientSiteID:             "0101-0101",
		LicenseKey:               "mylicense",
		ExternalURL:              "https://sourcegraph.test",
		DeployType:               "server",
		ClientVersionString:      "3.12.6",
		AuthProviders:            []string{"foo", "bar"},
		ExternalServices:         []string{extsvc.KindGitHub, extsvc.KindGitLab},
		CodeHostVersions:         nil,
		BuiltinSignupAllowed:     true,
		AccessRequestEnabled:     true,
		HasExtURL:                false,
		UniqueUsers:              123,
		Activity:                 json.RawMessage(`{"foo":"bar"}`),
		BatchChangesUsage:        nil,
		CodeIntelUsage:           nil,
		CodeMonitoringUsage:      nil,
		NotebooksUsage:           nil,
		CodeHostIntegrationUsage: nil,
		IDEExtensionsUsage:       nil,
		MigratedExtensionsUsage:  nil,
		OwnUsage:                 nil,
		SearchUsage:              nil,
		GrowthStatistics:         nil,
		SavedSearches:            nil,
		HomepagePanels:           nil,
		SearchOnboarding:         nil,
		InitialAdminEmail:        "test@sourcegraph.com",
		TotalUsers:               234,
		HasRepos:                 true,
		EverSearched:             false,
		EverFindRefs:             true,
		RetentionStatistics:      nil,
		HasCodyEnabled:           false,
		CodyUsage:                nil,
		CodyProviders:            nil,
	}
}

func makeLimitedPingRequest(t *testing.T) *pingRequest {
	return &pingRequest{
		ClientSiteID:        "0101-0101",
		DeployType:          "app",
		ClientVersionString: "2023.03.23+205275.dd37e7",
		Os:                  "mac",
		TotalRepos:          345,
		ActiveToday:         true,
	}
}

func TestSerializeBasic(t *testing.T) {
	pr := makeDefaultPingRequest(t)

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"external_url": "https://sourcegraph.test",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"homepage_panels": null,
		"llm_usage": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"access_request_enabled": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeLimited(t *testing.T) {
	pr := makeLimitedPingRequest(t)

	pingRequestBody, err := json.Marshal(pr)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	// This is the expected JSON request that will be sent over HTTP to the
	// handler. This checks that omitempty is applied to all the absent fields.
	compareJSON(t, pingRequestBody, `{
		"site": "0101-0101",
		"deployType": "app",
		"version": "2023.03.23+205275.dd37e7",
		"os": "mac",
		"totalRepos": 345,
		"activeToday": true
	}`)

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "2023.03.23+205275.dd37e7",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "",
		"external_url": "",
		"has_update": "true",
		"unique_users_today": "0",
		"site_activity": null,
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"homepage_panels": null,
		"llm_usage": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "",
		"auth_providers": "",
		"ext_services": "",
		"code_host_versions": null,
		"builtin_signup_allowed": "false",
		"access_request_enabled": "false",
		"deploy_type": "app",
		"total_user_accounts": "0",
		"has_external_url": "false",
		"has_repos": "false",
		"ever_searched": "false",
		"ever_find_refs": "false",
		"total_repos": "345",
		"active_today": "true",
		"os": "mac",
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
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "",
		"external_url": "",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"homepage_panels": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"repositories": null,
		"llm_usage": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"access_request_enabled": "false",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeBatchChangesUsage(t *testing.T) {
	pr := makeDefaultPingRequest(t)
	pr.BatchChangesUsage = json.RawMessage(`{"baz":"bonk"}`)

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"external_url": "https://sourcegraph.test",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": {"baz":"bonk"},
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"homepage_panels": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"repositories": null,
		"repository_size_histogram": null,
		"llm_usage": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"access_request_enabled": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeGrowthStatistics(t *testing.T) {
	pr := makeDefaultPingRequest(t)
	pr.GrowthStatistics = json.RawMessage(`{"baz":"bonk"}`)

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"external_url": "https://sourcegraph.test",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"llm_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"search_usage": null,
		"growth_statistics": {"baz":"bonk"},
		"has_cody_enabled": "false",
		"saved_searches": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"homepage_panels": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"access_request_enabled": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeCodeIntelUsage(t *testing.T) {
	now := time.Unix(1587396557, 0).UTC()

	testUsage, err := json.Marshal(types.NewCodeIntelUsageStatistics{
		StartOfWeek:                now,
		WAUs:                       pointers.Ptr(int32(25)),
		SearchBasedWAUs:            pointers.Ptr(int32(10)),
		PreciseCrossRepositoryWAUs: pointers.Ptr(int32(40)),
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
		NumRepositories:                                  pointers.Ptr(int32(50 + 85)),
		NumRepositoriesWithUploadRecords:                 pointers.Ptr(int32(50)),
		NumRepositoriesWithFreshUploadRecords:            pointers.Ptr(int32(40)),
		NumRepositoriesWithIndexRecords:                  pointers.Ptr(int32(30)),
		NumRepositoriesWithFreshIndexRecords:             pointers.Ptr(int32(20)),
		NumRepositoriesWithAutoIndexConfigurationRecords: pointers.Ptr(int32(7)),
		CountsByLanguage: map[string]types.CodeIntelRepositoryCountsByLanguage{
			"go": {
				NumRepositoriesWithUploadRecords:      pointers.Ptr(int32(10)),
				NumRepositoriesWithFreshUploadRecords: pointers.Ptr(int32(20)),
				NumRepositoriesWithIndexRecords:       pointers.Ptr(int32(30)),
				NumRepositoriesWithFreshIndexRecords:  pointers.Ptr(int32(40)),
			},
			"typescript": {
				NumRepositoriesWithUploadRecords:      pointers.Ptr(int32(15)),
				NumRepositoriesWithFreshUploadRecords: pointers.Ptr(int32(25)),
				NumRepositoriesWithIndexRecords:       pointers.Ptr(int32(35)),
				NumRepositoriesWithFreshIndexRecords:  pointers.Ptr(int32(45)),
			},
		},
		SettingsPageViewCount:            pointers.Ptr(int32(1489)),
		UsersWithRefPanelRedesignEnabled: pointers.Ptr(int32(46)),
		LanguageRequests: []types.LanguageRequest{
			{
				LanguageID:  "frob",
				NumRequests: 123,
			},
			{
				LanguageID:  "borf",
				NumRequests: 321,
			},
		},
		InvestigationEvents: []types.CodeIntelInvestigationEvent{
			{
				Type:  types.CodeIntelUploadErrorInvestigationType,
				WAUs:  25,
				Total: 42,
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	pr := makeDefaultPingRequest(t)

	pr.NewCodeIntelUsage = testUsage

	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"external_url": "https://sourcegraph.test",
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
			],
			"num_repositories": 135,
			"num_repositories_with_upload_records": 50,
			"num_repositories_without_upload_records": 85,
			"num_repositories_with_fresh_upload_records": 40,
			"num_repositories_with_index_records": 30,
			"num_repositories_with_fresh_index_records": 20,
			"num_repositories_with_index_configuration_records": 7,
			"counts_by_language": [
				{
					"language_id": "go",
					"num_repositories_with_upload_records": 10,
					"num_repositories_with_fresh_upload_records": 20,
					"num_repositories_with_index_records": 30,
					"num_repositories_with_fresh_index_records": 40
				},
				{
					"language_id": "typescript",
					"num_repositories_with_upload_records": 15,
					"num_repositories_with_fresh_upload_records": 25,
					"num_repositories_with_index_records": 35,
					"num_repositories_with_fresh_index_records": 45
				}
			],
			"settings_page_view_count": 1489,
			"users_with_ref_panel_redesign_enabled": 46,
			"language_requests": [
				{
					"language_id": "frob",
					"num_requests": 123
				},
				{
					"language_id": "borf",
					"num_requests": 321
				}
			],
			"investigation_events": [
				{
					"type": "CodeIntelligenceUploadErrorInvestigated",
					"waus": 25,
					"total": 42
				}
			]
		},
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"llm_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"cody_providers": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"homepage_panels": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"access_request_enabled": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeOldCodeIntelUsage(t *testing.T) {
	now := time.Unix(1587396557, 0).UTC()

	testPeriod, err := json.Marshal(&types.OldCodeIntelUsagePeriod{
		StartTime: now,
		Hover: &types.OldCodeIntelEventCategoryStatistics{
			LSIF:   &types.OldCodeIntelEventStatistics{UsersCount: 1, EventsCount: pointers.Ptr(int32(1))},
			Search: &types.OldCodeIntelEventStatistics{UsersCount: 2, EventsCount: pointers.Ptr(int32(2))},
		},
		Definitions: &types.OldCodeIntelEventCategoryStatistics{
			LSIF:   &types.OldCodeIntelEventStatistics{UsersCount: 3, EventsCount: pointers.Ptr(int32(3))},
			Search: &types.OldCodeIntelEventStatistics{UsersCount: 4, EventsCount: pointers.Ptr(int32(4))},
		},
		References: &types.OldCodeIntelEventCategoryStatistics{
			LSIF:   &types.OldCodeIntelEventStatistics{UsersCount: 5, EventsCount: pointers.Ptr(int32(1))},
			Search: &types.OldCodeIntelEventStatistics{UsersCount: 6, EventsCount: pointers.Ptr(int32(3))},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	period := string(testPeriod)

	pr := makeDefaultPingRequest(t)

	pr.CodeIntelUsage = json.RawMessage(`{"Weekly": [` + period + `]}`)

	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"external_url": "https://sourcegraph.test",
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
			],
			"num_repositories": null,
			"num_repositories_with_upload_records": null,
			"num_repositories_without_upload_records": null,
			"num_repositories_with_fresh_upload_records": null,
			"num_repositories_with_index_records": null,
			"num_repositories_with_fresh_index_records": null,
			"num_repositories_with_index_configuration_records": null,
			"counts_by_language": null,
			"settings_page_view_count": null,
			"users_with_ref_panel_redesign_enabled": null,
			"language_requests": null,
			"investigation_events": null
		},
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"migrated_extensions_usage": null,
		"llm_usage": null,
		"own_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"homepage_panels": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"access_request_enabled": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeCodeHostVersions(t *testing.T) {
	pr := makeDefaultPingRequest(t)
	pr.CodeHostVersions = json.RawMessage(`[{"external_service_kind":"GITHUB","version":"1.2.3.4"}]`)

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"external_url": "https://sourcegraph.test",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo": "bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"homepage_panels": null,
		"llm_usage": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": [{"external_service_kind":"GITHUB","version":"1.2.3.4"}],
		"builtin_signup_allowed": "true",
		"access_request_enabled": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeOwn(t *testing.T) {
	pr := &pingRequest{
		ClientSiteID:         "0101-0101",
		DeployType:           "server",
		ClientVersionString:  "3.12.6",
		AuthProviders:        []string{"foo", "bar"},
		ExternalServices:     []string{extsvc.KindGitHub, extsvc.KindGitLab},
		BuiltinSignupAllowed: true,
		HasExtURL:            false,
		UniqueUsers:          123,
		InitialAdminEmail:    "test@sourcegraph.com",
		TotalUsers:           234,
		HasRepos:             true,
		EverSearched:         false,
		EverFindRefs:         true,
		OwnUsage: json.RawMessage(`{
			"feature_flag_on": true,
			"repos_count": {
				"total": 42,
				"with_ingested_ownership": 15
			},
			"select_file_owners_search": {
				"dau": 100,
				"wau": 150,
				"mau": 300
			},
			"file_has_owner_search": {
				"dau": 100,
				"wau": 150,
				"mau": 300
			},
			"ownership_panel_opened": {
				"dau": 100,
				"wau": 150,
				"mau": 300
			},
			"assigned_owners_count": 12
		}`),
	}

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"access_request_enabled": "false",
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "",
		"external_url": "",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": null,
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"llm_usage": null,
		"migrated_extensions_usage": null,
		"own_usage": {
			"feature_flag_on": true,
			"repos_count": {
				"total": 42,
				"with_ingested_ownership": 15
			},
			"select_file_owners_search": {
				"dau": 100,
				"wau": 150,
				"mau": 300
			},
			"file_has_owner_search": {
				"dau": 100,
				"wau": 150,
				"mau": 300
			},
			"ownership_panel_opened": {
				"dau": 100,
				"wau": 150,
				"mau": 300
			},
			"assigned_owners_count": 12
		},
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"homepage_panels": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeRepoMetadataUsage(t *testing.T) {
	pr := &pingRequest{
		ClientSiteID:         "0101-0101",
		DeployType:           "server",
		ClientVersionString:  "3.12.6",
		AuthProviders:        []string{"foo", "bar"},
		ExternalServices:     []string{extsvc.KindGitHub, extsvc.KindGitLab},
		BuiltinSignupAllowed: true,
		HasExtURL:            false,
		UniqueUsers:          123,
		InitialAdminEmail:    "test@sourcegraph.com",
		TotalUsers:           234,
		HasRepos:             true,
		EverSearched:         false,
		EverFindRefs:         true,
		RepoMetadataUsage: json.RawMessage(`{
			"summary": {
				"is_enabled": true,
				"repos_with_metadata_count": 10,
				"repo_metadata_count": 100
			},
			"daily": {
				"start_time": "2020-01-01T00:00:00Z",
				"create_repo_metadata": {
					"events_count": 10,
					"users_count": 5
				}
			}
		}`),
		LlmUsage: nil,
	}

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"access_request_enabled": "false",
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "",
		"external_url": "",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": null,
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": null,
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"migrated_extensions_usage": null,
		"llm_usage": null,
		"own_usage": null,
		"repo_metadata_usage": {
			"summary": {
				"is_enabled": true,
				"repos_with_metadata_count": 10,
				"repo_metadata_count": 100
			},
			"daily": {
				"start_time": "2020-01-01T00:00:00Z",
				"create_repo_metadata": {
					"events_count": 10,
					"users_count": 5
				}
			}
		},
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"homepage_panels": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func TestSerializeCodyProviders(t *testing.T) {
	pr := makeDefaultPingRequest(t)
	pr.CodyProviders = json.RawMessage(`{"baz":"bonk"}`)

	now := time.Now()
	payload, err := marshalPing(pr, true, "127.0.0.1", now)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	compareJSON(t, payload, `{
		"remote_ip": "127.0.0.1",
		"remote_site_version": "3.12.6",
		"repo_metadata_usage": null,
		"remote_site_id": "0101-0101",
		"license_key": "mylicense",
		"external_url": "https://sourcegraph.test",
		"has_update": "true",
		"unique_users_today": "123",
		"site_activity": {"foo":"bar"},
		"batch_changes_usage": null,
		"code_intel_usage": null,
		"new_code_intel_usage": null,
		"dependency_versions": null,
		"extensions_usage": null,
		"code_insights_usage": null,
		"code_insights_critical_telemetry": null,
		"code_monitoring_usage": null,
		"cody_usage_2": null,
		"cody_providers": {"baz":"bonk"},
		"notebooks_usage": null,
		"code_host_integration_usage": null,
		"ide_extensions_usage": null,
		"llm_usage":                 null,
		"migrated_extensions_usage": null,
		"own_usage": null,
		"search_usage": null,
		"growth_statistics": null,
		"has_cody_enabled": "false",
		"saved_searches": null,
		"search_jobs_usage": null,
		"search_onboarding": null,
		"homepage_panels": null,
		"repositories": null,
		"repository_size_histogram": null,
		"retention_statistics": null,
		"installer_email": "test@sourcegraph.com",
		"auth_providers": "foo,bar",
		"ext_services": "GITHUB,GITLAB",
		"code_host_versions": null,
		"builtin_signup_allowed": "true",
		"access_request_enabled": "true",
		"deploy_type": "server",
		"total_user_accounts": "234",
		"has_external_url": "false",
		"has_repos": "true",
		"ever_searched": "false",
		"ever_find_refs": "true",
		"total_repos": "0",
		"active_today": "false",
		"os": "",
		"timestamp": "`+now.UTC().Format(time.RFC3339)+`"
	}`)
}

func compareJSON(t *testing.T, actual []byte, expected string) {
	var o1 any
	if err := json.Unmarshal(actual, &o1); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	var o2 any
	if err := json.Unmarshal([]byte(expected), &o2); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	if diff := cmp.Diff(o2, o1); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}
