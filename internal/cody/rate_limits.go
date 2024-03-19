package cody

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/accesstoken"
	"github.com/sourcegraph/sourcegraph/internal/codygateway"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/hashutil"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// RefreshGatewayRateLimits refreshes the rate limits for the user on Cody Gateway.
func RefreshGatewayRateLimits(ctx context.Context, userID int32, db database.DB) (error, int) {
	logger := log.Scoped("RefreshGatewayRateLimits")
	resp, err := requestGatewayWithUserCreds(ctx, userID, db, http.MethodPost, "/v1/limits/refresh", nil)
	if err != nil {
		logger.Error("failed request to Cody Gateway to refresh rate limits", log.Error(err))
		return err, http.StatusInternalServerError
	}
	// Both resp and err are nil if cody gateway is not configured as the completions provider.
	if resp == nil {
		return nil, http.StatusOK
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("non-200 response refreshing Gateway rate limits: %d", resp.StatusCode))
		return errors.Errorf("non-200 response refreshing Gateway rate limits: %d", resp.StatusCode), resp.StatusCode
	}

	return nil, http.StatusOK
}

// GetGatewayRateLimits fetches rate limits values for the user from Cody Gateway.
func GetGatewayRateLimits(ctx context.Context, userID int32, db database.DB) ([]codygateway.LimitStatus, error) {
	logger := log.Scoped("GetGatewayRateLimits")

	resp, err := requestGatewayWithUserCreds(ctx, userID, db, http.MethodGet, "/v1/limits", nil)
	if err != nil {
		logger.Error("failed request to Cody Gateway to fetch rate limits", log.Error(err))
		return []codygateway.LimitStatus{}, err
	}
	// Both resp and err are nil if cody gateway is not configured as the completions provider.
	if resp == nil {
		return []codygateway.LimitStatus{}, nil
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error(fmt.Sprintf("non-200 response fetching rate limits from Gateway: %d", resp.StatusCode))
		return []codygateway.LimitStatus{}, errors.Errorf("non-200 response fetching rate limits from Gateway: %d", resp.StatusCode)
	}

	var featureLimits map[string]codygateway.LimitStatus
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&featureLimits); err != nil {
		return nil, err
	}

	rateLimits := make([]codygateway.LimitStatus, 0, len(featureLimits))
	for f, limit := range featureLimits {
		feat := codygateway.Feature(f)
		// Check if this is a limit for a feature we know about.
		if feat.IsValid() {
			limit.Feature = feat
			rateLimits = append(rateLimits, limit)
		}
	}

	// Make sure the limits are always returned in the same order, since the map
	// above doesn't have deterministic ordering.
	sort.Slice(rateLimits, func(i, j int) bool {
		return rateLimits[i].Feature < rateLimits[j].Feature
	})

	return rateLimits, nil
}

func requestGatewayWithUserCreds(ctx context.Context, userID int32, db database.DB, method string, pathname string, body io.Reader) (*http.Response, error) {
	completionsConfig := conf.GetCompletionsConfig(conf.Get().SiteConfig())
	// We don't need to do anything if the target is not Cody Gateway, but it's not an error either.
	if completionsConfig.Provider != conftypes.CompletionsProviderNameSourcegraph {
		return nil, nil
	}

	apiTokenSha256, err := db.AccessTokens().GetOrCreateInternalToken(ctx, userID, []string{"user:all"})
	if err != nil {
		return nil, errors.Wrap(err, "getting internal access token")
	}
	gatewayToken := accesstoken.DotcomUserGatewayAccessTokenPrefix + hex.EncodeToString(hashutil.ToSHA256Bytes(apiTokenSha256))

	req, err := http.NewRequestWithContext(ctx, method, completionsConfig.Endpoint+pathname, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", gatewayToken))

	return httpcli.UncachedExternalDoer.Do(req)
}
