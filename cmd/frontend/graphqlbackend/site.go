package graphqlbackend

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sourcegraph/sourcegraph/schema"

	"github.com/google/go-cmp/cmp"
	"github.com/graph-gophers/graphql-go"
	"github.com/graph-gophers/graphql-go/relay"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/auth"
	"github.com/sourcegraph/sourcegraph/internal/cloud"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/migration"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/cliutil"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/drift"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/multiversion"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/runner"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/schemas"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/featureflag"
	"github.com/sourcegraph/sourcegraph/internal/gqlutil"
	"github.com/sourcegraph/sourcegraph/internal/insights"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/siteid"
	"github.com/sourcegraph/sourcegraph/internal/updatecheck"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/internal/version/upgradestore"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
	"github.com/sourcegraph/sourcegraph/lib/pointers"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
)

const singletonSiteGQLID = "site"

func (r *schemaResolver) siteByGQLID(_ context.Context, id graphql.ID) (Node, error) {
	siteGQLID, err := unmarshalSiteGQLID(id)
	if err != nil {
		return nil, err
	}
	if siteGQLID != singletonSiteGQLID {
		return nil, errors.Errorf("site not found: %q", siteGQLID)
	}
	return NewSiteResolver(r.logger, r.db), nil
}

func marshalSiteGQLID(siteID string) graphql.ID { return relay.MarshalID("Site", siteID) }

// SiteGQLID is the GraphQL ID of the Sourcegraph site. It is a constant across all Sourcegraph
// instances.
func SiteGQLID() graphql.ID { return (&siteResolver{gqlID: singletonSiteGQLID}).ID() }

func unmarshalSiteGQLID(id graphql.ID) (siteID string, err error) {
	err = relay.UnmarshalSpec(id, &siteID)
	return
}

func (r *schemaResolver) Site() *siteResolver {
	return NewSiteResolver(r.logger, r.db)
}

func NewSiteResolver(logger log.Logger, db database.DB) *siteResolver {
	return &siteResolver{
		logger: logger,
		db:     db,
		gqlID:  singletonSiteGQLID,
	}
}

type siteResolver struct {
	logger log.Logger
	db     database.DB
	gqlID  string // == singletonSiteGQLID, not the site ID
}

func (r *siteResolver) ID() graphql.ID { return marshalSiteGQLID(r.gqlID) }

func (r *siteResolver) SiteID() string { return siteid.Get(r.db) }

type SiteConfigurationArgs struct {
	ReturnSafeConfigsOnly *bool
}

func (r *siteResolver) Configuration(ctx context.Context, args *SiteConfigurationArgs) (*siteConfigurationResolver, error) {
	var returnSafeConfigsOnly = pointers.Deref(args.ReturnSafeConfigsOnly, false)

	// 🚨 SECURITY: The site configuration contains secret tokens and credentials,
	// so only admins may view it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		// returnSafeConfigsOnly determines whether to return a redacted version of the
		// site configuration that removes sensitive information. If true, returns a
		// siteConfigurationResolver that will return the redacted configuration. If
		// false, returns an error.
		//
		// The only way a non-admin can access this field is when `returnSafeConfigsOnly`
		// is set to true.
		if returnSafeConfigsOnly {
			if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {

				// Log an event when site config is viewed by non-admin user.
				if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameSiteConfigRedactedViewed, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", nil); err != nil {
					r.logger.Warn("Error logging security event", log.Error(err))
				}
			}
			return &siteConfigurationResolver{db: r.db, returnSafeConfigsOnly: returnSafeConfigsOnly}, nil
		}
		return nil, err
	}
	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {

		// Log an event when site config is viewed by admin user.
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameSiteConfigViewed, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", nil); err != nil {
			r.logger.Warn("Error logging security event", log.Error(err))
		}
	}
	return &siteConfigurationResolver{db: r.db, returnSafeConfigsOnly: returnSafeConfigsOnly}, nil
}

