package types

import (
	"context"
	"time"

	gogithub "github.com/google/go-github/v55/github"

	"github.com/sourcegraph/sourcegraph/internal/types"
)

// GitHubApp represents a GitHub App.
type GitHubApp struct {
	ID            int
	AppID         int
	Name          string
	Domain        types.GitHubAppDomain
	Slug          string
	BaseURL       string
	AppURL        string
	ClientID      string
	ClientSecret  string
	WebhookSecret string
	WebhookID     *int
	PrivateKey    string
	EncryptionKey string
	Logo          string
	Kind          types.GitHubAppKind
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func NormalizeGitHubAppKind(app *GitHubApp) types.GitHubAppKind {
	// currently GitHub apps with the repo domain are only used for repo syncing
	// so we normalize them to repo syncing apps
	if app.Domain == types.ReposGitHubAppDomain {
		return types.RepoSyncGitHubAppKind
	}

	// If we fall into this block it means the `domain` is for BatchChanges
	// and if the `kind` is unspecified we default to CommitSigning
	if app.Kind == "" {
		return types.CommitSigningGitHubAppKind
	}

	return app.Kind
}

// GitHubAppInstallation represents an installation of a GitHub App.
type GitHubAppInstallation struct {
	ID               int
	AppID            int
	InstallationID   int
	URL              string
	AccountLogin     string
	AccountAvatarURL string
	AccountURL       string
	AccountType      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type GitHubAppClient interface {
	GetAppInstallations(ctx context.Context, page int) (_ []*gogithub.Installation, hasNextPage bool, _ error)
}
