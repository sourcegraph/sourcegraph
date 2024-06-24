package licensing

// The list of plans.
const (
	// PlanOldEnterprise is the old "Enterprise" plan.
	// Deprecated: PlanOldEnterprise has been deprecated and we will stop issuing licenses for it.
	PlanOldEnterprise Plan = "old-enterprise-0"

	// PlanTeam0 is the "Team" plan pre-4.0.
	// Deprecated: PlanTeam0 has been deprecated and we will stop issuing licenses for it.
	PlanTeam0 Plan = "team-0"

	// PlanEnterprise0 is the "Enterprise" plan pre-4.0.
	// This represents the "System 1" in our feature matrix.
	PlanEnterprise0 Plan = "enterprise-0"

	// PlanEnterprise1 is the "Enterprise" plan for 4.0.
	// This represents the "System 2" in our feature matrix.
	PlanEnterprise1 Plan = "enterprise-1"

	// PlanFree0 is the default plan if no license key is set before 4.5.
	// Deprecated: PlanFree0 has been deprecated and we will stop issuing licenses for it.
	PlanFree0 Plan = "free-0"

	// PlanFree1 is the default plan if no license key is set from 4.5 onwards.
	// Do not issue licenses for this plan directly, Sourcegraph auto-applies this
	// plan.
	PlanFree1 Plan = "free-1"

	// PlanAirGappedEnterprise is the same PlanEnterprise1 but with FeatureAllowAirGapped, and works starting from 5.1.
	// Deprecated: PlanAirGappedEnterprise has been deprecated and we will stop issuing licenses for it.
	PlanAirGappedEnterprise Plan = "enterprise-air-gap-0"

	// PlanCodyEnterprise is the cody-only plan.
	PlanCodyEnterprise Plan = "cody-enterprise-0"

	// PlanCodeIntelligencePlatform is the full package plan. It includes code search and cody.
	PlanCodeIntelligencePlatform Plan = "code-ai-enterprise-0"
)

var AllPlans = []Plan{
	// Deprecated plans:
	PlanOldEnterprise,
	PlanTeam0,
	PlanFree0,
	PlanAirGappedEnterprise,

	// Current plans:
	PlanFree1,
	PlanEnterprise0,
	PlanEnterprise1,
	PlanCodyEnterprise,
	PlanCodeIntelligencePlatform,
}

// The list of features. For each feature, add a new const here and the checking logic in
// isFeatureEnabled.
const (
	// FeatureSSO is whether non-builtin authentication may be used, such as GitHub
	// OAuth, GitLab OAuth, SAML, and OpenID.
	FeatureSSO BasicFeature = "sso"

	// FeatureACLs is whether the Background Permissions Syncing may be be used for
	// setting repository permissions.
	FeatureACLs BasicFeature = "acls"

	// FeatureExplicitPermissionsAPI is whether the Explicit Permissions API may be be used for
	// setting repository permissions.
	FeatureExplicitPermissionsAPI BasicFeature = "explicit-permissions-api"

	// FeatureCodeInsights is whether Code Insights on this Sourcegraph instance has been purchased.
	FeatureCodeInsights BasicFeature = "code-insights"

	// FeatureSCIM is whether SCIM User Management has been purchased on this instance.
	FeatureSCIM BasicFeature = "SCIM"

	// FeatureCody is whether or not Cody has been purchased on this instance.
	FeatureCody BasicFeature = "cody"

	// FeatureAllowAirGapped is whether or not air gapped mode is allowed on this instance.
	FeatureAllowAirGapped BasicFeature = "allow-air-gapped"

	// FeatureCodeMonitors is whether code monitors is allowed on this Sourcegraph instance.
	FeatureCodeMonitors BasicFeature = "code-monitors"

	// FeatureNotebooks is whether the notebooks feature is allowed on this Sourcegraph instance.
	FeatureNotebooks BasicFeature = "notebooks"

	// FeatureCodeSearch is whether the code search feature suite is allowed on this Sourcegraph instance.
	FeatureCodeSearch BasicFeature = "code-search"
)