func (r *siteResolver) ViewerCanAdminister(ctx context.Context) (bool, error) {
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err == auth.ErrMustBeSiteAdmin || err == auth.ErrNotAuthenticated {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func (r *siteResolver) settingsSubject() api.SettingsSubject {
	return api.SettingsSubject{Site: true}
}

func (r *siteResolver) LatestSettings(ctx context.Context) (*settingsResolver, error) {
	settings, err := r.db.Settings().GetLatest(ctx, r.settingsSubject())
	if err != nil {
		return nil, err
	}
	if settings == nil {
		return nil, nil
	}
	return &settingsResolver{r.db, &settingsSubjectResolver{site: r}, settings, nil}, nil
}

func (r *siteResolver) SettingsCascade() *settingsCascade {
	return &settingsCascade{db: r.db, subject: &settingsSubjectResolver{site: r}}
}

func (r *siteResolver) ConfigurationCascade() *settingsCascade { return r.SettingsCascade() }

func (r *siteResolver) SettingsURL() *string { return strptr("/site-admin/global-settings") }

func (r *siteResolver) CanReloadSite(ctx context.Context) bool {
	err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db)
	return canReloadSite && err == nil
}

func (r *siteResolver) BuildVersion() string { return version.Version() }

func (r *siteResolver) ProductVersion() string { return version.Version() }

func (r *siteResolver) HasCodeIntelligence() bool {
	// BACKCOMPAT: Always return true.
	return true
}

func (r *siteResolver) ProductSubscription() *productSubscriptionStatus {
	return &productSubscriptionStatus{}
}

func (r *siteResolver) AllowSiteSettingsEdits() bool {
	return canUpdateSiteConfiguration()
}

type siteConfigurationResolver struct {
	db                    database.DB
	returnSafeConfigsOnly bool
}

func (r *siteConfigurationResolver) ID(ctx context.Context) (int32, error) {
	// 🚨 SECURITY: The site configuration contains secret tokens and credentials,
	// so only admins may view it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return 0, err
	}
	config, err := r.db.Conf().SiteGetLatest(ctx)
	if err != nil {
		return 0, err
	}
	return config.ID, nil
}

func (r *siteConfigurationResolver) EffectiveContents(ctx context.Context) (JSONCString, error) {
	// returnSafeConfigsOnly determines whether to return a redacted version of the
	// site configuration that removes sensitive information. If true, uses
	// conf.ReturnSafeConfigs to return a redacted configuration. If false, checks if the
	// current user is a site admin and returns the full unredacted configuration.
	if r.returnSafeConfigsOnly {
		safeConfig, err := conf.ReturnSafeConfigs(conf.Raw())
		return JSONCString(safeConfig.Site), err
	}
	// 🚨 SECURITY: The site configuration contains secret tokens and credentials,
	// so only admins may view it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return "", err
	}
	siteConfig, err := conf.RedactSecrets(conf.Raw())
	return JSONCString(siteConfig.Site), err
}

type licenseInfoResolver struct {
	tags      []string
	userCount int32
	expiresAt gqlutil.DateTime
}

func (r *licenseInfoResolver) Tags() []string   { return r.tags }
func (r *licenseInfoResolver) UserCount() int32 { return r.userCount }

func (r *licenseInfoResolver) ExpiresAt() gqlutil.DateTime {
	return r.expiresAt
}

func (r *siteConfigurationResolver) LicenseInfo(ctx context.Context) (*licenseInfoResolver, error) {
	// 🚨 SECURITY: Only site admins can view license information.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	license, err := licensing.GetConfiguredProductLicenseInfo()
	if err != nil {
		return nil, err
	}

	return &licenseInfoResolver{
		tags:      license.Tags,
		userCount: int32(license.UserCount),
		expiresAt: gqlutil.DateTime{Time: license.ExpiresAt},
	}, nil
}

func (r *siteConfigurationResolver) ValidationMessages(ctx context.Context) ([]string, error) {
	// 🚨 SECURITY: The site configuration contains secret tokens and credentials,
	// so only admins may view it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}
	contents, err := r.EffectiveContents(ctx)
	if err != nil {
		return nil, err
	}
	return conf.ValidateSite(string(contents))
}

