package conf

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf/confdefaults"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func init() {
	deployType := DeployType()
	if !IsValidDeployType(deployType) {
		log.Fatalf("The 'DEPLOY_TYPE' environment variable is invalid. Expected one of: %q, %q, %q, %q, %q. Got: %q", DeployKubernetes, DeployDockerCompose, DeployPureDocker, DeploySingleDocker, DeployDev, deployType)
	}

	confdefaults.Default = defaultConfigForDeployment()
}

func defaultConfigForDeployment() conftypes.RawUnified {
	deployType := DeployType()
	switch {
	case IsDev(deployType):
		return confdefaults.DevAndTesting
	case IsDeployTypeSingleDockerContainer(deployType):
		return confdefaults.DockerContainer
	case IsDeployTypeKubernetes(deployType), IsDeployTypeDockerCompose(deployType), IsDeployTypePureDocker(deployType):
		return confdefaults.KubernetesOrDockerComposeOrPureDocker
	default:
		panic("deploy type did not register default configuration")
	}
}

func AWSCodeCommitConfigs(ctx context.Context) ([]*schema.AWSCodeCommitConnection, error) {
	var config []*schema.AWSCodeCommitConnection
	if err := api.InternalClient.ExternalServiceConfigs(ctx, extsvc.KindAWSCodeCommit, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func BitbucketServerConfigs(ctx context.Context) ([]*schema.BitbucketServerConnection, error) {
	var config []*schema.BitbucketServerConnection
	if err := api.InternalClient.ExternalServiceConfigs(ctx, extsvc.KindBitbucketServer, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func GitHubConfigs(ctx context.Context) ([]*schema.GitHubConnection, error) {
	var config []*schema.GitHubConnection
	if err := api.InternalClient.ExternalServiceConfigs(ctx, extsvc.KindGitHub, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func GitLabConfigs(ctx context.Context) ([]*schema.GitLabConnection, error) {
	var config []*schema.GitLabConnection
	if err := api.InternalClient.ExternalServiceConfigs(ctx, extsvc.KindGitLab, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func GitoliteConfigs(ctx context.Context) ([]*schema.GitoliteConnection, error) {
	var config []*schema.GitoliteConnection
	if err := api.InternalClient.ExternalServiceConfigs(ctx, extsvc.KindGitolite, &config); err != nil {
		return nil, err
	}
	return config, nil
}

func PhabricatorConfigs(ctx context.Context) ([]*schema.PhabricatorConnection, error) {
	var config []*schema.PhabricatorConnection
	if err := api.InternalClient.ExternalServiceConfigs(ctx, extsvc.KindPhabricator, &config); err != nil {
		return nil, err
	}
	return config, nil
}

type AccessTokAllow string

const (
	AccessTokensNone  AccessTokAllow = "none"
	AccessTokensAll   AccessTokAllow = "all-users-create"
	AccessTokensAdmin AccessTokAllow = "site-admin-create"
)

// AccessTokensAllow returns whether access tokens are enabled, disabled, or restricted to creation by admin users.
func AccessTokensAllow() AccessTokAllow {
	cfg := Get().AuthAccessTokens
	if cfg == nil {
		return AccessTokensAll
	}
	switch cfg.Allow {
	case "":
		return AccessTokensAll
	case string(AccessTokensAll):
		return AccessTokensAll
	case string(AccessTokensNone):
		return AccessTokensNone
	case string(AccessTokensAdmin):
		return AccessTokensAdmin
	default:
		return AccessTokensNone
	}
}

// EmailVerificationRequired returns whether users must verify an email address before they
// can perform most actions on this site.
//
// It's false for sites that do not have an email sending API key set up.
func EmailVerificationRequired() bool {
	return Get().EmailSmtp != nil
}

// CanSendEmail returns whether the site can send emails (e.g., to reset a password or
// invite a user to an org).
//
// It's false for sites that do not have an email sending API key set up.
func CanSendEmail() bool {
	return Get().EmailSmtp != nil
}

// Deploy type constants. Any changes here should be reflected in the DeployType type declared in web/src/globals.d.ts:
// https://sourcegraph.com/search?q=r:github.com/sourcegraph/sourcegraph%24+%22type+DeployType%22
const (
	DeployKubernetes    = "kubernetes"
	DeploySingleDocker  = "docker-container"
	DeployDockerCompose = "docker-compose"
	DeployPureDocker    = "pure-docker"
	DeployDev           = "dev"
)

// DeployType tells the deployment type.
func DeployType() string {
	if e := os.Getenv("DEPLOY_TYPE"); e != "" {
		return e
	}
	// Default to Kubernetes cluster so that every Kubernetes
	// cluster deployment doesn't need to be configured with DEPLOY_TYPE.
	return DeployKubernetes
}

// IsDeployTypeKubernetes tells if the given deployment type is a Kubernetes
// cluster (and non-dev, not docker-compose, not pure-docker, and non-single Docker image).
func IsDeployTypeKubernetes(deployType string) bool {
	switch deployType {
	// includes older Kubernetes aliases for backwards compatibility
	case "k8s", "cluster", DeployKubernetes:
		return true
	}

	return false
}

// IsDeployTypeDockerCompose tells if the given deployment type is the Docker Compose
// deployment (and non-dev, not pure-docker, non-cluster, and non-single Docker image).
func IsDeployTypeDockerCompose(deployType string) bool {
	return deployType == DeployDockerCompose
}

// IsDeployTypePureDocker tells if the given deployment type is the pure Docker
// deployment (and non-dev, not docker-compose, non-cluster, and non-single Docker image).
func IsDeployTypePureDocker(deployType string) bool {
	return deployType == DeployPureDocker
}

// IsDeployTypeSingleDockerContainer tells if the given deployment type is Docker sourcegraph/server
// single-container (non-Kubernetes, not docker-compose, not pure-docker, non-cluster, non-dev).
func IsDeployTypeSingleDockerContainer(deployType string) bool {
	return deployType == DeploySingleDocker
}

// IsDev tells if the given deployment type is "dev".
func IsDev(deployType string) bool {
	return deployType == DeployDev
}

// IsValidDeployType returns true iff the given deployType is a Kubernetes deployment, a Docker Compose
// deployment, a pure Docker deployment, a Docker deployment, or a local development environment.
func IsValidDeployType(deployType string) bool {
	return IsDeployTypeKubernetes(deployType) ||
		IsDeployTypeDockerCompose(deployType) ||
		IsDeployTypePureDocker(deployType) ||
		IsDeployTypeSingleDockerContainer(deployType) ||
		IsDev(deployType)
}

// UpdateChannel tells the update channel. Default is "release".
func UpdateChannel() string {
	channel := Get().UpdateChannel
	if channel == "" {
		return "release"
	}
	return channel
}

// SearchIndexEnabled returns true if sourcegraph should index all
// repositories for text search. If the configuration is unset, it returns
// false for the docker server image (due to resource usage) but true
// elsewhere. Additionally it also checks for the outdated environment
// variable INDEXED_SEARCH.
func SearchIndexEnabled() bool {
	if v := Get().SearchIndexEnabled; v != nil {
		return *v
	}
	if v := os.Getenv("INDEXED_SEARCH"); v != "" {
		enabled, _ := strconv.ParseBool(v)
		return enabled
	}
	return DeployType() != DeploySingleDocker
}

func BatchChangesEnabled() bool {
	// TODO(campaigns-deprecation): This check can be removed once we remove
	// the deprecated site-config settings.
	if deprecated := Get().CampaignsEnabled; deprecated != nil {
		return *deprecated
	}

	if enabled := Get().BatchChangesEnabled; enabled != nil {
		return *enabled
	}
	return true
}

func BatchChangesRestrictedToAdmins() bool {
	// TODO(campaigns-deprecation): This check can be removed once we remove
	// the deprecated site-config settings.
	if deprecated := Get().CampaignsRestrictToAdmins; deprecated != nil {
		return *deprecated
	}

	if restricted := Get().BatchChangesRestrictToAdmins; restricted != nil {
		return *restricted
	}
	return false
}

func CodeIntelAutoIndexingEnabled() bool {
	if enabled := Get().CodeIntelAutoIndexingEnabled; enabled != nil {
		return *enabled
	}
	return false
}

func ProductResearchPageEnabled() bool {
	if enabled := Get().ProductResearchPageEnabled; enabled != nil {
		return *enabled
	}
	return true
}

func ExternalURL() string {
	return Get().ExternalURL
}

func UsingExternalURL() bool {
	url := Get().ExternalURL
	return !(url == "" || strings.HasPrefix(url, "http://localhost") || strings.HasPrefix(url, "https://localhost") || strings.HasPrefix(url, "http://127.0.0.1") || strings.HasPrefix(url, "https://127.0.0.1")) // CI:LOCALHOST_OK
}

func IsExternalURLSecure() bool {
	return strings.HasPrefix(Get().ExternalURL, "https:")
}

func IsBuiltinSignupAllowed() bool {
	provs := Get().AuthProviders
	for _, prov := range provs {
		if prov.Builtin != nil {
			return prov.Builtin.AllowSignup
		}
	}
	return false
}

// SearchSymbolsParallelism returns 20, or the site config
// "debug.search.symbolsParallelism" value if configured.
func SearchSymbolsParallelism() int {
	val := Get().DebugSearchSymbolsParallelism
	if val == 0 {
		return 20
	}
	return val
}

func BitbucketServerPluginPerm() bool {
	val := Get().ExperimentalFeatures.BitbucketServerFastPerm
	return val == "enabled"
}

func EventLoggingEnabled() bool {
	val := Get().ExperimentalFeatures.EventLogging
	if val == "" {
		return true
	}
	return val == "enabled"
}

func StructuralSearchEnabled() bool {
	val := Get().ExperimentalFeatures.StructuralSearch
	if val == "" {
		return true
	}
	return val == "enabled"
}

func AndOrQueryEnabled() bool {
	e := Get().ExperimentalFeatures
	if e == nil || e.AndOrQuery == "" {
		return true
	}
	return e.AndOrQuery == "enabled"
}

func ExperimentalFeatures() schema.ExperimentalFeatures {
	val := Get().ExperimentalFeatures
	if val == nil {
		return schema.ExperimentalFeatures{}
	}
	return *val
}

// AuthMinPasswordLength returns the value of minimum password length requirement.
// If not set, it returns the default value 12.
func AuthMinPasswordLength() int {
	val := Get().AuthMinPasswordLength
	if val <= 0 {
		return 12
	}
	return val
}

// By default, password reset links are valid for 4 hours.
const defaultPasswordLinkExpiry = 14400

// AuthPasswordResetLinkExpiry returns the time (in seconds) indicating how long password
// reset links are considered valid. If not set, it returns the default value.
func AuthPasswordResetLinkExpiry() int {
	val := Get().AuthPasswordResetLinkExpiry
	if val <= 0 {
		return defaultPasswordLinkExpiry
	}
	return val
}

type ExternalServiceMode int

const (
	ExternalServiceModeDisabled ExternalServiceMode = 0
	ExternalServiceModePublic   ExternalServiceMode = 1
	ExternalServiceModeAll      ExternalServiceMode = 2
)

// ExternalServiceUserMode returns the mode describing if users are allowed to add external services
// for public and private repositories.
func ExternalServiceUserMode() ExternalServiceMode {
	switch Get().ExternalServiceUserMode {
	case "public":
		return ExternalServiceModePublic
	case "all":
		return ExternalServiceModeAll
	default:
		return ExternalServiceModeDisabled
	}
}
