package graphqlbackend

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/hubspot/hubspotutil"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/usagestats"
)

func (r *UserResolver) UsageStatistics(ctx context.Context) (*userUsageStatisticsResolver, error) {
	if err := auth.CheckSiteAdminOrSameUser(ctx, r.db, r.user.ID); err != nil {
		return nil, err
	}

	stats, err := usagestats.GetByUserID(ctx, r.db, r.user.ID)
	if err != nil {
		return nil, err
	}
	return &userUsageStatisticsResolver{stats}, nil
}

type userUsageStatisticsResolver struct {
	userUsageStatistics *types.UserUsageStatistics
}

func (s *userUsageStatisticsResolver) PageViews() int32 { return s.userUsageStatistics.PageViews }

func (s *userUsageStatisticsResolver) SearchQueries() int32 {
	return s.userUsageStatistics.SearchQueries
}

func (s *userUsageStatisticsResolver) CodeIntelligenceActions() int32 {
	return s.userUsageStatistics.CodeIntelligenceActions
}

func (s *userUsageStatisticsResolver) FindReferencesActions() int32 {
	return s.userUsageStatistics.FindReferencesActions
}

func (s *userUsageStatisticsResolver) LastActiveTime() *string {
	if s.userUsageStatistics.LastActiveTime != nil {
		t := s.userUsageStatistics.LastActiveTime.Format(time.RFC3339)
		return &t
	}
	return nil
}

func (s *userUsageStatisticsResolver) LastActiveCodeHostIntegrationTime() *string {
	if s.userUsageStatistics.LastCodeHostIntegrationTime != nil {
		t := s.userUsageStatistics.LastCodeHostIntegrationTime.Format(time.RFC3339)
		return &t
	}
	return nil
}

// LogUserEvent is no longer used, only here for backwards compatibility with IDE and browser extensions.
// Functionality removed in https://github.com/sourcegraph/sourcegraph/pull/38826.
func (*schemaResolver) LogUserEvent(ctx context.Context, args *struct {
	Event        string
	UserCookieID string
}) (*EmptyResponse, error) {
	return nil, nil
}

type Event struct {
	Event                  string
	UserCookieID           string
	FirstSourceURL         *string
	LastSourceURL          *string
	URL                    string
	Source                 string
	Argument               *string
	CohortID               *string
	Referrer               *string
	OriginalReferrer       *string
	SessionReferrer        *string
	SessionFirstURL        *string
	DeviceSessionID        *string
	PublicArgument         *string
	UserProperties         *string
	DeviceID               *string
	InsertID               *string
	EventID                *int32
	Client                 *string
	BillingProductCategory *string
	BillingEventID         *string
	ConnectedSiteID        *string
	HashedLicenseKey       *string
}

type EventBatch struct {
	Events *[]Event
}

// LogEvent is the deprecated mutation, superceded by { telemetry { recordEvents } }
func (r *schemaResolver) LogEvent(ctx context.Context, args *Event) (*EmptyResponse, error) {
	if args == nil {
		return nil, nil
	}

	return r.LogEvents(ctx, &EventBatch{Events: &[]Event{*args}})
}

// LogEvents is the deprecated mutation, superceded by { telemetry { recordEvents } }
func (r *schemaResolver) LogEvents(ctx context.Context, args *EventBatch) (*EmptyResponse, error) {
	if !conf.EventLoggingEnabled() || args.Events == nil {
		return nil, nil
	}

	userID := actor.FromContext(ctx).UID

	events := make([]usagestats.Event, 0, len(*args.Events))
	for _, args := range *args.Events {
		if strings.HasPrefix(args.Event, "search.latencies.frontend.") {
			argumentPayload, err := decode(args.Argument)
			if err != nil {
				return nil, err
			}

			if err := exportPrometheusSearchLatencies(args.Event, argumentPayload); err != nil {
				log15.Error("export prometheus search latencies", "error", err)
			}

			// Future(slimsag): implement actual event logging for these events
			continue
		}

		if strings.HasPrefix(args.Event, "search.ranking.") {
			argumentPayload, err := decode(args.Argument)
			if err != nil {
				return nil, err
			}
			if err := exportPrometheusSearchRanking(argumentPayload); err != nil {
				log15.Error("exportPrometheusSearchRanking", "error", err)
			}
			continue
		}

		// On Sourcegraph.com only, log a HubSpot event indicating when the user installed a Cody client.
		// if  dotcom.SourcegraphDotComMode() && args.Event == "CodyInstalled" && userID != 0 && userPrimaryEmail != "" {
		if dotcom.SourcegraphDotComMode() && args.Event == "CodyInstalled" {
			emailsEnabled := false

			ide := getIdeFromEvent(&args)

			if strings.ToLower(ide) == "vscode" {
				if ffs := featureflag.FromContext(ctx); ffs != nil {
					emailsEnabled = ffs.GetBoolOr("vscodeCodyEmailsEnabled", false)
				}
			}

			userPrimaryEmail, _, _ := r.db.UserEmails().GetPrimaryEmail(ctx, userID)

			hubspotutil.SyncUser(userPrimaryEmail, hubspotutil.CodyClientInstalledEventID, &hubspot.ContactProperties{
				DatabaseID: userID,
			})

			hubspotutil.SyncUserWithV3Event(userPrimaryEmail, hubspotutil.CodyClientInstalledV3EventID,
				&hubspot.ContactProperties{
					DatabaseID: userID,
				},
				&hubspot.CodyInstallV3EventProperties{
					Ide:           ide,
					EmailsEnabled: strconv.FormatBool(emailsEnabled),
				})
		}

		argumentPayload, err := decode(args.Argument)
		if err != nil {
			return nil, err
		}

		publicArgumentPayload, err := decode(args.PublicArgument)
		if err != nil {
			return nil, err
		}

		userPropertiesPayload, err := decode(args.UserProperties)
		if err != nil {
			return nil, err
		}

		events = append(events, usagestats.Event{
			EventName:              args.Event,
			URL:                    args.URL,
			UserID:                 userID,
			UserCookieID:           args.UserCookieID,
			FirstSourceURL:         args.FirstSourceURL,
			LastSourceURL:          args.LastSourceURL,
			Source:                 args.Source,
			Argument:               argumentPayload,
			EvaluatedFlagSet:       featureflag.GetEvaluatedFlagSet(ctx),
			CohortID:               args.CohortID,
			Referrer:               args.Referrer,
			OriginalReferrer:       args.OriginalReferrer,
			SessionReferrer:        args.SessionReferrer,
			SessionFirstURL:        args.SessionFirstURL,
			PublicArgument:         publicArgumentPayload,
			UserProperties:         userPropertiesPayload,
			DeviceID:               args.DeviceID,
			EventID:                args.EventID,
			InsertID:               args.InsertID,
			DeviceSessionID:        args.DeviceSessionID,
			Client:                 args.Client,
			BillingProductCategory: args.BillingProductCategory,
			BillingEventID:         args.BillingEventID,
			ConnectedSiteID:        args.ConnectedSiteID,
			HashedLicenseKey:       args.HashedLicenseKey,
		})
	}

	//lint:ignore SA1019 existing usage of deprecated functionality to back deprecated GraphQL mutation
	if err := usagestats.LogEvents(ctx, r.db, events); err != nil {
		return nil, err
	}

	return nil, nil
}

