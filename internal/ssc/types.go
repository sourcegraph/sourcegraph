package ssc

type BillingInterval string

const (
	BillingIntervalDaily   BillingInterval = "daily"
	BillingIntervalMonthly BillingInterval = "monthly"
	BillingIntervalYearly  BillingInterval = "yearly"
)

type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "ACTIVE"
	SubscriptionStatusPastDue  SubscriptionStatus = "PAST_DUE"
	SubscriptionStatusUnpaid   SubscriptionStatus = "UNPAID"
	SubscriptionStatusCanceled SubscriptionStatus = "CANCELED"
	SubscriptionStatusTrialing SubscriptionStatus = "TRIALING"
	SubscriptionStatusOther    SubscriptionStatus = "OTHER"
)

type Subscription struct {
	// Status is the current status of the subscription, e.g. "active" or "canceled".
	StatusRaw       string             `json:"status"`
	Status          SubscriptionStatus `json:"-"`
	BillingInterval BillingInterval    `json:"billingInterval"`

	// CancelAtPeriodEnd flags whether or not a subscription will automatically cancel at the end
	// of the current billing cycle, or if it will renew.
	CancelAtPeriodEnd bool `json:"cancelAtPeriodEnd"`

	// Billing cycle anchors are times represented as an ISO-8601 string.
	// e.g. "2024-01-17T13:18:05−07:00"
	CurrentPeriodStart string `json:"currentPeriodStart"`
	CurrentPeriodEnd   string `json:"currentPeriodEnd"`
}