func (r *siteConfigurationResolver) History(ctx context.Context, args *graphqlutil.ConnectionResolverArgs) (*graphqlutil.ConnectionResolver[*SiteConfigurationChangeResolver], error) {
	// 🚨 SECURITY: The site configuration contains secret tokens and credentials,
	// so only admins may view the history.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	connectionStore := SiteConfigurationChangeConnectionStore{db: r.db}

	return graphqlutil.NewConnectionResolver[*SiteConfigurationChangeResolver](
		&connectionStore,
		args,
		nil,
	)
}

func (r *schemaResolver) UpdateSiteConfiguration(ctx context.Context, args *struct {
	LastID int32
	Input  string
},
) (bool, error) {
	// 🚨 SECURITY: The site configuration contains secret tokens and credentials,
	// so only admins may view it.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return false, err
	}
	if !canUpdateSiteConfiguration() {
		return false, errors.New("updating site configuration not allowed when using SITE_CONFIG_FILE")
	}
	if strings.TrimSpace(args.Input) == "" {
		return false, errors.Errorf("blank site configuration is invalid (you can clear the site configuration by entering an empty JSON object: {})")
	}

	prev := conf.Raw()

	// 🚨 SECURITY: The site configuration contains secret tokens and credentials,
	// so take the redacted version for logging purposes.
	prevSCredacted, _ := conf.RedactSecrets(prev)
	arg := struct {
		PrevConfig string `json:"prev_config"`
		NewConfig  string `json:"new_config"`
	}{
		PrevConfig: prevSCredacted.Site,
		NewConfig:  args.Input,
	}

	unredacted, err := conf.UnredactSecrets(args.Input, prev)
	if err != nil {
		return false, errors.Errorf("error unredacting secrets: %s", err)
	}

	cloudSiteConfig := cloud.SiteConfig()
	if cloudSiteConfig.SiteConfigAllowlistEnabled() && !actor.FromContext(ctx).SourcegraphOperator {
		if p, ok := allowEdit(prev.Site, unredacted, cloudSiteConfig.SiteConfigAllowlist.Paths); !ok {
			return false, cloudSiteConfig.SiteConfigAllowlistOnError(p)
		}
	}

	prev.Site = unredacted

	server := globals.ConfigurationServerFrontendOnly
	if err := server.Write(ctx, prev, args.LastID, actor.FromContext(ctx).UID); err != nil {
		return false, err
	}

	if featureflag.FromContext(ctx).GetBoolOr("auditlog-expansion", false) {

		// Log an event when site config is updated
		if err := r.db.SecurityEventLogs().LogSecurityEvent(ctx, database.SecurityEventNameSiteConfigUpdated, "", uint32(actor.FromContext(ctx).UID), "", "BACKEND", arg); err != nil {
			r.logger.Warn("Error logging security event", log.Error(err))
		}
	}
	return server.NeedServerRestart(), nil
}

var siteConfigAllowEdits, _ = strconv.ParseBool(env.Get("SITE_CONFIG_ALLOW_EDITS", "false", "When SITE_CONFIG_FILE is in use, allow edits in the application to be made which will be overwritten on next process restart"))

func canUpdateSiteConfiguration() bool {
	return os.Getenv("SITE_CONFIG_FILE") == "" || siteConfigAllowEdits
}

func (r *siteResolver) UpgradeReadiness(ctx context.Context) (*upgradeReadinessResolver, error) {
	// 🚨 SECURITY: Only site admins may view upgrade readiness information.
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return nil, err
	}

	return &upgradeReadinessResolver{
		logger: r.logger.Scoped("upgradeReadiness"),
		db:     r.db,
	}, nil
}

type upgradeReadinessResolver struct {
	logger log.Logger
	db     database.DB

	initOnce    sync.Once
	initErr     error
	runner      *runner.Runner
	version     string
	schemaNames []string
}

var devSchemaFactory = schemas.NewExpectedSchemaFactory(
	"Local file",
	[]schemas.NamedRegexp{{Regexp: lazyregexp.New(`^(dev|0\.0\.0\+dev)$`)}},
	func(filename, _ string) string { return filename },
	schemas.ReadSchemaFromFile,
)

