package updatecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"go.opentelemetry.io/otel/metric"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot/hubspotutil"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/pubsub"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var (
	// latestReleaseDockerServerImageBuild is only used by sourcegraph.com to tell existing
	// non-cluster, non-docker-compose, and non-pure-docker installations what the latest
	// version is. The version here _must_ be available at https://hub.docker.com/r/sourcegraph/server/tags/
	// before landing in master.
	latestReleaseDockerServerImageBuild = newPingResponse("5.3.2")

	// latestReleaseKubernetesBuild is only used by sourcegraph.com to tell existing Sourcegraph
	// cluster deployments what the latest version is. The version here _must_ be available in
	// a tag at https://github.com/sourcegraph/deploy-sourcegraph before landing in master.
	latestReleaseKubernetesBuild = newPingResponse("5.3.2")

	// latestReleaseDockerComposeOrPureDocker is only used by sourcegraph.com to tell existing Sourcegraph
	// Docker Compose or Pure Docker deployments what the latest version is. The version here _must_ be
	// available in a tag at https://github.com/sourcegraph/deploy-sourcegraph-docker before landing in master.
	latestReleaseDockerComposeOrPureDocker = newPingResponse("5.3.2")
)

func getLatestRelease(deployType string) pingResponse {
	switch {
	case deploy.IsDeployTypeKubernetes(deployType):
		return latestReleaseKubernetesBuild
	case deploy.IsDeployTypeDockerCompose(deployType), deploy.IsDeployTypePureDocker(deployType):
		return latestReleaseDockerComposeOrPureDocker
	default:
		return latestReleaseDockerServerImageBuild
	}
}

// ForwardHandler returns a handler that forwards the request to
// https://pings.sourcegraph.com.
func ForwardHandler() (http.HandlerFunc, error) {
	remote, err := url.Parse(defaultUpdateCheckURL)
	if err != nil {
		return nil, errors.Errorf("parse default update check URL: %v", err)
	}

	// If remote has a path, the proxy server will always append an unnecessary "/" to the path.
	remotePath := remote.Path
	remote.Path = ""
	proxy := httputil.NewSingleHostReverseProxy(remote)
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = remote.Host
		r.URL.Path = remotePath
		proxy.ServeHTTP(w, r)
	}, nil
}

type Meter struct {
	RequestCounter          metric.Int64Counter
	RequestHasUpdateCounter metric.Int64Counter
	ErrorCounter            metric.Int64Counter
}

