package codyaccess

import "github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/database/subscriptions"

type CodyGatewayAccess struct {
	// ⚠️ DO NOT USE: This field is only used for creating foreign key constraint.
	Subscription *subscriptions.Subscription `gorm:"foreignKey:SubscriptionID"`

	// SubscriptionID is the internal unprefixed UUID of the related subscription.
	SubscriptionID string `gorm:"type:uuid;not null;unique"`

	// Whether or not a subscription has Cody Gateway access enabled.
	Enabled bool `gorm:"not null"`

	// chat_completions_rate_limit
	ChatCompletionsRateLimit                int64 `gorm:"type:bigint;not null"`
	ChatCompletionsRateLimitIntervalSeconds int   `gorm:"not null"`

	// code_completions_rate_limit
	CodeCompletionsRateLimit                int64 `gorm:"type:bigint;not null"`
	CodeCompletionsRateLimitIntervalSeconds int   `gorm:"not null"`

	// embeddings_rate_limit
	EmbeddingsRateLimit                int64 `gorm:"type:bigint;not null"`
	EmbeddingsRateLimitIntervalSeconds int   `gorm:"not null"`
}

func (s *CodyGatewayAccess) TableName() string {
	return "enterprise_portal_cody_gateway_access"
}
