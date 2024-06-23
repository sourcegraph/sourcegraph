package internal

type GitserverConfiguration interface {
	DisableAutoGitUpdates() bool
	PinnedRepos() []string
	GitServerAddresses() []string
	CloneProgressLog() bool
	EnablePerforceChangelistMapping() bool
	GitRecorder() GitRecorder
	PostgresDSN() string
	GitMaxConcurrentClones() int
	InstanceExternalURL() string
	TraceURLTemplate() string
	// CustomGitFetch description: JSON array of configuration that maps from Git clone URL domain/path to custom git fetch command. To enable this feature set environment variable `ENABLE_CUSTOM_GIT_FETCH` as `true` on gitserver.
	CustomGitFetch() []*CustomGitFetchMapping
}

// GitRecorder description: Record git operations that are executed on configured repositories.
type GitRecorder struct {
	// IgnoredGitCommands description: List of git commands that should be ignored and not recorded.
	IgnoredGitCommands []string `json:"ignoredGitCommands,omitempty"`
	// Repos description: List of repositories whose git operations should be recorded. To record commands on all repositories, simply pass in an asterisk as the only item in the array.
	Repos []string `json:"repos,omitempty"`
	// Size description: Defines how many recordings to keep. Once this size is reached, the oldest entry will be removed.
	Size int `json:"size,omitempty"`
}

// CustomGitFetchMapping description: Mapping from Git clone URl domain/path to git fetch command. The `domainPath` field contains the Git clone URL domain/path part. The `fetch` field contains the custom git fetch command.
type CustomGitFetchMapping struct {
	// DomainPath description: Git clone URL domain/path
	DomainPath string `json:"domainPath"`
	// Fetch description: Git fetch command
	Fetch string `json:"fetch"`
}
