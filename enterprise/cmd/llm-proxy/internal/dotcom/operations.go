// Code generated by github.com/Khan/genqlient, DO NOT EDIT.

package dotcom

import (
	"context"
	"encoding/json"

	"github.com/Khan/genqlient/graphql"
)

// CheckAccessTokenDotcomDotcomQuery includes the requested fields of the GraphQL type DotcomQuery.
// The GraphQL type's documentation follows.
//
// Mutations that are only used on Sourcegraph.com.
// FOR INTERNAL USE ONLY.
type CheckAccessTokenDotcomDotcomQuery struct {
	// The access available to the product subscription with the given access token.
	// The returned ProductSubscription may be archived or not associated with an active license.
	//
	// Only Sourcegraph.com site admins, the account owners of the product subscription, and
	// specific service accounts may perform this query.
	// FOR INTERNAL USE ONLY.
	ProductSubscriptionByAccessToken CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription `json:"productSubscriptionByAccessToken"`
}

// GetProductSubscriptionByAccessToken returns CheckAccessTokenDotcomDotcomQuery.ProductSubscriptionByAccessToken, and is useful for accessing the field via an interface.
func (v *CheckAccessTokenDotcomDotcomQuery) GetProductSubscriptionByAccessToken() CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription {
	return v.ProductSubscriptionByAccessToken
}

// CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription includes the requested fields of the GraphQL type ProductSubscription.
// The GraphQL type's documentation follows.
//
// A product subscription that was created on Sourcegraph.com.
// FOR INTERNAL USE ONLY.
type CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription struct {
	ProductSubscriptionState `json:"-"`
}

// GetId returns CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription.Id, and is useful for accessing the field via an interface.
func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) GetId() string {
	return v.ProductSubscriptionState.Id
}

// GetUuid returns CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription.Uuid, and is useful for accessing the field via an interface.
func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) GetUuid() string {
	return v.ProductSubscriptionState.Uuid
}

// GetIsArchived returns CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription.IsArchived, and is useful for accessing the field via an interface.
func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) GetIsArchived() bool {
	return v.ProductSubscriptionState.IsArchived
}

// GetLlmProxyAccess returns CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription.LlmProxyAccess, and is useful for accessing the field via an interface.
func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) GetLlmProxyAccess() ProductSubscriptionStateLlmProxyAccessLLMProxyAccess {
	return v.ProductSubscriptionState.LlmProxyAccess
}

// GetActiveLicense returns CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription.ActiveLicense, and is useful for accessing the field via an interface.
func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) GetActiveLicense() *ProductSubscriptionStateActiveLicenseProductLicense {
	return v.ProductSubscriptionState.ActiveLicense
}

func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) UnmarshalJSON(b []byte) error {

	if string(b) == "null" {
		return nil
	}

	var firstPass struct {
		*CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription
		graphql.NoUnmarshalJSON
	}
	firstPass.CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription = v

	err := json.Unmarshal(b, &firstPass)
	if err != nil {
		return err
	}

	err = json.Unmarshal(
		b, &v.ProductSubscriptionState)
	if err != nil {
		return err
	}
	return nil
}

type __premarshalCheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription struct {
	Id string `json:"id"`

	Uuid string `json:"uuid"`

	IsArchived bool `json:"isArchived"`

	LlmProxyAccess ProductSubscriptionStateLlmProxyAccessLLMProxyAccess `json:"llmProxyAccess"`

	ActiveLicense *ProductSubscriptionStateActiveLicenseProductLicense `json:"activeLicense"`
}

func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) MarshalJSON() ([]byte, error) {
	premarshaled, err := v.__premarshalJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(premarshaled)
}

func (v *CheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription) __premarshalJSON() (*__premarshalCheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription, error) {
	var retval __premarshalCheckAccessTokenDotcomDotcomQueryProductSubscriptionByAccessTokenProductSubscription

	retval.Id = v.ProductSubscriptionState.Id
	retval.Uuid = v.ProductSubscriptionState.Uuid
	retval.IsArchived = v.ProductSubscriptionState.IsArchived
	retval.LlmProxyAccess = v.ProductSubscriptionState.LlmProxyAccess
	retval.ActiveLicense = v.ProductSubscriptionState.ActiveLicense
	return &retval, nil
}