var schemaFactories = append(
	schemas.DefaultSchemaFactories,
	// Special schema factory for dev environment.
	devSchemaFactory,
)

var insidersVersionPattern = lazyregexp.New(`^[\w-]+_\d{4}-\d{2}-\d{2}_\d+\.\d+-(\w+)$`)

func (r *upgradeReadinessResolver) init(ctx context.Context) (_ *runner.Runner, version string, schemaNames []string, _ error) {
	r.initOnce.Do(func() {
		r.runner, r.version, r.schemaNames, r.initErr = func() (*runner.Runner, string, []string, error) {
			schemaNames := []string{schemas.Frontend.Name, schemas.CodeIntel.Name}
			schemaList := []*schemas.Schema{schemas.Frontend, schemas.CodeIntel}
			if insights.IsEnabled() {
				schemaNames = append(schemaNames, schemas.CodeInsights.Name)
				schemaList = append(schemaList, schemas.CodeInsights)
			}
			observationCtx := observation.NewContext(r.logger)
			runner, err := migration.NewRunnerWithSchemas(observationCtx, output.OutputFromLogger(r.logger), "frontend-upgradereadiness", schemaNames, schemaList)
			if err != nil {
				return nil, "", nil, errors.Wrap(err, "new runner")
			}

			versionStr, ok, err := cliutil.GetRawServiceVersion(ctx, runner)
			if err != nil {
				return nil, "", nil, errors.Wrap(err, "get service version")
			} else if !ok {
				return nil, "", nil, errors.New("invalid service version")
			}

			// Return abbreviated commit hash from insiders version
			if matches := insidersVersionPattern.FindStringSubmatch(versionStr); len(matches) > 0 {
				return runner, matches[1], schemaNames, nil
			}

			v, patch, ok := oobmigration.NewVersionAndPatchFromString(versionStr)
			if !ok {
				return nil, "", nil, errors.Newf("cannot parse version: %q - expected [v]X.Y[.Z]", versionStr)
			}

			if v.Dev {
				return runner, "0.0.0+dev", schemaNames, nil
			}

			return runner, v.GitTagWithPatch(patch), schemaNames, nil
		}()
	})

	return r.runner, r.version, r.schemaNames, r.initErr
}

type schemaDriftResolver struct {
	summary drift.Summary
}

func (r *schemaDriftResolver) Name() string {
	return r.summary.Name()
}

func (r *schemaDriftResolver) Problem() string {
	return r.summary.Problem()
}

func (r *schemaDriftResolver) Solution() string {
	return r.summary.Solution()
}

func (r *schemaDriftResolver) Diff() *string {
	if a, b, ok := r.summary.Diff(); ok {
		v := cmp.Diff(a, b)
		return &v
	}

	return nil
}

func (r *schemaDriftResolver) Statements() *[]string {
	if statements, ok := r.summary.Statements(); ok {
		return &statements
	}

	return nil
}

func (r *schemaDriftResolver) URLHint() *string {
	if urlHint, ok := r.summary.URLHint(); ok {
		return &urlHint
	}

	return nil
}

func (r *upgradeReadinessResolver) SchemaDrift(ctx context.Context) ([]*schemaDriftResolver, error) {
	runner, version, schemaNames, err := r.init(ctx)
	if err != nil {
		return nil, err
	}
	r.logger.Debug("schema drift", log.String("version", version))

	var resolvers []*schemaDriftResolver
	for _, schemaName := range schemaNames {
		store, err := runner.Store(ctx, schemaName)
		if err != nil {
			return nil, errors.Wrap(err, "get migration store")
		}
		schemaDescriptions, err := store.Describe(ctx)
		if err != nil {
			return nil, err
		}
		schema := schemaDescriptions["public"]

		var buf bytes.Buffer
		driftOut := output.NewOutput(&buf, output.OutputOpts{})

		expectedSchema, err := multiversion.FetchExpectedSchema(ctx, schemaName, version, driftOut, schemaFactories)
		if err != nil {
			return nil, err
		}

		for _, summary := range drift.CompareSchemaDescriptions(schemaName, version, multiversion.Canonicalize(schema), multiversion.Canonicalize(expectedSchema)) {
			resolvers = append(resolvers, &schemaDriftResolver{
				summary: summary,
			})
		}
	}

	return resolvers, nil
}

