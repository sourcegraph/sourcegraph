package schema

// NOTE
// This code is largely copy-pasta from internal/extsvc/types.go, because we
// do not want to import internal packages into the OOB migrations where possible.
// This will allow OOB migrations to still work even if internal packages change in
// incompatible ways for multi version upgrades from versions many many releases ago.

// =======================================================================
// =======================================================================
// =======================================================================
// =======================================================================

// AWSCodeCommitConnection description: Configuration for a connection to AWS CodeCommit.
type AWSCodeCommitConnection struct {
	// AccessKeyID description: The AWS access key ID to use when listing and updating repositories from AWS CodeCommit. Must have the AWSCodeCommitReadOnly IAM policy.
	AccessKeyID string `json:"accessKeyID"`
	// Region description: The AWS region in which to access AWS CodeCommit. See the list of supported regions at https://docs.aws.amazon.com/codecommit/latest/userguide/regions.html#regions-git.
	Region string `json:"region"`
}

// AzureDevOpsConnection description: Configuration for a connection to Azure DevOps.
type AzureDevOpsConnection struct {
	// Url description: URL for Azure DevOps Services, set to https://dev.azure.com.
	Url string `json:"url"`
}

// BitbucketCloudConnection description: Configuration for a connection to Bitbucket Cloud.
type BitbucketCloudConnection struct {
	// ApiURL description: The API URL of Bitbucket Cloud, such as https://api.bitbucket.org. Generally, admin should not modify the value of this option because Bitbucket Cloud is a public hosting platform.
	ApiURL string `json:"apiURL,omitempty"`
	// RateLimit description: Rate limit applied when making background API requests to Bitbucket Cloud.
	RateLimit *BitbucketCloudRateLimit `json:"rateLimit,omitempty"`
	// Url description: URL of Bitbucket Cloud, such as https://bitbucket.org. Generally, admin should not modify the value of this option because Bitbucket Cloud is a public hosting platform.
	Url string `json:"url"`
}

// BitbucketCloudRateLimit description: Rate limit applied when making background API requests to Bitbucket Cloud.
type BitbucketCloudRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 500, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 500 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// BitbucketServerConnection description: Configuration for a connection to Bitbucket Server / Bitbucket Data Center.
type BitbucketServerConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to BitbucketServer.
	RateLimit *BitbucketServerRateLimit `json:"rateLimit,omitempty"`
	// Url description: URL of a Bitbucket Server / Bitbucket Data Center instance, such as https://bitbucket.example.com.
	Url string `json:"url"`
}

// BitbucketServerRateLimit description: Rate limit applied when making background API requests to BitbucketServer.
type BitbucketServerRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 500, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 500 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// GerritConnection description: Configuration for a connection to Gerrit.
type GerritConnection struct {
	// Url description: URL of a Gerrit instance, such as https://gerrit.example.com.
	Url string `json:"url"`
}

// GitHubConnection description: Configuration for a connection to GitHub or GitHub Enterprise.
type GitHubConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to GitHub.
	RateLimit *GitHubRateLimit `json:"rateLimit,omitempty"`
	// Url description: URL of a GitHub instance, such as https://github.com or https://github-enterprise.example.com.
	Url string `json:"url"`
}

// GitHubRateLimit description: Rate limit applied when making background API requests to GitHub.
type GitHubRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// GitLabConnection description: Configuration for a connection to GitLab (GitLab.com or GitLab self-managed).
type GitLabConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to GitLab.
	RateLimit *GitLabRateLimit `json:"rateLimit,omitempty"`
	// Url description: URL of a GitLab instance, such as https://gitlab.example.com or (for GitLab.com) https://gitlab.com.
	Url string `json:"url"`
}

// GitLabRateLimit description: Rate limit applied when making background API requests to GitLab.
type GitLabRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// GitoliteConnection description: Configuration for a connection to Gitolite.
type GitoliteConnection struct {
	// Host description: Gitolite host that stores the repositories (e.g., git@gitolite.example.com, ssh://git@gitolite.example.com:2222/).
	Host string `json:"host"`
}

// GoModulesConnection description: Configuration for a connection to Go module proxies
type GoModulesConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to the configured Go module proxies.
	RateLimit *GoRateLimit `json:"rateLimit,omitempty"`
	// Urls description: The list of Go module proxy URLs to fetch modules from. 404 Not found or 410 Gone responses will result in the next URL to be attempted.
	Urls []string `json:"urls"`
}