// CheckAccessTokenResponse is returned by CheckAccessToken on success.
type CheckAccessTokenResponse struct {
	// Queries that are only used on Sourcegraph.com.
	//
	// FOR INTERNAL USE ONLY.
	Dotcom CheckAccessTokenDotcomDotcomQuery `json:"dotcom"`
}

// GetDotcom returns CheckAccessTokenResponse.Dotcom, and is useful for accessing the field via an interface.
func (v *CheckAccessTokenResponse) GetDotcom() CheckAccessTokenDotcomDotcomQuery { return v.Dotcom }

// LLMProxyAccessFields includes the GraphQL fields of LLMProxyAccess requested by the fragment LLMProxyAccessFields.
// The GraphQL type's documentation follows.
//
// LLM-proxy access granted to a subscription.
// FOR INTERNAL USE ONLY.
type LLMProxyAccessFields struct {
	// Whether or not a subscription has LLM-proxy access.
	Enabled bool `json:"enabled"`
	// Rate limits for LLM-proxy access, or null if not enabled.
	RateLimit *LLMProxyAccessFieldsRateLimitLLMProxyRateLimit `json:"rateLimit"`
}

// GetEnabled returns LLMProxyAccessFields.Enabled, and is useful for accessing the field via an interface.
func (v *LLMProxyAccessFields) GetEnabled() bool { return v.Enabled }

// GetRateLimit returns LLMProxyAccessFields.RateLimit, and is useful for accessing the field via an interface.
func (v *LLMProxyAccessFields) GetRateLimit() *LLMProxyAccessFieldsRateLimitLLMProxyRateLimit {
	return v.RateLimit
}

// LLMProxyAccessFieldsRateLimitLLMProxyRateLimit includes the requested fields of the GraphQL type LLMProxyRateLimit.
// The GraphQL type's documentation follows.
//
// LLM-proxy access rate limits for a subscription.
// FOR INTERNAL USE ONLY.
type LLMProxyAccessFieldsRateLimitLLMProxyRateLimit struct {
	// The source of the rate limit configuration.
	Source LLMProxyRateLimitSource `json:"source"`
	// Requests per time interval.
	Limit int `json:"limit"`
	// Interval for rate limiting.
	IntervalSeconds int `json:"intervalSeconds"`
}

// GetSource returns LLMProxyAccessFieldsRateLimitLLMProxyRateLimit.Source, and is useful for accessing the field via an interface.
func (v *LLMProxyAccessFieldsRateLimitLLMProxyRateLimit) GetSource() LLMProxyRateLimitSource {
	return v.Source
}

// GetLimit returns LLMProxyAccessFieldsRateLimitLLMProxyRateLimit.Limit, and is useful for accessing the field via an interface.
func (v *LLMProxyAccessFieldsRateLimitLLMProxyRateLimit) GetLimit() int { return v.Limit }

// GetIntervalSeconds returns LLMProxyAccessFieldsRateLimitLLMProxyRateLimit.IntervalSeconds, and is useful for accessing the field via an interface.
func (v *LLMProxyAccessFieldsRateLimitLLMProxyRateLimit) GetIntervalSeconds() int {
	return v.IntervalSeconds
}

// The source of the rate limit returned.
// FOR INTERNAL USE ONLY.
type LLMProxyRateLimitSource string

const (
	// Indicates that a custom override for the rate limit has been stored.
	LLMProxyRateLimitSourceOverride LLMProxyRateLimitSource = "OVERRIDE"
	// Indicates that the rate limit is inferred by the subscriptions active plan.
	LLMProxyRateLimitSourcePlan LLMProxyRateLimitSource = "PLAN"
)

// ListProductSubscriptionFields includes the GraphQL fields of ProductSubscription requested by the fragment ListProductSubscriptionFields.
// The GraphQL type's documentation follows.
//
// A product subscription that was created on Sourcegraph.com.
// FOR INTERNAL USE ONLY.
type ListProductSubscriptionFields struct {
	ProductSubscriptionState `json:"-"`
	// Available access tokens for authenticating as the subscription holder with managed
	// Sourcegraph services.
	SourcegraphAccessTokens []string `json:"sourcegraphAccessTokens"`
}

// GetSourcegraphAccessTokens returns ListProductSubscriptionFields.SourcegraphAccessTokens, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionFields) GetSourcegraphAccessTokens() []string {
	return v.SourcegraphAccessTokens
}

// GetId returns ListProductSubscriptionFields.Id, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionFields) GetId() string { return v.ProductSubscriptionState.Id }

