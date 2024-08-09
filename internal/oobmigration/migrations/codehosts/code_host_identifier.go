package codehosts

import (
	"net/url"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/codehosts/schema"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NOTE
// This code is largely copy-pasta from internal/extsvc/types.go, because we
// do not want to import internal packages into the OOB migrations where possible.
// This will allow OOB migrations to still work even if internal packages change in
// incompatible ways for multi version upgrades from versions many many releases ago.

// =======================================================================
// =======================================================================
// =======================================================================
// =======================================================================

// uniqueCodeHostIdentifierFromConfig returns a string that uniquely identifies the
// instance of a code host an external service is pointing at.
//
// E.g.: multiple external service configurations might point at the same
// GitHub Enterprise instance. All of them would return the normalized base URL
// as a unique identifier.
//
// In case an external service doesn't have a base URL (e.g. AWS Code Commit)
// another unique identifier is returned.
//
// This function can be used to group external services by the code host
// instance they point at.
func uniqueCodeHostIdentifierFromConfig(kind, config string) (string, error) {
	cfg, err := parseConfig(kind, config)
	if err != nil {
		return "", err
	}

	return uniqueCodeHostIdentifier(kind, cfg)
}

// parseConfig attempts to unmarshal the given JSON config into a configuration struct defined in the schema package.
func parseConfig(kind, config string) (any, error) {
	cfg, err := getConfigPrototype(kind)
	if err != nil {
		return nil, err
	}

	return cfg, jsonc.Unmarshal(config, cfg)
}

func getConfigPrototype(kind string) (any, error) {
	variant, err := variantValueOf(kind)
	if err != nil {
		return nil, errors.Errorf("unknown external service kind %q", kind)
	}
	if variant.ConfigPrototype() == nil {
		return nil, errors.Errorf("no config prototype for %q", kind)
	}
	return variant.ConfigPrototype(), nil
}

// case-insensitive matching of an input string against the Variant kinds and types
// returns the matching Variant or an error if the given value is not a kind or type value
func variantValueOf(input string) (variant, error) {
	for variant, value := range variantValuesMap {
		if strings.EqualFold(value.AsKind, input) || strings.EqualFold(value.AsType, input) {
			return variant, nil
		}
	}
	return 0, errors.Newf("no Variant found for %s", input)
}

// variant enumerates different types/kinds of external services.
// Currently it backs the Type... and Kind... variables, avoiding duplication.
// Eventually it will replace the Type... and Kind... variables,
// providing a single place to declare and resolve values for Type and Kind
//
// Types and Kinds are exposed through AsKind and AsType functions
// so that usages relying on the particular string of Type vs Kind
// will continue to behave correctly.
// The Type... and Kind... variables are left in place to avoid edge-case issues and to support
// commits that come in while the switch to variant is ongoing.
// The Type... and Kind... variables are turned from consts into vars and use
// the corresponding variant's AsType()/AsKind() functions.
// Consolidating Type... and Kind... into a single enum should decrease the smell
// and increase the usability and maintainability of this code.
// Note that Go Packages and Modules seem to have been a victim of the confusion engendered by having both Type and Kind:
// There are `KindGoPackages` and `TypeGoModules`, both with the value of (case insensitivly) "gomodules".
// Those two have been standardized as `VariantGoPackages` in the variant enum to align naming conventions with the other `...Packages` variables.
//
// To add another external service variant
// 1. Add the name to the enum
// 2. Add an entry to the `variantValuesMap` map, containing the appropriate values for `AsType`, `AsKind`, and the other values, if applicable
// 3. Use that variant elsewhere in code, using the `AsType` and `AsKind` functions as necessary.
// Note: do not use the enum value directly, instead use the helper functions `AsType` and `AsKind`.
type variant int64

const (
	// start from 1 to avoid accicentally using the default value
	_ variant = iota

	// variantAWSCodeCommit is the (api.ExternalRepoSpec).ServiceType value for AWS CodeCommit
	// repositories. The ServiceID value is the ARN (Amazon Resource Name) omitting the repository name
	// suffix (e.g., "arn:aws:codecommit:us-west-1:123456789:").
	variantAWSCodeCommit

	// variantBitbucketServer is the (api.ExternalRepoSpec).ServiceType value for Bitbucket Server projects. The
	// ServiceID value is the base URL to the Bitbucket Server instance.
	variantBitbucketServer

	// variantBitbucketCloud is the (api.ExternalRepoSpec).ServiceType value for Bitbucket Cloud projects. The
	// ServiceID value is the base URL to the Bitbucket Cloud.
	variantBitbucketCloud

	// variantGerrit is the (api.ExternalRepoSpec).ServiceType value for Gerrit projects.
	variantGerrit

	// variantGitHub is the (api.ExternalRepoSpec).ServiceType value for GitHub repositories. The ServiceID value
	// is the base URL to the GitHub instance (https://github.com or the GitHub Enterprise URL).
	variantGitHub

	// variantGitLab is the (api.ExternalRepoSpec).ServiceType value for GitLab projects. The ServiceID
	// value is the base URL to the GitLab instance (https://gitlab.com or self-hosted GitLab URL).
	variantGitLab

	// variantGitolite is the (api.ExternalRepoSpec).ServiceType value for Gitolite projects.
	variantGitolite

	// variantPerforce is the (api.ExternalRepoSpec).ServiceType value for Perforce projects.
	variantPerforce

	// variantPhabricator is the (api.ExternalRepoSpec).ServiceType value for Phabricator projects.
	variantPhabricator

	// VariangGoPackages is the (api.ExternalRepoSpec).ServiceType value for Golang packages.
	variantGoPackages

	// variantJVMPackages is the (api.ExternalRepoSpec).ServiceType value for Maven packages (Java/JVM ecosystem libraries).
	variantJVMPackages

	// variantPagure is the (api.ExternalRepoSpec).ServiceType value for Pagure projects.
	variantPagure

	// variantAzureDevOps is the (api.ExternalRepoSpec).ServiceType value for ADO projects.
	variantAzureDevOps

	// variantAzureDevOps is the (api.ExternalRepoSpec).ServiceType value for ADO projects.
	variantSCIM

	// variantNpmPackages is the (api.ExternalRepoSpec).ServiceType value for Npm packages (JavaScript/VariantScript ecosystem libraries).
	variantNpmPackages

	// variantPythonPackages is the (api.ExternalRepoSpec).ServiceType value for Python packages.
	variantPythonPackages

	// variantRustPackages is the (api.ExternalRepoSpec).ServiceType value for Rust packages.
	variantRustPackages

	// variantRubyPackages is the (api.ExternalRepoSpec).ServiceType value for Ruby packages.
	variantRubyPackages

	// variantOther is the (api.ExternalRepoSpec).ServiceType value for other projects.
	variantOther

	// variantLocalGit is the (api.ExternalRepoSpec).ServiceType for local git repositories
	variantLocalGit
)

func (v variant) AsKind() string {
	return variantValuesMap[v].AsKind
}

func (v variant) ConfigPrototype() any {
	f := variantValuesMap[v].ConfigPrototype
	if f == nil {
		return nil
	}
	return f()
}

type variantValues struct {
	AsKind                string
	AsType                string
	ConfigPrototype       func() any
	WebhookURLPath        string
	SupportsRepoExclusion bool
}

var variantValuesMap = map[variant]variantValues{
	variantAWSCodeCommit:   {AsKind: "AWSCODECOMMIT", AsType: "awscodecommit", ConfigPrototype: func() any { return &schema.AWSCodeCommitConnection{} }, SupportsRepoExclusion: true},
	variantAzureDevOps:     {AsKind: "AZUREDEVOPS", AsType: "azuredevops", ConfigPrototype: func() any { return &schema.AzureDevOpsConnection{} }, SupportsRepoExclusion: true},
	variantBitbucketCloud:  {AsKind: "BITBUCKETCLOUD", AsType: "bitbucketCloud", ConfigPrototype: func() any { return &schema.BitbucketCloudConnection{} }, WebhookURLPath: "bitbucket-cloud-webhooks", SupportsRepoExclusion: true},
	variantBitbucketServer: {AsKind: "BITBUCKETSERVER", AsType: "bitbucketServer", ConfigPrototype: func() any { return &schema.BitbucketServerConnection{} }, WebhookURLPath: "bitbucket-server-webhooks", SupportsRepoExclusion: true},
	variantGerrit:          {AsKind: "GERRIT", AsType: "gerrit", ConfigPrototype: func() any { return &schema.GerritConnection{} }},
	variantGitHub:          {AsKind: "GITHUB", AsType: "github", ConfigPrototype: func() any { return &schema.GitHubConnection{} }, WebhookURLPath: "github-webhooks", SupportsRepoExclusion: true},
	variantGitLab:          {AsKind: "GITLAB", AsType: "gitlab", ConfigPrototype: func() any { return &schema.GitLabConnection{} }, WebhookURLPath: "gitlab-webhooks", SupportsRepoExclusion: true},
	variantGitolite:        {AsKind: "GITOLITE", AsType: "gitolite", ConfigPrototype: func() any { return &schema.GitoliteConnection{} }, SupportsRepoExclusion: true},
	variantGoPackages:      {AsKind: "GOMODULES", AsType: "goModules", ConfigPrototype: func() any { return &schema.GoModulesConnection{} }},
	variantJVMPackages:     {AsKind: "JVMPACKAGES", AsType: "jvmPackages", ConfigPrototype: func() any { return &schema.JVMPackagesConnection{} }},
	variantNpmPackages:     {AsKind: "NPMPACKAGES", AsType: "npmPackages", ConfigPrototype: func() any { return &schema.NpmPackagesConnection{} }},
	variantOther:           {AsKind: "OTHER", AsType: "other", ConfigPrototype: func() any { return &schema.OtherExternalServiceConnection{} }},
	variantPagure:          {AsKind: "PAGURE", AsType: "pagure", ConfigPrototype: func() any { return &schema.PagureConnection{} }},
	variantPerforce:        {AsKind: "PERFORCE", AsType: "perforce", ConfigPrototype: func() any { return &schema.PerforceConnection{} }},
	variantPhabricator:     {AsKind: "PHABRICATOR", AsType: "phabricator", ConfigPrototype: func() any { return &schema.PhabricatorConnection{} }},
	variantPythonPackages:  {AsKind: "PYTHONPACKAGES", AsType: "pythonPackages", ConfigPrototype: func() any { return &schema.PythonPackagesConnection{} }},
	variantRubyPackages:    {AsKind: "RUBYPACKAGES", AsType: "rubyPackages", ConfigPrototype: func() any { return &schema.RubyPackagesConnection{} }},
	variantRustPackages:    {AsKind: "RUSTPACKAGES", AsType: "rustPackages", ConfigPrototype: func() any { return &schema.RustPackagesConnection{} }},
	variantSCIM:            {AsKind: "SCIM", AsType: "scim"},
	variantLocalGit:        {AsKind: "LOCALGIT", AsType: "localgit", ConfigPrototype: func() any { return &schema.LocalGitExternalService{} }},
}

func uniqueCodeHostIdentifier(kind string, cfg any) (string, error) {
	var rawURL string
	switch c := cfg.(type) {
	case *schema.GitLabConnection:
		rawURL = c.Url
	case *schema.GitHubConnection:
		rawURL = c.Url
	case *schema.AzureDevOpsConnection:
		rawURL = c.Url
	case *schema.BitbucketServerConnection:
		rawURL = c.Url
	case *schema.BitbucketCloudConnection:
		rawURL = c.Url
	case *schema.GerritConnection:
		rawURL = c.Url
	case *schema.PhabricatorConnection:
		rawURL = c.Url
	case *schema.OtherExternalServiceConnection:
		rawURL = c.Url
	case *schema.GitoliteConnection:
		rawURL = c.Host
	case *schema.AWSCodeCommitConnection:
		// AWS Code Commit does not have a URL in the config, so we return a
		// unique string here and return early:
		return c.Region + ":" + c.AccessKeyID, nil
	case *schema.PerforceConnection:
		// Perforce uses the P4PORT to specify the instance, so we use that
		return c.P4Port, nil
	case *schema.GoModulesConnection:
		return variantGoPackages.AsKind(), nil
	case *schema.JVMPackagesConnection:
		return variantJVMPackages.AsKind(), nil
	case *schema.NpmPackagesConnection:
		return variantNpmPackages.AsKind(), nil
	case *schema.PythonPackagesConnection:
		return variantPythonPackages.AsKind(), nil
	case *schema.RustPackagesConnection:
		return variantRustPackages.AsKind(), nil
	case *schema.RubyPackagesConnection:
		return variantRubyPackages.AsKind(), nil
	case *schema.PagureConnection:
		rawURL = c.Url
	case *schema.LocalGitExternalService:
		return variantLocalGit.AsKind(), nil
	default:
		return "", errors.Errorf("unknown external service kind: %s", kind)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	return normalizeBaseURL(u).String(), nil
}

// normalizeBaseURL modifies the input and returns a normalized form of the a base URL with insignificant
// differences (such as in presence of a trailing slash, or hostname case) eliminated. Its return value should be
// used for the (ExternalRepoSpec).ServiceID field (and passed to XyzExternalRepoSpec) instead of a non-normalized
// base URL.
func normalizeBaseURL(baseURL *url.URL) *url.URL {
	baseURL.Host = strings.ToLower(baseURL.Host)
	if !strings.HasSuffix(baseURL.Path, "/") {
		baseURL.Path += "/"
	}
	return baseURL
}
