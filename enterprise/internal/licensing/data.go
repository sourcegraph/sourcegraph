package licensing

// The list of plans.
const (
	// oldEnterpriseStarter is the old "Enterprise Starter" plan.
	oldEnterpriseStarter Plan = "old-starter-0"
	// oldEnterprise is the old "Enterprise" plan.
	oldEnterprise Plan = "old-enterprise-0"

	// team is the "Team" plan.
	team Plan = "team-0"
	// enterprise is the "Enterprise" plan.
	enterprise Plan = "enterprise-0"
)

var allPlans = []Plan{
	oldEnterpriseStarter,
	oldEnterprise,
	team,
	enterprise,
}

// The list of features. For each feature, add a new const here and the checking logic in
// isFeatureEnabled.
const (
	// FeatureACLs is whether ACLs may be used, such as GitHub, GitLab or Bitbucket Server repository
	// permissions and integration with GitHub, GitLab or Bitbucket Server for user authentication.
	FeatureACLs Feature = "acls"

	// FeatureExtensionRegistry is whether publishing extensions to this Sourcegraph instance has been
	// purchased. If not, then extensions must be published to Sourcegraph.com. All instances may use
	// extensions published to Sourcegraph.com.
	FeatureExtensionRegistry Feature = "private-extension-registry"

	// FeatureRemoteExtensionsAllowDisallow is whether explicitly specify a list of allowed remote
	// extensions and prevent any other remote extensions from being used has been purchased. It
	// does not apply to locally published extensions.
	FeatureRemoteExtensionsAllowDisallow Feature = "remote-extensions-allow-disallow"

	// FeatureBranding is whether custom branding of this Sourcegraph instance has been purchased.
	FeatureBranding Feature = "branding"

	// FeatureCampaigns is whether campaigns (now: batch changes) on this Sourcegraph instance has been purchased.
	//
	// DEPRECATED: See FeatureBatchChanges.
	FeatureCampaigns Feature = "campaigns"

	// FeatureBatchChanges is whether Batch Changes on this Sourcegraph instance has been purchased.
	FeatureBatchChanges Feature = "batch-changes"

	// FeatureMonitoring is whether monitoring on this Sourcegraph instance has been purchased.
	FeatureMonitoring Feature = "monitoring"

	// FeatureBackupAndRestore is whether builtin backup and restore on this Sourcegraph instance
	// has been purchased.
	FeatureBackupAndRestore Feature = "backup-and-restore"
)

// planFeatures defines the features that are enabled for each plan.
var planFeatures = map[Plan][]Feature{
	oldEnterpriseStarter: {},
	oldEnterprise: {
		FeatureACLs,
		FeatureExtensionRegistry,
		FeatureRemoteExtensionsAllowDisallow,
		FeatureBranding,
		FeatureCampaigns,
		FeatureBatchChanges,
		FeatureMonitoring,
		FeatureBackupAndRestore,
	},
	team:       {},
	enterprise: {},
}

// NoLicenseMaximumExternalServiceCount is the maximum number of external services that the
// instance supports when running without a license.
const NoLicenseMaximumExternalServiceCount = 1