func decode(v *string) (payload json.RawMessage, _ error) {
	if v != nil {
		if err := json.Unmarshal([]byte(*v), &payload); err != nil {
			return nil, err
		}
	}

	return payload, nil
}

type VSCodeEventExtensionDetails struct {
	Ide string `json:"ide"`
}

type VSCodeEventPublicArgument struct {
	ExtensionDetails VSCodeEventExtensionDetails `json:"extensionDetails"`
}

func getIdeFromEvent(args *Event) string {
	payload, err := decode(args.PublicArgument)
	if err != nil {
		return ""
	}

	var argument VSCodeEventPublicArgument

	if err := json.Unmarshal(payload, &argument); err != nil {
		return ""
	}

	return argument.ExtensionDetails.Ide
}

var (
	searchLatenciesFrontendCodeLoad = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_search_latency_frontend_code_load_seconds",
		Help:    "Milliseconds the webapp frontend spends waiting for search result code snippets to load.",
		Buckets: trace.UserLatencyBuckets,
	}, nil)
	searchLatenciesFrontendFirstResult = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "src_search_latency_frontend_first_result_seconds",
		Help:    "Milliseconds the webapp frontend spends waiting for the first search result to load.",
		Buckets: trace.UserLatencyBuckets,
	}, []string{"type"})
)

// exportPrometheusSearchLatencies exports Prometheus search latency metrics given a GraphQL
// LogEvent payload.
func exportPrometheusSearchLatencies(event string, payload json.RawMessage) error {
	var v struct {
		DurationMS float64 `json:"durationMs"`
	}
	if err := json.Unmarshal(payload, &v); err != nil {
		return err
	}
	if event == "search.latencies.frontend.code-load" {
		searchLatenciesFrontendCodeLoad.WithLabelValues().Observe(v.DurationMS / 1000.0)
	}
	if strings.HasPrefix(event, "search.latencies.frontend.") && strings.HasSuffix(event, ".first-result") {
		searchType := strings.TrimSuffix(strings.TrimPrefix(event, "search.latencies.frontend."), ".first-result")
		searchLatenciesFrontendFirstResult.WithLabelValues(searchType).Observe(v.DurationMS / 1000.0)
	}
	return nil
}

var searchRankingResultClicked = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "src_search_ranking_result_clicked",
	Help:    "the index of the search result which was clicked on by the user",
	Buckets: prometheus.LinearBuckets(1, 1, 10),
}, []string{"type", "resultsLength", "ranked"})

func exportPrometheusSearchRanking(payload json.RawMessage) error {
	var v struct {
		Index         float64 `json:"index"`
		Type          string  `json:"type"`
		ResultsLength int     `json:"resultsLength"`
		Ranked        bool    `json:"ranked"`
	}

	if err := json.Unmarshal(payload, &v); err != nil {
		return err
	}

	var resultsLength string
	switch {
	case v.ResultsLength <= 3:
		resultsLength = "<=3"
	default:
		resultsLength = ">3"
	}

	ranked := strconv.FormatBool(v.Ranked)

	searchRankingResultClicked.WithLabelValues(v.Type, resultsLength, ranked).Observe(v.Index)
	return nil
}