// GetUuid returns ListProductSubscriptionFields.Uuid, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionFields) GetUuid() string { return v.ProductSubscriptionState.Uuid }

// GetIsArchived returns ListProductSubscriptionFields.IsArchived, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionFields) GetIsArchived() bool {
	return v.ProductSubscriptionState.IsArchived
}

// GetLlmProxyAccess returns ListProductSubscriptionFields.LlmProxyAccess, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionFields) GetLlmProxyAccess() ProductSubscriptionStateLlmProxyAccessLLMProxyAccess {
	return v.ProductSubscriptionState.LlmProxyAccess
}

// GetActiveLicense returns ListProductSubscriptionFields.ActiveLicense, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionFields) GetActiveLicense() *ProductSubscriptionStateActiveLicenseProductLicense {
	return v.ProductSubscriptionState.ActiveLicense
}

func (v *ListProductSubscriptionFields) UnmarshalJSON(b []byte) error {

	if string(b) == "null" {
		return nil
	}

	var firstPass struct {
		*ListProductSubscriptionFields
		graphql.NoUnmarshalJSON
	}
	firstPass.ListProductSubscriptionFields = v

	err := json.Unmarshal(b, &firstPass)
	if err != nil {
		return err
	}

	err = json.Unmarshal(
		b, &v.ProductSubscriptionState)
	if err != nil {
		return err
	}
	return nil
}

type __premarshalListProductSubscriptionFields struct {
	SourcegraphAccessTokens []string `json:"sourcegraphAccessTokens"`

	Id string `json:"id"`

	Uuid string `json:"uuid"`

	IsArchived bool `json:"isArchived"`

	LlmProxyAccess ProductSubscriptionStateLlmProxyAccessLLMProxyAccess `json:"llmProxyAccess"`

	ActiveLicense *ProductSubscriptionStateActiveLicenseProductLicense `json:"activeLicense"`
}

func (v *ListProductSubscriptionFields) MarshalJSON() ([]byte, error) {
	premarshaled, err := v.__premarshalJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(premarshaled)
}

func (v *ListProductSubscriptionFields) __premarshalJSON() (*__premarshalListProductSubscriptionFields, error) {
	var retval __premarshalListProductSubscriptionFields

	retval.SourcegraphAccessTokens = v.SourcegraphAccessTokens
	retval.Id = v.ProductSubscriptionState.Id
	retval.Uuid = v.ProductSubscriptionState.Uuid
	retval.IsArchived = v.ProductSubscriptionState.IsArchived
	retval.LlmProxyAccess = v.ProductSubscriptionState.LlmProxyAccess
	retval.ActiveLicense = v.ProductSubscriptionState.ActiveLicense
	return &retval, nil
}

// ListProductSubscriptionsDotcomDotcomQuery includes the requested fields of the GraphQL type DotcomQuery.
// The GraphQL type's documentation follows.
//
// Mutations that are only used on Sourcegraph.com.
// FOR INTERNAL USE ONLY.
type ListProductSubscriptionsDotcomDotcomQuery struct {
	// A list of product subscriptions.
	// FOR INTERNAL USE ONLY.
	ProductSubscriptions ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection `json:"productSubscriptions"`
}

// GetProductSubscriptions returns ListProductSubscriptionsDotcomDotcomQuery.ProductSubscriptions, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQuery) GetProductSubscriptions() ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection {
	return v.ProductSubscriptions
}

// ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection includes the requested fields of the GraphQL type ProductSubscriptionConnection.
// The GraphQL type's documentation follows.
//
// A list of product subscriptions.
// FOR INTERNAL USE ONLY.
type ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection struct {
	// The total count of product subscriptions in the connection. This total count may be larger than the number of
	// nodes in this object when the result is paginated.
	TotalCount int `json:"totalCount"`
	// Pagination information.
	PageInfo ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo `json:"pageInfo"`
	// A list of product subscriptions.
	Nodes []ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription `json:"nodes"`
}

// GetTotalCount returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection.TotalCount, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection) GetTotalCount() int {
	return v.TotalCount
}

// GetPageInfo returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection.PageInfo, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection) GetPageInfo() ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo {
	return v.PageInfo
}

// GetNodes returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection.Nodes, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnection) GetNodes() []ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription {
	return v.Nodes
}

// ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription includes the requested fields of the GraphQL type ProductSubscription.
// The GraphQL type's documentation follows.
//
// A product subscription that was created on Sourcegraph.com.
// FOR INTERNAL USE ONLY.
type ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription struct {
	ListProductSubscriptionFields `json:"-"`
}

// GetSourcegraphAccessTokens returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription.SourcegraphAccessTokens, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) GetSourcegraphAccessTokens() []string {
	return v.ListProductSubscriptionFields.SourcegraphAccessTokens
}

// GetId returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription.Id, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) GetId() string {
	return v.ListProductSubscriptionFields.ProductSubscriptionState.Id
}

// GetUuid returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription.Uuid, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) GetUuid() string {
	return v.ListProductSubscriptionFields.ProductSubscriptionState.Uuid
}

// GetIsArchived returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription.IsArchived, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) GetIsArchived() bool {
	return v.ListProductSubscriptionFields.ProductSubscriptionState.IsArchived
}

// GetLlmProxyAccess returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription.LlmProxyAccess, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) GetLlmProxyAccess() ProductSubscriptionStateLlmProxyAccessLLMProxyAccess {
	return v.ListProductSubscriptionFields.ProductSubscriptionState.LlmProxyAccess
}

// GetActiveLicense returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription.ActiveLicense, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) GetActiveLicense() *ProductSubscriptionStateActiveLicenseProductLicense {
	return v.ListProductSubscriptionFields.ProductSubscriptionState.ActiveLicense
}

func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) UnmarshalJSON(b []byte) error {

	if string(b) == "null" {
		return nil
	}

	var firstPass struct {
		*ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription
		graphql.NoUnmarshalJSON
	}
	firstPass.ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription = v

	err := json.Unmarshal(b, &firstPass)
	if err != nil {
		return err
	}

	err = json.Unmarshal(
		b, &v.ListProductSubscriptionFields)
	if err != nil {
		return err
	}
	return nil
}

type __premarshalListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription struct {
	SourcegraphAccessTokens []string `json:"sourcegraphAccessTokens"`

	Id string `json:"id"`

	Uuid string `json:"uuid"`

	IsArchived bool `json:"isArchived"`

	LlmProxyAccess ProductSubscriptionStateLlmProxyAccessLLMProxyAccess `json:"llmProxyAccess"`

	ActiveLicense *ProductSubscriptionStateActiveLicenseProductLicense `json:"activeLicense"`
}

func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) MarshalJSON() ([]byte, error) {
	premarshaled, err := v.__premarshalJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(premarshaled)
}

func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription) __premarshalJSON() (*__premarshalListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription, error) {
	var retval __premarshalListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionNodesProductSubscription

	retval.SourcegraphAccessTokens = v.ListProductSubscriptionFields.SourcegraphAccessTokens
	retval.Id = v.ListProductSubscriptionFields.ProductSubscriptionState.Id
	retval.Uuid = v.ListProductSubscriptionFields.ProductSubscriptionState.Uuid
	retval.IsArchived = v.ListProductSubscriptionFields.ProductSubscriptionState.IsArchived
	retval.LlmProxyAccess = v.ListProductSubscriptionFields.ProductSubscriptionState.LlmProxyAccess
	retval.ActiveLicense = v.ListProductSubscriptionFields.ProductSubscriptionState.ActiveLicense
	return &retval, nil
}

// ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo includes the requested fields of the GraphQL type PageInfo.
// The GraphQL type's documentation follows.
//
// Pagination information for forward-only pagination. See https://facebook.github.io/relay/graphql/connections.htm#sec-undefined.PageInfo.
type ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo struct {
	// When paginating forwards, the cursor to continue.
	EndCursor *string `json:"endCursor"`
	// When paginating forwards, are there more items?
	HasNextPage bool `json:"hasNextPage"`
}

// GetEndCursor returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo.EndCursor, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo) GetEndCursor() *string {
	return v.EndCursor
}

// GetHasNextPage returns ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo.HasNextPage, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsDotcomDotcomQueryProductSubscriptionsProductSubscriptionConnectionPageInfo) GetHasNextPage() bool {
	return v.HasNextPage
}

// ListProductSubscriptionsResponse is returned by ListProductSubscriptions on success.
type ListProductSubscriptionsResponse struct {
	// Queries that are only used on Sourcegraph.com.
	//
	// FOR INTERNAL USE ONLY.
	Dotcom ListProductSubscriptionsDotcomDotcomQuery `json:"dotcom"`
}