type PlanDetails struct {
	DisplayName string
	Deprecated  bool
	Features    []Feature
}

// planDetails defines the features that are enabled for each plan.
var planDetails = map[Plan]PlanDetails{
	PlanOldEnterprise: {
		DisplayName: "Sourcegraph Enterprise",
		Deprecated:  true,
		Features: []Feature{
			FeatureSSO,
			FeatureACLs,
			FeatureExplicitPermissionsAPI,
			&FeatureBatchChanges{Unrestricted: true},
			&FeaturePrivateRepositories{Unrestricted: true},
			FeatureCodeInsights,
			FeatureSCIM,
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
		},
	},
	PlanTeam0: {
		DisplayName: "Sourcegraph Team",
		Deprecated:  true,
		Features: []Feature{
			FeatureACLs,
			FeatureExplicitPermissionsAPI,
			FeatureSSO,
			&FeatureBatchChanges{MaxNumChangesets: 10},
			&FeaturePrivateRepositories{Unrestricted: true},
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
		},
	},
	PlanEnterprise0: {
		DisplayName: "Sourcegraph Enterprise",
		Features: []Feature{
			FeatureACLs,
			FeatureExplicitPermissionsAPI,
			FeatureSSO,
			&FeatureBatchChanges{MaxNumChangesets: 10},
			&FeaturePrivateRepositories{Unrestricted: true},
			FeatureSCIM,
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
		},
	},
	// The differences to enterprise-1 are:
	// - max 10 batch changes on enterprise-0 vs unlimited batch changes.
	// - No code insights on enterprise-0
	PlanEnterprise1: {
		DisplayName: "Code Search Enterprise",
		Features: []Feature{
			FeatureACLs,
			FeatureCodeInsights,
			&FeatureBatchChanges{Unrestricted: true},
			&FeaturePrivateRepositories{Unrestricted: true},
			FeatureExplicitPermissionsAPI,
			FeatureSSO,
			FeatureSCIM,
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
		},
	},
	PlanFree0: {
		DisplayName: "Sourcegraph Free",
		Deprecated:  true,
		Features: []Feature{
			FeatureSSO,
			&FeatureBatchChanges{MaxNumChangesets: 10},
			&FeaturePrivateRepositories{Unrestricted: true},
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
		},
	},
	PlanFree1: {
		DisplayName: "Sourcegraph Free",
		Features: []Feature{
			&FeatureBatchChanges{MaxNumChangesets: 10},
			&FeaturePrivateRepositories{MaxNumPrivateRepos: 1},
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
		},
	},
	PlanAirGappedEnterprise: {
		DisplayName: "Sourcegraph Enterprise",
		Deprecated:  true,
		Features: []Feature{
			FeatureACLs,
			FeatureCodeInsights,
			&FeatureBatchChanges{Unrestricted: true},
			&FeaturePrivateRepositories{Unrestricted: true},
			FeatureExplicitPermissionsAPI,
			FeatureSSO,
			FeatureSCIM,
			FeatureAllowAirGapped,
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
		},
	},
	PlanCodyEnterprise: {
		DisplayName: "Cody Enterprise",
		Features: []Feature{
			FeatureSSO,
			FeatureACLs,
			FeatureExplicitPermissionsAPI,
			FeatureSCIM,
			FeatureCody,
			&FeaturePrivateRepositories{Unrestricted: true},
		},
	},
	PlanCodeIntelligencePlatform: {
		DisplayName: "Code Intelligence Platform",
		Features: []Feature{
			FeatureSSO,
			FeatureACLs,
			FeatureExplicitPermissionsAPI,
			FeatureCodeInsights,
			FeatureSCIM,
			FeatureCody,
			FeatureCodeMonitors,
			FeatureNotebooks,
			FeatureCodeSearch,
			&FeaturePrivateRepositories{Unrestricted: true},
			&FeatureBatchChanges{Unrestricted: true},
		},
	},
}
