package awscodecommit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/aws/aws-sdk-go-v2/aws"
	codecommittypes "github.com/aws/aws-sdk-go-v2/service/codecommit/types"
	"github.com/aws/smithy-go"

	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Client is a AWS CodeCommit API client.
type Client struct {
	aws       aws.Config
	repoCache *rcache.Cache
}

// NewClient creates a new AWS CodeCommit API client.
func NewClient(config aws.Config) *Client {
	// Cache for repository metadata. The configuration-specific key prefix is not known
	// synchronously, so cache consumers must call (*Client).cacheKeyPrefix to obtain the
	// prefix value and prepend it explicitly.
	repoCache := rcache.NewWithTTL("cc_repo:", 60 /* seconds */, false)

	return &Client{
		aws:       config,
		repoCache: repoCache,
	}
}

// cacheKeyPrefix returns the cache key prefix to use. It incorporates the credentials to
// avoid leaking cached data that was fetched with one set of credentials to a (possibly
// different) user with a different set of credentials.
func (c *Client) cacheKeyPrefix(ctx context.Context) (string, error) {
	cred, err := c.aws.Credentials.Retrieve(ctx) // typically instant, or at least cached and fast
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(cred.AccessKeyID + ":" + cred.SecretAccessKey + ":" + cred.SessionToken))
	return hex.EncodeToString(key[:]), nil
}

// ErrNotFound is when the requested AWS CodeCommit repository is not found.
var ErrNotFound = errors.New("AWS CodeCommit repository not found")

// IsNotFound reports whether err is a AWS CodeCommit API not-found error or the
// equivalent cached response error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.HasType(err, &codecommittypes.RepositoryDoesNotExistException{})
}

// IsUnauthorized reports whether err is a AWS CodeCommit API unauthorized error.
func IsUnauthorized(err error) bool {
	var e smithy.APIError
	return errors.As(err, &e) && e.ErrorCode() == "SignatureDoesNotMatch"
}