// isRequiredOutOfBandMigration returns true if an OOB migration will be deprecated in the latest version and has not progressed to completion.
func isRequiredOutOfBandMigration(currentVersion, latestVersion oobmigration.Version, m oobmigration.Migration) bool {
	// If current version is dev, no migrations are required.
	if currentVersion.Dev {
		return false
	}

	// If the migration is not marked as deprecated, or was deprecated before the current product version, it is not required.
	if m.Deprecated == nil || oobmigration.CompareVersions(*m.Deprecated, currentVersion) == oobmigration.VersionOrderBefore {
		return false
	}

	// The version the migration is marked as deprecated is not after the latest release version, and is incomplete.
	return oobmigration.CompareVersions(*m.Deprecated, latestVersion) != oobmigration.VersionOrderAfter && m.Progress < 1
}

func (r *upgradeReadinessResolver) RequiredOutOfBandMigrations(ctx context.Context) ([]*outOfBandMigrationResolver, error) {
	// Get the current version by initializing the resolver
	_, version, _, err := r.init(ctx)
	if err != nil {
		return nil, err
	}
	currentVersion, _, ok := oobmigration.NewVersionAndPatchFromString(version)
	if !ok {
		return nil, errors.Errorf("invalid current version %s", r.version)
	}

	updateStatus := updatecheck.Last()
	if updateStatus == nil {
		return nil, errors.New("no latest update version available (reload in a few seconds)")
	}
	if !updateStatus.HasUpdate() {
		return nil, nil
	}

	// The latest sourcegraph version available, returned from the updateCheck
	latestVersion, _, ok := oobmigration.NewVersionAndPatchFromString(updateStatus.UpdateVersion)
	if !ok {
		return nil, errors.Errorf("invalid latest update version %q", updateStatus.UpdateVersion)
	}

	migrations, err := oobmigration.NewStoreWithDB(r.db).List(ctx)
	if err != nil {
		return nil, err
	}

	var requiredMigrations []*outOfBandMigrationResolver
	for _, m := range migrations {
		if isRequiredOutOfBandMigration(currentVersion, latestVersion, m) {
			requiredMigrations = append(requiredMigrations, &outOfBandMigrationResolver{m})
		}
	}
	return requiredMigrations, nil
}

// Return the enablement of auto upgrades
func (r *siteResolver) AutoUpgradeEnabled(ctx context.Context) (bool, error) {
	// 🚨 SECURITY: Only site admins can set auto_upgrade readiness
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return false, err
	}
	_, enabled, err := upgradestore.NewWith(r.db.Handle()).GetAutoUpgrade(ctx)
	if err != nil {
		return false, err
	}
	return enabled, nil
}

func (r *schemaResolver) SetAutoUpgrade(ctx context.Context, args *struct {
	Enable bool
},
) (*EmptyResponse, error) {
	// 🚨 SECURITY: Only site admins can set auto_upgrade readiness
	if err := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db); err != nil {
		return &EmptyResponse{}, err
	}
	err := upgradestore.NewWith(r.db.Handle()).SetAutoUpgrade(ctx, args.Enable)
	return &EmptyResponse{}, err
}

func (r *siteResolver) PerUserCompletionsQuota() *int32 {
	c := conf.GetCompletionsConfig(conf.Get().SiteConfig())
	if c != nil && c.PerUserDailyLimit > 0 {
		i := int32(c.PerUserDailyLimit)
		return &i
	}
	return nil
}

func (r *siteResolver) PerUserCodeCompletionsQuota() *int32 {
	c := conf.GetCompletionsConfig(conf.Get().SiteConfig())
	if c != nil && c.PerUserCodeCompletionsDailyLimit > 0 {
		i := int32(c.PerUserCodeCompletionsDailyLimit)
		return &i
	}
	return nil
}