// GetDotcom returns ListProductSubscriptionsResponse.Dotcom, and is useful for accessing the field via an interface.
func (v *ListProductSubscriptionsResponse) GetDotcom() ListProductSubscriptionsDotcomDotcomQuery {
	return v.Dotcom
}

// ProductSubscriptionState includes the GraphQL fields of ProductSubscription requested by the fragment ProductSubscriptionState.
// The GraphQL type's documentation follows.
//
// A product subscription that was created on Sourcegraph.com.
// FOR INTERNAL USE ONLY.
type ProductSubscriptionState struct {
	// The unique ID of this product subscription.
	Id string `json:"id"`
	// The unique UUID of this product subscription. Unlike ProductSubscription.id, this does not
	// encode the type and is not a GraphQL node ID.
	Uuid string `json:"uuid"`
	// Whether this product subscription was archived.
	IsArchived bool `json:"isArchived"`
	// LLM-proxy access granted to this subscription. Properties may be inferred from the active license, or be defined in overrides.
	LlmProxyAccess ProductSubscriptionStateLlmProxyAccessLLMProxyAccess `json:"llmProxyAccess"`
	// The currently active product license associated with this product subscription, if any.
	ActiveLicense *ProductSubscriptionStateActiveLicenseProductLicense `json:"activeLicense"`
}

// GetId returns ProductSubscriptionState.Id, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionState) GetId() string { return v.Id }

// GetUuid returns ProductSubscriptionState.Uuid, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionState) GetUuid() string { return v.Uuid }

// GetIsArchived returns ProductSubscriptionState.IsArchived, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionState) GetIsArchived() bool { return v.IsArchived }

// GetLlmProxyAccess returns ProductSubscriptionState.LlmProxyAccess, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionState) GetLlmProxyAccess() ProductSubscriptionStateLlmProxyAccessLLMProxyAccess {
	return v.LlmProxyAccess
}

// GetActiveLicense returns ProductSubscriptionState.ActiveLicense, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionState) GetActiveLicense() *ProductSubscriptionStateActiveLicenseProductLicense {
	return v.ActiveLicense
}

// ProductSubscriptionStateActiveLicenseProductLicense includes the requested fields of the GraphQL type ProductLicense.
// The GraphQL type's documentation follows.
//
// A product license that was created on Sourcegraph.com.
// FOR INTERNAL USE ONLY.
type ProductSubscriptionStateActiveLicenseProductLicense struct {
	// Information about this product license.
	Info *ProductSubscriptionStateActiveLicenseProductLicenseInfo `json:"info"`
}

// GetInfo returns ProductSubscriptionStateActiveLicenseProductLicense.Info, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionStateActiveLicenseProductLicense) GetInfo() *ProductSubscriptionStateActiveLicenseProductLicenseInfo {
	return v.Info
}

// ProductSubscriptionStateActiveLicenseProductLicenseInfo includes the requested fields of the GraphQL type ProductLicenseInfo.
// The GraphQL type's documentation follows.
//
// Information about this site's product license (which activates certain Sourcegraph features).
type ProductSubscriptionStateActiveLicenseProductLicenseInfo struct {
	// Tags indicating the product plan and features activated by this license.
	Tags []string `json:"tags"`
}

// GetTags returns ProductSubscriptionStateActiveLicenseProductLicenseInfo.Tags, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionStateActiveLicenseProductLicenseInfo) GetTags() []string { return v.Tags }

// ProductSubscriptionStateLlmProxyAccessLLMProxyAccess includes the requested fields of the GraphQL type LLMProxyAccess.
// The GraphQL type's documentation follows.
//
// LLM-proxy access granted to a subscription.
// FOR INTERNAL USE ONLY.
type ProductSubscriptionStateLlmProxyAccessLLMProxyAccess struct {
	LLMProxyAccessFields `json:"-"`
}

// GetEnabled returns ProductSubscriptionStateLlmProxyAccessLLMProxyAccess.Enabled, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionStateLlmProxyAccessLLMProxyAccess) GetEnabled() bool {
	return v.LLMProxyAccessFields.Enabled
}

// GetRateLimit returns ProductSubscriptionStateLlmProxyAccessLLMProxyAccess.RateLimit, and is useful for accessing the field via an interface.
func (v *ProductSubscriptionStateLlmProxyAccessLLMProxyAccess) GetRateLimit() *LLMProxyAccessFieldsRateLimitLLMProxyRateLimit {
	return v.LLMProxyAccessFields.RateLimit
}

