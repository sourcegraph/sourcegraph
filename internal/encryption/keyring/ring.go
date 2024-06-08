package keyring

import (
	"context"
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/encryption/awskms"
	"github.com/sourcegraph/sourcegraph/internal/encryption/cache"
	"github.com/sourcegraph/sourcegraph/internal/encryption/cloudkms"
	"github.com/sourcegraph/sourcegraph/internal/encryption/mounted"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

var (
	mu          sync.RWMutex
	defaultRing Ring
)

// Default returns the default keyring, if you can avoid using this from arbitrary points in your code, please do!
// we would rather inject the individual keys as dependencies when initialised from main(), but if that's impractical
// it's fine to use this.
func Default() Ring {
	mu.RLock()
	defer mu.RUnlock()
	return defaultRing
}

// MockDefault overrides the default keyring.
// Note: This function is defined for testing purpose.
// Use Init to correctly setup a keyring.
func MockDefault(r Ring) {
	mu.Lock()
	defer mu.Unlock()
	defaultRing = r
}

func Init(ctx context.Context) error {
	config := conf.Get().EncryptionKeys
	ring, err := NewRing(ctx, config)
	if err != nil {
		return err
	}
	if ring != nil {
		mu.Lock()
		defaultRing = *ring
		mu.Unlock()
	}

	conf.Watch(func() {
		newConfig := conf.Get().EncryptionKeys
		if newConfig == config {
			return
		}
		newRing, err := NewRing(ctx, newConfig)
		if err != nil {
			panic("creating encryption keyring: " + err.Error())
		}
		mu.Lock()
		defaultRing = *newRing
		mu.Unlock()
	})
	return nil
}

// NewRing creates a keyring.Ring containing all the keys configured in site config
func NewRing(ctx context.Context, keyConfig *schema.EncryptionKeys) (*Ring, error) {
	if keyConfig == nil {
		return nil, nil
	}

	var (
		r   Ring
		err error
	)

	if keyConfig.BatchChangesCredentialKey != nil {
		r.BatchChangesCredentialKey, err = NewKey(ctx, keyConfig.BatchChangesCredentialKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	if keyConfig.ExternalServiceKey != nil {
		r.ExternalServiceKey, err = NewKey(ctx, keyConfig.ExternalServiceKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	if keyConfig.GitHubAppKey != nil {
		r.GitHubAppKey, err = NewKey(ctx, keyConfig.GitHubAppKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	if keyConfig.UserExternalAccountKey != nil {
		r.UserExternalAccountKey, err = NewKey(ctx, keyConfig.UserExternalAccountKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	if keyConfig.WebhookKey != nil {
		r.WebhookKey, err = NewKey(ctx, keyConfig.WebhookKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	if keyConfig.WebhookLogKey != nil {
		r.WebhookLogKey, err = NewKey(ctx, keyConfig.WebhookLogKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	if keyConfig.ExecutorSecretKey != nil {
		r.ExecutorSecretKey, err = NewKey(ctx, keyConfig.ExecutorSecretKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	if keyConfig.OAuthProviderKey != nil {
		r.OAuthProviderKey, err = NewKey(ctx, keyConfig.OAuthProviderKey, keyConfig)
		if err != nil {
			return nil, err
		}
	}

	return &r, nil
}

type Ring struct {
	BatchChangesCredentialKey encryption.Key
	ExternalServiceKey        encryption.Key
	GitHubAppKey              encryption.Key
	OutboundWebhookKey        encryption.Key
	OAuthProviderKey          encryption.Key
	UserExternalAccountKey    encryption.Key
	WebhookKey                encryption.Key
	WebhookLogKey             encryption.Key
	ExecutorSecretKey         encryption.Key
}

func NewKey(ctx context.Context, k *schema.EncryptionKey, config *schema.EncryptionKeys) (encryption.Key, error) {
	if k == nil {
		return nil, errors.Errorf("cannot configure nil key")
	}
	var (
		key encryption.Key
		err error
	)
	switch {
	case k.Cloudkms != nil:
		key, err = cloudkms.NewKey(ctx, *k.Cloudkms)
	case k.Awskms != nil:
		key, err = awskms.NewKey(ctx, *k.Awskms)
	case k.Mounted != nil:
		key, err = mounted.NewKey(ctx, *k.Mounted)
	case k.Noop != nil:
		key = &encryption.NoopKey{}
	default:
		return nil, errors.Errorf("couldn't configure key: %v", *k)
	}
	if err != nil {
		return nil, err
	}

	if config.EnableCache {
		key, err = cache.New(key, config.CacheSize)
	}
	return key, err
}