// GoRateLimit description: Rate limit applied when making background API requests to the configured Go module proxies.
type GoRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// JVMPackagesConnection description: Configuration for a connection to a JVM packages repository.
type JVMPackagesConnection struct {
	// Maven description: Configuration for resolving from Maven repositories.
	Maven Maven `json:"maven"`
}

// LocalGitExternalService description: Configuration for integration local Git repositories.
type LocalGitExternalService struct {
	Repos []*LocalGitRepoPattern `json:"repos,omitempty"`
}
type LocalGitRepoPattern struct {
	Group   string `json:"group,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// Maven description: Configuration for resolving from Maven repositories.
type Maven struct {
	// RateLimit description: Rate limit applied when making background API requests to the Maven repository.
	RateLimit *MavenRateLimit `json:"rateLimit,omitempty"`
}

// MavenRateLimit description: Rate limit applied when making background API requests to the Maven repository.
type MavenRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// NpmPackagesConnection description: Configuration for a connection to an npm packages repository.
type NpmPackagesConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to the npm registry.
	RateLimit *NpmRateLimit `json:"rateLimit,omitempty"`
	// Registry description: The URL at which the npm registry can be found.
	Registry string `json:"registry"`
}

// NpmRateLimit description: Rate limit applied when making background API requests to the npm registry.
type NpmRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// OtherExternalServiceConnection description: Configuration for a Connection to Git repositories for which an external service integration isn't yet available.
type OtherExternalServiceConnection struct {
	Url string `json:"url,omitempty"`
}

// PagureConnection description: Configuration for a connection to Pagure.
type PagureConnection struct {
	// RateLimit description: Rate limit applied when making API requests to Pagure.
	RateLimit *PagureRateLimit `json:"rateLimit,omitempty"`
	// Url description: URL of a Pagure instance, such as https://pagure.example.com
	Url string `json:"url,omitempty"`
}

// PagureRateLimit description: Rate limit applied when making API requests to Pagure.
type PagureRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 500, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 500 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// PerforceConnection description: Configuration for a connection to Perforce Server.
type PerforceConnection struct {
	// P4Port description: The Perforce Server address to be used for p4 CLI (P4PORT).
	P4Port string `json:"p4.port"`
	// RateLimit description: Rate limit applied when making background API requests to Perforce.
	RateLimit *PerforceRateLimit `json:"rateLimit,omitempty"`
}

// PerforceRateLimit description: Rate limit applied when making background API requests to Perforce.
type PerforceRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// PhabricatorConnection description: Configuration for a connection to Phabricator.
type PhabricatorConnection struct {
	// Url description: URL of a Phabricator instance, such as https://phabricator.example.com
	Url string `json:"url,omitempty"`
}

// PythonPackagesConnection description: Configuration for a connection to Python simple repository APIs compatible with PEP 503
type PythonPackagesConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to the configured Python simple repository APIs.
	RateLimit *PythonRateLimit `json:"rateLimit,omitempty"`
	// Urls description: The list of Python simple repository URLs to fetch packages from. 404 Not found or 410 Gone responses will result in the next URL to be attempted.
	Urls []string `json:"urls"`
}

// PythonRateLimit description: Rate limit applied when making background API requests to the configured Python simple repository APIs.
type PythonRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// RubyPackagesConnection description: Configuration for a connection to Ruby packages
type RubyPackagesConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to the configured Ruby repository APIs.
	RateLimit *RubyRateLimit `json:"rateLimit,omitempty"`
}

// RubyRateLimit description: Rate limit applied when making background API requests to the configured Ruby repository APIs.
type RubyRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}

// RustPackagesConnection description: Configuration for a connection to Rust packages
type RustPackagesConnection struct {
	// RateLimit description: Rate limit applied when making background API requests to the configured Rust repository APIs.
	RateLimit *RustRateLimit `json:"rateLimit,omitempty"`
}

// RustRateLimit description: Rate limit applied when making background API requests to the configured Rust repository APIs.
type RustRateLimit struct {
	// Enabled description: true if rate limiting is enabled.
	Enabled bool `json:"enabled"`
	// RequestsPerHour description: Requests per hour permitted. This is an average, calculated per second. Internally, the burst limit is set to 100, which implies that for a requests per hour limit as low as 1, users will continue to be able to send a maximum of 100 requests immediately, provided that the complexity cost of each request is 1.
	RequestsPerHour float64 `json:"requestsPerHour"`
}