func (v *ProductSubscriptionStateLlmProxyAccessLLMProxyAccess) UnmarshalJSON(b []byte) error {

	if string(b) == "null" {
		return nil
	}

	var firstPass struct {
		*ProductSubscriptionStateLlmProxyAccessLLMProxyAccess
		graphql.NoUnmarshalJSON
	}
	firstPass.ProductSubscriptionStateLlmProxyAccessLLMProxyAccess = v

	err := json.Unmarshal(b, &firstPass)
	if err != nil {
		return err
	}

	err = json.Unmarshal(
		b, &v.LLMProxyAccessFields)
	if err != nil {
		return err
	}
	return nil
}

type __premarshalProductSubscriptionStateLlmProxyAccessLLMProxyAccess struct {
	Enabled bool `json:"enabled"`

	RateLimit *LLMProxyAccessFieldsRateLimitLLMProxyRateLimit `json:"rateLimit"`
}

func (v *ProductSubscriptionStateLlmProxyAccessLLMProxyAccess) MarshalJSON() ([]byte, error) {
	premarshaled, err := v.__premarshalJSON()
	if err != nil {
		return nil, err
	}
	return json.Marshal(premarshaled)
}

func (v *ProductSubscriptionStateLlmProxyAccessLLMProxyAccess) __premarshalJSON() (*__premarshalProductSubscriptionStateLlmProxyAccessLLMProxyAccess, error) {
	var retval __premarshalProductSubscriptionStateLlmProxyAccessLLMProxyAccess

	retval.Enabled = v.LLMProxyAccessFields.Enabled
	retval.RateLimit = v.LLMProxyAccessFields.RateLimit
	return &retval, nil
}

// __CheckAccessTokenInput is used internally by genqlient
type __CheckAccessTokenInput struct {
	Token string `json:"token"`
}

// GetToken returns __CheckAccessTokenInput.Token, and is useful for accessing the field via an interface.
func (v *__CheckAccessTokenInput) GetToken() string { return v.Token }

// CheckAccessToken returns traits of the product subscription associated with
// the given access token.
func CheckAccessToken(
	ctx context.Context,
	client graphql.Client,
	token string,
) (*CheckAccessTokenResponse, error) {
	req := &graphql.Request{
		OpName: "CheckAccessToken",
		Query: `
query CheckAccessToken ($token: String!) {
	dotcom {
		productSubscriptionByAccessToken(accessToken: $token) {
			... ProductSubscriptionState
		}
	}
}
fragment ProductSubscriptionState on ProductSubscription {
	id
	uuid
	isArchived
	llmProxyAccess {
		... LLMProxyAccessFields
	}
	activeLicense {
		info {
			tags
		}
	}
}
fragment LLMProxyAccessFields on LLMProxyAccess {
	enabled
	rateLimit {
		source
		limit
		intervalSeconds
	}
}
`,
		Variables: &__CheckAccessTokenInput{
			Token: token,
		},
	}
	var err error

	var data CheckAccessTokenResponse
	resp := &graphql.Response{Data: &data}

	err = client.MakeRequest(
		ctx,
		req,
		resp,
	)

	return &data, err
}

func ListProductSubscriptions(
	ctx context.Context,
	client graphql.Client,
) (*ListProductSubscriptionsResponse, error) {
	req := &graphql.Request{
		OpName: "ListProductSubscriptions",
		Query: `
query ListProductSubscriptions {
	dotcom {
		productSubscriptions {
			totalCount
			pageInfo {
				endCursor
				hasNextPage
			}
			nodes {
				... ListProductSubscriptionFields
			}
		}
	}
}
fragment ListProductSubscriptionFields on ProductSubscription {
	... ProductSubscriptionState
	sourcegraphAccessTokens
}
fragment ProductSubscriptionState on ProductSubscription {
	id
	uuid
	isArchived
	llmProxyAccess {
		... LLMProxyAccessFields
	}
	activeLicense {
		info {
			tags
		}
	}
}
fragment LLMProxyAccessFields on LLMProxyAccess {
	enabled
	rateLimit {
		source
		limit
		intervalSeconds
	}
}
`,
	}
	var err error

	var data ListProductSubscriptionsResponse
	resp := &graphql.Response{Data: &data}

	err = client.MakeRequest(
		ctx,
		req,
		resp,
	)

	return &data, err
}