// Handle handles the ping requests and responds with information about software
// updates for Sourcegraph.
func Handle(logger log.Logger, pubsubClient pubsub.TopicPublisher, meter *Meter, w http.ResponseWriter, r *http.Request) {
	meter.RequestCounter.Add(r.Context(), 1)

	pr, err := readPingRequest(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if pr.ClientSiteID == "" {
		logger.Error("no site ID specified")
		http.Error(w, "no site ID specified", http.StatusBadRequest)
		return
	}
	if pr.ClientVersionString == "" {
		logger.Error("no version specified")
		http.Error(w, "no version specified", http.StatusBadRequest)
		return
	}
	if pr.ClientVersionString == "dev" {
		// No updates for dev servers.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	pingResponse := getLatestRelease(pr.DeployType)
	hasUpdate, err := canUpdate(pr.ClientVersionString, pingResponse)

	// Always log, even on malformed version strings
	logPing(logger, pubsubClient, meter, r, pr, hasUpdate)

	if err != nil {
		http.Error(w, pr.ClientVersionString+" is a bad version string: "+err.Error(), http.StatusBadRequest)
		return
	}
	body, err := json.Marshal(pingResponse)
	if err != nil {
		logger.Error("error preparing update check response", log.Error(err))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if !hasUpdate {
		// No newer version.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("content-type", "application/json; charset=utf-8")
	meter.RequestHasUpdateCounter.Add(r.Context(), 1)
	_, _ = w.Write(body)
}

// canUpdate returns true if the latestReleaseBuild is newer than the clientVersionString.
func canUpdate(clientVersionString string, latestReleaseBuild pingResponse) (bool, error) {
	// Check for a date in the version string to handle developer builds that don't have a semver.
	// If there is an error parsing a date out of the version string, then we ignore the error
	// and parse it as a semver.
	if hasDateUpdate, err := canUpdateDate(clientVersionString); err == nil {
		return hasDateUpdate, nil
	}

	// Released builds will have a semantic version that we can compare.
	return canUpdateVersion(clientVersionString, latestReleaseBuild)
}

// canUpdateVersion returns true if the latest released build is newer than
// the clientVersionString. It returns an error if clientVersionString is not a semver.
func canUpdateVersion(clientVersionString string, latestReleaseBuild pingResponse) (bool, error) {
	clientVersionString = strings.TrimPrefix(clientVersionString, "v")
	clientVersion, err := semver.NewVersion(clientVersionString)
	if err != nil {
		return false, err
	}
	return clientVersion.LessThan(latestReleaseBuild.Version), nil
}

var (
	dateRegex = lazyregexp.New("_([0-9]{4}-[0-9]{2}-[0-9]{2})_")
	timeNow   = time.Now
)

// canUpdateDate returns true if clientVersionString contains a date
// more than 40 days in the past. It returns an error if there is no
// parsable date in clientVersionString
func canUpdateDate(clientVersionString string) (bool, error) {
	match := dateRegex.FindStringSubmatch(clientVersionString)
	if len(match) != 2 {
		return false, errors.Errorf("no date in version string %q", clientVersionString)
	}

	t, err := time.ParseInLocation("2006-01-02", match[1], time.UTC)
	if err != nil {
		// This shouldn't ever happen if the above code is correct.
		return false, err
	}

	// Assume that we release a new version at least every 40 days.
	return timeNow().After(t.Add(40 * 24 * time.Hour)), nil
}

// pingRequest is the payload of the update check request. These values either
// supplied via query string or by a JSON body (when the request method is POST).
// We need to maintain backwards compatibility with the GET-only update checks
// while expanding the payload size for newer instance versions (via HTTP body).
type pingRequest struct {
	ClientSiteID         string          `json:"site"`
	LicenseKey           string          `json:",omitempty"`
	ExternalURL          string          `json:"externalURL,omitempty"`
	DeployType           string          `json:"deployType"`
	Os                   string          `json:"os,omitempty"` // Only used in Cody App
	ClientVersionString  string          `json:"version"`
	DependencyVersions   json.RawMessage `json:"dependencyVersions,omitempty"`
	AuthProviders        []string        `json:"auth,omitempty"`
	ExternalServices     []string        `json:"extsvcs,omitempty"`
	BuiltinSignupAllowed bool            `json:"signup,omitempty"`
	AccessRequestEnabled bool            `json:"accessRequestEnabled,omitempty"`
	HasExtURL            bool            `json:"hasExtURL,omitempty"`
	UniqueUsers          int32           `json:"u,omitempty"`
	Activity             json.RawMessage `json:"act,omitempty"`
	BatchChangesUsage    json.RawMessage `json:"batchChangesUsage,omitempty"`
	// AutomationUsage (campaigns) is deprecated, but here so we can receive pings from older instances
	AutomationUsage               json.RawMessage `json:"automationUsage,omitempty"`
	GrowthStatistics              json.RawMessage `json:"growthStatistics,omitempty"`
	SavedSearches                 json.RawMessage `json:"savedSearches,omitempty"`
	HomepagePanels                json.RawMessage `json:"homepagePanels,omitempty"`
	SearchOnboarding              json.RawMessage `json:"searchOnboarding,omitempty"`
	Repositories                  json.RawMessage `json:"repositories,omitempty"`
	RepositorySizeHistogram       json.RawMessage `json:"repository_size_histogram,omitempty"`
	RetentionStatistics           json.RawMessage `json:"retentionStatistics,omitempty"`
	CodeIntelUsage                json.RawMessage `json:"codeIntelUsage,omitempty"`
	NewCodeIntelUsage             json.RawMessage `json:"newCodeIntelUsage,omitempty"`
	SearchUsage                   json.RawMessage `json:"searchUsage,omitempty"`
	ExtensionsUsage               json.RawMessage `json:"extensionsUsage,omitempty"`
	CodeInsightsUsage             json.RawMessage `json:"codeInsightsUsage,omitempty"`
	SearchJobsUsage               json.RawMessage `json:"searchJobsUsage,omitempty"`
	CodeInsightsCriticalTelemetry json.RawMessage `json:"codeInsightsCriticalTelemetry,omitempty"`
	CodeMonitoringUsage           json.RawMessage `json:"codeMonitoringUsage,omitempty"`
	NotebooksUsage                json.RawMessage `json:"notebooksUsage,omitempty"`
	CodeHostVersions              json.RawMessage `json:"codeHostVersions,omitempty"`
	CodeHostIntegrationUsage      json.RawMessage `json:"codeHostIntegrationUsage,omitempty"`
	IDEExtensionsUsage            json.RawMessage `json:"ideExtensionsUsage,omitempty"`
	MigratedExtensionsUsage       json.RawMessage `json:"migratedExtensionsUsage,omitempty"`
	OwnUsage                      json.RawMessage `json:"ownUsage,omitempty"`
	InitialAdminEmail             string          `json:"initAdmin,omitempty"`
	TosAccepted                   bool            `json:"tosAccepted,omitempty"`
	TotalUsers                    int32           `json:"totalUsers,omitempty"`
	TotalOrgs                     int32           `json:"totalOrgs,omitempty"`
	TotalRepos                    int32           `json:"totalRepos,omitempty"` // Only used in Cody App
	HasRepos                      bool            `json:"repos,omitempty"`
	EverSearched                  bool            `json:"searched,omitempty"`
	EverFindRefs                  bool            `json:"refs,omitempty"`
	ActiveToday                   bool            `json:"activeToday,omitempty"` // Only used in Cody App
	HasCodyEnabled                bool            `json:"hasCodyEnabled,omitempty"`
	// CodyUsage is deprecated, but here so we can receive pings from older instances
	CodyUsage         json.RawMessage `json:"codyUsage,omitempty"`
	CodyUsage2        json.RawMessage `json:"codyUsage2,omitempty"`
	CodyProviders     json.RawMessage `json:"codyProviders,omitempty"`
	RepoMetadataUsage json.RawMessage `json:"repoMetadataUsage,omitempty"`
}

type dependencyVersions struct {
	PostgresVersion   string `json:"postgresVersion"`
	RedisCacheVersion string `json:"redisCacheVersion"`
	RedisStoreVersion string `json:"redisStoreVersion"`
}

// readPingRequest reads the ping request payload from the request. If the
// request method is GET, it will read all parameters from the query string.
// If the request method is POST, it will read the parameters via a JSON
// encoded HTTP body.
func readPingRequest(r *http.Request) (*pingRequest, error) {
	if r.Method == "GET" {
		return readPingRequestFromQuery(r.URL.Query())
	}

	return readPingRequestFromBody(r.Body)
}

func readPingRequestFromQuery(q url.Values) (*pingRequest, error) {
	return &pingRequest{
		ClientSiteID: q.Get("site"),
		// LicenseKey was added after the switch from query strings to POST data, so it's not
		// available.
		DeployType:           q.Get("deployType"),
		ClientVersionString:  q.Get("version"),
		AuthProviders:        strings.Split(q.Get("auth"), ","),
		ExternalServices:     strings.Split(q.Get("extsvcs"), ","),
		BuiltinSignupAllowed: toBool(q.Get("signup")),
		AccessRequestEnabled: toBool(q.Get("accessRequestEnabled")),
		HasExtURL:            toBool(q.Get("hasExtURL")),
		UniqueUsers:          toInt(q.Get("u")),
		Activity:             toRawMessage(q.Get("act")),
		InitialAdminEmail:    q.Get("initAdmin"),
		TotalUsers:           toInt(q.Get("totalUsers")),
		HasRepos:             toBool(q.Get("repos")),
		EverSearched:         toBool(q.Get("searched")),
		EverFindRefs:         toBool(q.Get("refs")),
		TosAccepted:          toBool(q.Get("tosAccepted")),
	}, nil
}

func readPingRequestFromBody(body io.ReadCloser) (*pingRequest, error) {
	defer func() { _ = body.Close() }()
	contents, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var payload *pingRequest
	if err := json.Unmarshal(contents, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func toInt(val string) int32 {
	value, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0
	}
	return int32(value)
}

func toBool(val string) bool {
	value, err := strconv.ParseBool(val)
	return err == nil && value
}

func toRawMessage(val string) json.RawMessage {
	if val == "" {
		return nil
	}
	var payload json.RawMessage
	_ = json.Unmarshal([]byte(val), &payload)
	return payload
}

type pingPayload struct {
	RemoteIP                      string          `json:"remote_ip"`
	RemoteSiteVersion             string          `json:"remote_site_version"`
	RemoteSiteID                  string          `json:"remote_site_id"`
	LicenseKey                    string          `json:"license_key"`
	ExternalURL                   string          `json:"external_url"`
	HasUpdate                     string          `json:"has_update"`
	UniqueUsersToday              string          `json:"unique_users_today"`
	SiteActivity                  json.RawMessage `json:"site_activity"`
	BatchChangesUsage             json.RawMessage `json:"batch_changes_usage"`
	CodeIntelUsage                json.RawMessage `json:"code_intel_usage"`
	NewCodeIntelUsage             json.RawMessage `json:"new_code_intel_usage"`
	SearchUsage                   json.RawMessage `json:"search_usage"`
	GrowthStatistics              json.RawMessage `json:"growth_statistics"`
	SavedSearches                 json.RawMessage `json:"saved_searches"`
	HomepagePanels                json.RawMessage `json:"homepage_panels"`
	RetentionStatistics           json.RawMessage `json:"retention_statistics"`
	Repositories                  json.RawMessage `json:"repositories"`
	RepositorySizeHistogram       json.RawMessage `json:"repository_size_histogram"`
	SearchOnboarding              json.RawMessage `json:"search_onboarding"`
	DependencyVersions            json.RawMessage `json:"dependency_versions"`
	ExtensionsUsage               json.RawMessage `json:"extensions_usage"`
	CodeInsightsUsage             json.RawMessage `json:"code_insights_usage"`
	SearchJobsUsage               json.RawMessage `json:"search_jobs_usage"`
	CodeInsightsCriticalTelemetry json.RawMessage `json:"code_insights_critical_telemetry"`
	CodeMonitoringUsage           json.RawMessage `json:"code_monitoring_usage"`
	NotebooksUsage                json.RawMessage `json:"notebooks_usage"`
	CodeHostVersions              json.RawMessage `json:"code_host_versions"`
	CodeHostIntegrationUsage      json.RawMessage `json:"code_host_integration_usage"`
	IDEExtensionsUsage            json.RawMessage `json:"ide_extensions_usage"`
	MigratedExtensionsUsage       json.RawMessage `json:"migrated_extensions_usage"`
	OwnUsage                      json.RawMessage `json:"own_usage"`
	InstallerEmail                string          `json:"installer_email"`
	AuthProviders                 string          `json:"auth_providers"`
	ExtServices                   string          `json:"ext_services"`
	BuiltinSignupAllowed          string          `json:"builtin_signup_allowed"`
	AccessRequestEnabled          string          `json:"access_request_enabled"`
	DeployType                    string          `json:"deploy_type"`
	TotalUserAccounts             string          `json:"total_user_accounts"`
	TotalRepos                    string          `json:"total_repos"`
	HasExternalURL                string          `json:"has_external_url"`
	HasRepos                      string          `json:"has_repos"`
	EverSearched                  string          `json:"ever_searched"`
	EverFindRefs                  string          `json:"ever_find_refs"`
	Os                            string          `json:"os"`
	ActiveToday                   string          `json:"active_today"`
	Timestamp                     string          `json:"timestamp"`
	HasCodyEnabled                string          `json:"has_cody_enabled"`
	CodyUsage2                    json.RawMessage `json:"cody_usage_2"`
	CodyProviders                 json.RawMessage `json:"cody_providers"`
	RepoMetadataUsage             json.RawMessage `json:"repo_metadata_usage"`
}

func logPing(logger log.Logger, pubsubClient pubsub.TopicPublisher, meter *Meter, r *http.Request, pr *pingRequest, hasUpdate bool) {
	logger = logger.Scoped("logPing")
	defer func() {
		if err := recover(); err != nil {
			logger.Warn("panic", log.String("recover", fmt.Sprintf("%+v", err)))
			meter.ErrorCounter.Add(r.Context(), 1)
		}
	}()

	// Sync the initial administrator email in HubSpot.
	if strings.Contains(pr.InitialAdminEmail, "@") {
		// Hubspot requires the timestamp to be rounded to the nearest day at midnight.
		now := time.Now().UTC()
		rounded := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		millis := rounded.UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
		go hubspotutil.SyncUser(pr.InitialAdminEmail, "", &hubspot.ContactProperties{IsServerAdmin: true, LatestPing: millis, HasAgreedToToS: pr.TosAccepted})
	}

	var clientAddr string
	if v := r.Header.Get("x-forwarded-for"); v != "" {
		clientAddr = v
	} else {
		clientAddr = r.RemoteAddr
	}

	message, err := marshalPing(pr, hasUpdate, clientAddr, time.Now())
	if err != nil {
		meter.ErrorCounter.Add(r.Context(), 1)
		logger.Error("failed to marshal payload", log.Error(err))
		return
	}

	err = pubsubClient.Publish(context.Background(), message)
	if err != nil {
		meter.ErrorCounter.Add(r.Context(), 1)
		logger.Error("failed to publish", log.String("message", string(message)), log.Error(err))
		return
	}
}

func marshalPing(pr *pingRequest, hasUpdate bool, clientAddr string, now time.Time) ([]byte, error) {
	codeIntelUsage, err := reserializeCodeIntelUsage(pr.NewCodeIntelUsage, pr.CodeIntelUsage)
	if err != nil {
		return nil, errors.Wrap(err, "malformed code intel usage")
	}

	searchUsage, err := reserializeSearchUsage(pr.SearchUsage)
	if err != nil {
		return nil, errors.Wrap(err, "malformed search usage")
	}

	return json.Marshal(&pingPayload{
		RemoteIP:                      clientAddr,
		RemoteSiteVersion:             pr.ClientVersionString,
		RemoteSiteID:                  pr.ClientSiteID,
		LicenseKey:                    pr.LicenseKey,
		ExternalURL:                   pr.ExternalURL,
		Os:                            pr.Os,
		HasUpdate:                     strconv.FormatBool(hasUpdate),
		UniqueUsersToday:              strconv.FormatInt(int64(pr.UniqueUsers), 10),
		SiteActivity:                  pr.Activity,          // no change in schema
		BatchChangesUsage:             pr.BatchChangesUsage, // no change in schema
		NewCodeIntelUsage:             codeIntelUsage,
		SearchUsage:                   searchUsage,
		GrowthStatistics:              pr.GrowthStatistics,
		SavedSearches:                 pr.SavedSearches,
		HomepagePanels:                pr.HomepagePanels,
		RetentionStatistics:           pr.RetentionStatistics,
		Repositories:                  pr.Repositories,
		RepositorySizeHistogram:       pr.RepositorySizeHistogram,
		SearchOnboarding:              pr.SearchOnboarding,
		InstallerEmail:                pr.InitialAdminEmail,
		DependencyVersions:            pr.DependencyVersions,
		ExtensionsUsage:               pr.ExtensionsUsage,
		CodeInsightsUsage:             pr.CodeInsightsUsage,
		CodeInsightsCriticalTelemetry: pr.CodeInsightsCriticalTelemetry,
		SearchJobsUsage:               pr.SearchJobsUsage,
		CodeMonitoringUsage:           pr.CodeMonitoringUsage,
		NotebooksUsage:                pr.NotebooksUsage,
		CodeHostVersions:              pr.CodeHostVersions,
		CodeHostIntegrationUsage:      pr.CodeHostIntegrationUsage,
		IDEExtensionsUsage:            pr.IDEExtensionsUsage,
		OwnUsage:                      pr.OwnUsage,
		AuthProviders:                 strings.Join(pr.AuthProviders, ","),
		ExtServices:                   strings.Join(pr.ExternalServices, ","),
		BuiltinSignupAllowed:          strconv.FormatBool(pr.BuiltinSignupAllowed),
		AccessRequestEnabled:          strconv.FormatBool(pr.AccessRequestEnabled),
		DeployType:                    pr.DeployType,
		TotalUserAccounts:             strconv.FormatInt(int64(pr.TotalUsers), 10),
		TotalRepos:                    strconv.FormatInt(int64(pr.TotalRepos), 10),
		HasExternalURL:                strconv.FormatBool(pr.HasExtURL),
		HasRepos:                      strconv.FormatBool(pr.HasRepos),
		EverSearched:                  strconv.FormatBool(pr.EverSearched),
		EverFindRefs:                  strconv.FormatBool(pr.EverFindRefs),
		ActiveToday:                   strconv.FormatBool(pr.ActiveToday),
		Timestamp:                     now.UTC().Format(time.RFC3339),
		HasCodyEnabled:                strconv.FormatBool(pr.HasCodyEnabled),
		CodyUsage2:                    pr.CodyUsage2,
		CodyProviders:                 pr.CodyProviders,
		RepoMetadataUsage:             pr.RepoMetadataUsage,
	})
}

// reserializeCodeIntelUsage returns the given data in the shape of the current code intel
// usage statistics format. The given payload should be populated with either the new-style
func reserializeCodeIntelUsage(payload, fallbackPayload json.RawMessage) (json.RawMessage, error) {
	if len(payload) != 0 {
		return reserializeNewCodeIntelUsage(payload)
	}
	if len(fallbackPayload) != 0 {
		return reserializeOldCodeIntelUsage(fallbackPayload)
	}

	return nil, nil
}

func reserializeNewCodeIntelUsage(payload json.RawMessage) (json.RawMessage, error) {
	var codeIntelUsage *types.NewCodeIntelUsageStatistics
	if err := json.Unmarshal(payload, &codeIntelUsage); err != nil {
		return nil, err
	}
	if codeIntelUsage == nil {
		return nil, nil
	}

	var eventSummaries []jsonEventSummary
	for _, event := range codeIntelUsage.EventSummaries {
		eventSummaries = append(eventSummaries, translateEventSummary(event))
	}

	var investigationEvents []jsonCodeIntelInvestigationEvent
	for _, event := range codeIntelUsage.InvestigationEvents {
		investigationEvents = append(investigationEvents, translateInvestigationEvent(event))
	}

	countsByLanguage := make([]jsonCodeIntelRepositoryCountsByLanguage, 0, len(codeIntelUsage.CountsByLanguage))
	for language, counts := range codeIntelUsage.CountsByLanguage {
		// note: do not capture loop var by ref
		languageID := language

		countsByLanguage = append(countsByLanguage, jsonCodeIntelRepositoryCountsByLanguage{
			LanguageID:                            &languageID,
			NumRepositoriesWithUploadRecords:      counts.NumRepositoriesWithUploadRecords,
			NumRepositoriesWithFreshUploadRecords: counts.NumRepositoriesWithFreshUploadRecords,
			NumRepositoriesWithIndexRecords:       counts.NumRepositoriesWithIndexRecords,
			NumRepositoriesWithFreshIndexRecords:  counts.NumRepositoriesWithFreshIndexRecords,
		})
	}
	sort.Slice(countsByLanguage, func(i, j int) bool {
		return *countsByLanguage[i].LanguageID < *countsByLanguage[j].LanguageID
	})

	numRepositories := codeIntelUsage.NumRepositories
	if numRepositories == nil && codeIntelUsage.NumRepositoriesWithUploadRecords != nil && codeIntelUsage.NumRepositoriesWithoutUploadRecords != nil {
		val := *codeIntelUsage.NumRepositoriesWithUploadRecords + *codeIntelUsage.NumRepositoriesWithoutUploadRecords
		numRepositories = &val
	}

	var numRepositoriesWithoutUploadRecords *int32
	if codeIntelUsage.NumRepositories != nil && codeIntelUsage.NumRepositoriesWithUploadRecords != nil {
		val := *codeIntelUsage.NumRepositories - *codeIntelUsage.NumRepositoriesWithUploadRecords
		numRepositoriesWithoutUploadRecords = &val
	}

	languageRequests := make([]jsonLanguageRequest, 0, len(codeIntelUsage.LanguageRequests))
	for _, request := range codeIntelUsage.LanguageRequests {
		// note: do not capture loop var by ref
		request := request

		languageRequests = append(languageRequests, jsonLanguageRequest{
			LanguageID:  &request.LanguageID,
			NumRequests: &request.NumRequests,
		})
	}

	return json.Marshal(jsonCodeIntelUsage{
		StartOfWeek:                                  codeIntelUsage.StartOfWeek,
		WAUs:                                         codeIntelUsage.WAUs,
		PreciseWAUs:                                  codeIntelUsage.PreciseWAUs,
		SearchBasedWAUs:                              codeIntelUsage.SearchBasedWAUs,
		CrossRepositoryWAUs:                          codeIntelUsage.CrossRepositoryWAUs,
		PreciseCrossRepositoryWAUs:                   codeIntelUsage.PreciseCrossRepositoryWAUs,
		SearchBasedCrossRepositoryWAUs:               codeIntelUsage.SearchBasedCrossRepositoryWAUs,
		EventSummaries:                               eventSummaries,
		NumRepositories:                              numRepositories,
		NumRepositoriesWithUploadRecords:             codeIntelUsage.NumRepositoriesWithUploadRecords,
		NumRepositoriesWithoutUploadRecords:          numRepositoriesWithoutUploadRecords,
		NumRepositoriesWithFreshUploadRecords:        codeIntelUsage.NumRepositoriesWithFreshUploadRecords,
		NumRepositoriesWithIndexRecords:              codeIntelUsage.NumRepositoriesWithIndexRecords,
		NumRepositoriesWithFreshIndexRecords:         codeIntelUsage.NumRepositoriesWithFreshIndexRecords,
		NumRepositoriesWithIndexConfigurationRecords: codeIntelUsage.NumRepositoriesWithAutoIndexConfigurationRecords,
		CountsByLanguage:                             countsByLanguage,
		SettingsPageViewCount:                        codeIntelUsage.SettingsPageViewCount,
		UsersWithRefPanelRedesignEnabled:             codeIntelUsage.UsersWithRefPanelRedesignEnabled,
		LanguageRequests:                             languageRequests,
		InvestigationEvents:                          investigationEvents,
	})
}

func reserializeOldCodeIntelUsage(payload json.RawMessage) (json.RawMessage, error) {
	var codeIntelUsage *types.OldCodeIntelUsageStatistics
	if err := json.Unmarshal(payload, &codeIntelUsage); err != nil {
		return nil, err
	}
	if codeIntelUsage == nil || len(codeIntelUsage.Weekly) == 0 {
		return nil, nil
	}

	unwrap := func(i *int32) int32 {
		if i == nil {
			return 0
		}
		return *i
	}

	week := codeIntelUsage.Weekly[0]
	hover := week.Hover
	definitions := week.Definitions
	references := week.References

	eventSummaries := []jsonEventSummary{
		{Action: "hover", Source: "precise", WAUs: hover.LSIF.UsersCount, TotalActions: unwrap(hover.LSIF.EventsCount)},
		{Action: "hover", Source: "search", WAUs: hover.Search.UsersCount, TotalActions: unwrap(hover.Search.EventsCount)},
		{Action: "definitions", Source: "precise", WAUs: definitions.LSIF.UsersCount, TotalActions: unwrap(definitions.LSIF.EventsCount)},
		{Action: "definitions", Source: "search", WAUs: definitions.Search.UsersCount, TotalActions: unwrap(definitions.Search.EventsCount)},
		{Action: "references", Source: "precise", WAUs: references.LSIF.UsersCount, TotalActions: unwrap(references.LSIF.EventsCount)},
		{Action: "references", Source: "search", WAUs: references.Search.UsersCount, TotalActions: unwrap(references.Search.EventsCount)},
	}

	return json.Marshal(jsonCodeIntelUsage{
		StartOfWeek:    week.StartTime,
		EventSummaries: eventSummaries,
	})
}

type jsonCodeIntelUsage struct {
	StartOfWeek                                  time.Time                                 `json:"start_time"`
	WAUs                                         *int32                                    `json:"waus"`
	PreciseWAUs                                  *int32                                    `json:"precise_waus"`
	SearchBasedWAUs                              *int32                                    `json:"search_waus"`
	CrossRepositoryWAUs                          *int32                                    `json:"xrepo_waus"`
	PreciseCrossRepositoryWAUs                   *int32                                    `json:"precise_xrepo_waus"`
	SearchBasedCrossRepositoryWAUs               *int32                                    `json:"search_xrepo_waus"`
	EventSummaries                               []jsonEventSummary                        `json:"event_summaries"`
	NumRepositories                              *int32                                    `json:"num_repositories"`
	NumRepositoriesWithUploadRecords             *int32                                    `json:"num_repositories_with_upload_records"`
	NumRepositoriesWithoutUploadRecords          *int32                                    `json:"num_repositories_without_upload_records"`
	NumRepositoriesWithFreshUploadRecords        *int32                                    `json:"num_repositories_with_fresh_upload_records"`
	NumRepositoriesWithIndexRecords              *int32                                    `json:"num_repositories_with_index_records"`
	NumRepositoriesWithFreshIndexRecords         *int32                                    `json:"num_repositories_with_fresh_index_records"`
	NumRepositoriesWithIndexConfigurationRecords *int32                                    `json:"num_repositories_with_index_configuration_records"`
	CountsByLanguage                             []jsonCodeIntelRepositoryCountsByLanguage `json:"counts_by_language"`
	SettingsPageViewCount                        *int32                                    `json:"settings_page_view_count"`
	UsersWithRefPanelRedesignEnabled             *int32                                    `json:"users_with_ref_panel_redesign_enabled"`
	LanguageRequests                             []jsonLanguageRequest                     `json:"language_requests"`
	InvestigationEvents                          []jsonCodeIntelInvestigationEvent         `json:"investigation_events"`
}

type jsonCodeIntelRepositoryCountsByLanguage struct {
	LanguageID                            *string `json:"language_id"`
	NumRepositoriesWithUploadRecords      *int32  `json:"num_repositories_with_upload_records"`
	NumRepositoriesWithFreshUploadRecords *int32  `json:"num_repositories_with_fresh_upload_records"`
	NumRepositoriesWithIndexRecords       *int32  `json:"num_repositories_with_index_records"`
	NumRepositoriesWithFreshIndexRecords  *int32  `json:"num_repositories_with_fresh_index_records"`
}

type jsonLanguageRequest struct {
	LanguageID  *string `json:"language_id"`
	NumRequests *int32  `json:"num_requests"`
}

type jsonCodeIntelInvestigationEvent struct {
	Type  string `json:"type"`
	WAUs  int32  `json:"waus"`
	Total int32  `json:"total"`
}

type jsonEventSummary struct {
	Action          string `json:"action"`
	Source          string `json:"source"`
	LanguageID      string `json:"language_id"`
	CrossRepository bool   `json:"cross_repository"`
	WAUs            int32  `json:"waus"`
	TotalActions    int32  `json:"total_actions"`
}

var codeIntelActionNames = map[types.CodeIntelAction]string{
	types.HoverAction:       "hover",
	types.DefinitionsAction: "definitions",
	types.ReferencesAction:  "references",
}

var codeIntelSourceNames = map[types.CodeIntelSource]string{
	types.PreciseSource: "precise",
	types.SearchSource:  "search",
}

var codeIntelInvestigationTypeNames = map[types.CodeIntelInvestigationType]string{
	types.CodeIntelIndexerSetupInvestigationType: "CodeIntelligenceIndexerSetupInvestigated",
	types.CodeIntelUploadErrorInvestigationType:  "CodeIntelligenceUploadErrorInvestigated",
	types.CodeIntelIndexErrorInvestigationType:   "CodeIntelligenceIndexErrorInvestigated",
}

func translateEventSummary(event types.CodeIntelEventSummary) jsonEventSummary {
	return jsonEventSummary{
		Action:          codeIntelActionNames[event.Action],
		Source:          codeIntelSourceNames[event.Source],
		LanguageID:      event.LanguageID,
		CrossRepository: event.CrossRepository,
		WAUs:            event.WAUs,
		TotalActions:    event.TotalActions,
	}
}

func translateInvestigationEvent(event types.CodeIntelInvestigationEvent) jsonCodeIntelInvestigationEvent {
	return jsonCodeIntelInvestigationEvent{
		Type:  codeIntelInvestigationTypeNames[event.Type],
		WAUs:  event.WAUs,
		Total: event.Total,
	}
}

// reserializeSearchUsage will reserialize a code intel usage statistics
// struct with only the first period in each period type. This reduces the
// complexity required in the BigQuery schema and downstream ETL transform
// logic.
func reserializeSearchUsage(payload json.RawMessage) (json.RawMessage, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var searchUsage *types.SearchUsageStatistics
	if err := json.Unmarshal(payload, &searchUsage); err != nil {
		return nil, err
	}
	if searchUsage == nil {
		return nil, nil
	}

	singlePeriodUsage := struct {
		Daily   *types.SearchUsagePeriod
		Weekly  *types.SearchUsagePeriod
		Monthly *types.SearchUsagePeriod
	}{}

	if len(searchUsage.Daily) > 0 {
		singlePeriodUsage.Daily = searchUsage.Daily[0]
	}
	if len(searchUsage.Weekly) > 0 {
		singlePeriodUsage.Weekly = searchUsage.Weekly[0]
	}
	if len(searchUsage.Monthly) > 0 {
		singlePeriodUsage.Monthly = searchUsage.Monthly[0]
	}

	return json.Marshal(singlePeriodUsage)
}