func (r *siteResolver) RequiresVerifiedEmailForCody(ctx context.Context) bool {
	// We only require this on dotcom
	if !dotcom.SourcegraphDotComMode() {
		return false
	}

	isAdmin := auth.CheckCurrentUserIsSiteAdmin(ctx, r.db) == nil
	return !isAdmin
}

func (r *siteResolver) IsCodyEnabled(ctx context.Context) bool {
	enabled, _ := cody.IsCodyEnabled(ctx, r.db)
	return enabled
}

func (r *siteResolver) CodyLLMConfiguration(ctx context.Context) *codyLLMConfigurationResolver {
	c := conf.GetCompletionsConfig(conf.Get().SiteConfig())
	if c == nil {
		return nil
	}

	return &codyLLMConfigurationResolver{config: c}
}

func (r *siteResolver) CodyConfigFeatures(ctx context.Context) *codyConfigFeaturesResolver {
	c := conf.GetConfigFeatures(conf.Get().SiteConfig())
	if c == nil {
		return nil
	}
	return &codyConfigFeaturesResolver{config: c}
}

type codyConfigFeaturesResolver struct {
	config *conftypes.ConfigFeatures
}

func (c *codyConfigFeaturesResolver) Chat() bool         { return c.config.Chat }
func (c *codyConfigFeaturesResolver) AutoComplete() bool { return c.config.AutoComplete }
func (c *codyConfigFeaturesResolver) Commands() bool     { return c.config.Commands }
func (c *codyConfigFeaturesResolver) Attribution() bool  { return c.config.Attribution }

type codyLLMConfigurationResolver struct {
	config *conftypes.CompletionsConfig
}

func (c *codyLLMConfigurationResolver) ChatModel() string { return c.config.ChatModel }
func (c *codyLLMConfigurationResolver) ChatModelMaxTokens() *int32 {
	if c.config.ChatModelMaxTokens != 0 {
		max := int32(c.config.ChatModelMaxTokens)
		return &max
	}
	return nil
}
func (c *codyLLMConfigurationResolver) SmartContextWindow() string {
	if c.config.SmartContextWindow == "disabled" {
		return "disabled"
	}
	return "enabled"
}
func (c *codyLLMConfigurationResolver) DisableClientConfigAPI() bool {
	return c.config.DisableClientConfigAPI
}

func (c *codyLLMConfigurationResolver) FastChatModel() string { return c.config.FastChatModel }
func (c *codyLLMConfigurationResolver) FastChatModelMaxTokens() *int32 {
	if c.config.FastChatModelMaxTokens != 0 {
		max := int32(c.config.FastChatModelMaxTokens)
		return &max
	}
	return nil
}

func (c *codyLLMConfigurationResolver) Provider() string        { return string(c.config.Provider) }
func (c *codyLLMConfigurationResolver) CompletionModel() string { return c.config.CompletionModel }
func (c *codyLLMConfigurationResolver) CompletionModelMaxTokens() *int32 {
	if c.config.CompletionModelMaxTokens != 0 {
		max := int32(c.config.CompletionModelMaxTokens)
		return &max
	}
	return nil
}

type CodyContextFiltersArgs struct {
	Version string
}

type codyContextFiltersResolver struct {
	ccf *schema.CodyContextFilters
}

func (c *codyContextFiltersResolver) Raw() *JSONValue {
	if c.ccf == nil {
		return nil
	}
	return &JSONValue{c.ccf}
}

func (r *siteResolver) CodyContextFilters(_ context.Context, _ *CodyContextFiltersArgs) *codyContextFiltersResolver {
	return &codyContextFiltersResolver{ccf: conf.Get().SiteConfig().CodyContextFilters}
}

func allowEdit(before, after string, allowlist []string) ([]string, bool) {
	var notAllowed []string
	changes := conf.Diff(before, after)
	for key := range changes {
		for _, p := range allowlist {
			if key != p {
				notAllowed = append(notAllowed, key)
			}
		}
	}
	return notAllowed, len(notAllowed) == 0
}
