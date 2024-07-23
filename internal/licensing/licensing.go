package licensing

import (
	"bytes"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/internal/redispool"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Info wraps the lower-level license.Info and exposes plan and feature information.
type Info struct {
	license.Info
}

// publicKeyData is the public key used to verify Sourcegraph license keys
const publicKeyData = `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDUUd9r83fGmYVLzcqQp5InyAoJB5lLxlM7s41SUUtxfnG6JpmvjNd+WuEptJGk0C/Zpyp/cCjCV4DljDs8Z7xjRbvJYW+vklFFxXrMTBs/+HjpIBKlYTmG8SqTyXyu1s4485Kh1fEC5SK6z2IbFaHuSHUXgDi/IepSOg1QudW4n8J91gPtT2E30/bPCBRq8oz/RVwJSDMvYYjYVb//LhV0Mx3O6hg4xzUNuwiCtNjCJ9t4YU2sV87+eJwWtQNbSQ8TelQa8WjG++XSnXUHw12bPDe7wGL/7/EJb7knggKSAMnpYpCyV35dyi4DsVc46c+b6P0gbVSosh3Uc3BJHSWF`

// publicKey is the public key used to verify product license keys.
var publicKey = func() ssh.PublicKey {
	// If a key is set from SOURCEGRAPH_LICENSE_GENERATION_KEY, use that key to verify licenses instead.
	if licenseGenerationPrivateKey != nil {
		return licenseGenerationPrivateKey.PublicKey()
	}

	// This key is hardcoded here intentionally (we only have one private signing key, and we don't yet
	// support/need key rotation). The corresponding private key is at
	// https://team-sourcegraph.1password.com/vaults/dnrhbauihkhjs5ag6vszsme45a/allitems/zkdx6gpw4uqejs3flzj7ef5j4i
	//
	// To convert PKCS#8 format (which `openssl rsa -in key.pem -pubout` produces) to the format
	// that ssh.ParseAuthorizedKey reads here, use `ssh-keygen -i -mPKCS8 -f key.pub`.
	var err error
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyData))
	if err != nil {
		panic("failed to parse public key for license verification: " + err.Error())
	}
	return publicKey
}()

// IsLicensePublicKeyOverridden checks if the hardcoded license public key has been overridden with a *different* key
func IsLicensePublicKeyOverridden() bool {
	return publicKeyData != string(bytes.TrimSpace(ssh.MarshalAuthorizedKey(publicKey)))
}

// toInfo converts from the return type of license.ParseSignedKey to the return type of this
// package's methods (which use the Info wrapper type).
func toInfo(origInfo *license.Info, origSignature string, origErr error) (info *Info, signature string, err error) {
	if origInfo != nil {
		info = &Info{Info: *origInfo}
	}
	return info, origSignature, origErr
}

// ParseProductLicenseKey parses and verifies the license key using the license verification public
// key (publicKey in this package).
func ParseProductLicenseKey(licenseKey string) (info *Info, signature string, err error) {
	return toInfo(license.ParseSignedKey(licenseKey, publicKey))
}

func GetFreeLicenseInfo() (info *Info) {
	return &Info{license.Info{
		Tags:      []string{"plan:free-1"},
		UserCount: 10,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Hour * 8760),
	}}
}

var MockParseProductLicenseKeyWithBuiltinOrGenerationKey func(licenseKey string) (*Info, string, error)

// ParseProductLicenseKeyWithBuiltinOrGenerationKey is like ParseProductLicenseKey, except it tries
// parsing and verifying the license key with the license generation key (if set), instead of always
// using the builtin license key.
//
// It is useful for local development when using a test license generation key (whose signatures
// aren't considered valid when verified using the builtin public key).
func ParseProductLicenseKeyWithBuiltinOrGenerationKey(licenseKey string) (*Info, string, error) {
	if MockParseProductLicenseKeyWithBuiltinOrGenerationKey != nil {
		return MockParseProductLicenseKeyWithBuiltinOrGenerationKey(licenseKey)
	}

	var k ssh.PublicKey
	if licenseGenerationPrivateKey != nil {
		k = licenseGenerationPrivateKey.PublicKey()
	} else {
		k = publicKey
	}
	return toInfo(license.ParseSignedKey(licenseKey, k))
}

// Cache the parsing of the license key because public key crypto can be slow.
var (
	mu            sync.Mutex
	lastKeyText   string
	lastInfo      *Info
	lastSignature string
)

var MockGetConfiguredProductLicenseInfo func() (*license.Info, string, error)

// GetConfiguredProductLicenseInfo returns information about the current product license key
// specified in site configuration.
func GetConfiguredProductLicenseInfo() (*Info, error) {
	info, _, err := GetConfiguredProductLicenseInfoWithSignature()
	return info, err
}

func IsLicenseValid() bool {
	val := store.Get(LicenseValidityStoreKey)
	if val.IsNil() {
		return true
	}

	v, err := val.Bool()
	if err != nil {
		return true
	}

	return v
}

var store = redispool.Store

func GetLicenseInvalidReason() string {
	if IsLicenseValid() {
		return ""
	}

	defaultReason := "unknown"

	val := store.Get(LicenseInvalidReason)
	if val.IsNil() {
		return defaultReason
	}

	v, err := val.String()
	if err != nil {
		return defaultReason
	}

	return v
}

// GetConfiguredProductLicenseInfoWithSignature returns information about the current product license key
// specified in site configuration, with the signed key's signature.
func GetConfiguredProductLicenseInfoWithSignature() (*Info, string, error) {
	if MockGetConfiguredProductLicenseInfo != nil {
		return toInfo(MockGetConfiguredProductLicenseInfo())
	}

	if keyText := conf.Get().LicenseKey; keyText != "" {
		mu.Lock()
		defer mu.Unlock()

		var (
			info      *Info
			signature string
		)
		if keyText == lastKeyText {
			info = lastInfo
			signature = lastSignature
		} else {
			var err error
			info, signature, err = ParseProductLicenseKey(keyText)
			if err != nil {
				return nil, "", err
			}

			if err = info.HasUnknownPlan(); err != nil {
				return nil, "", err
			}

			lastKeyText = keyText
			lastInfo = info
			lastSignature = signature
		}
		return info, signature, nil
	}

	// If no license key, default to free tier
	return GetFreeLicenseInfo(), "", nil
}

// envLicenseGenerationPrivateKey (the env var SOURCEGRAPH_LICENSE_GENERATION_KEY) is the
// PEM-encoded form of the private key used to sign product license keys. It is stored at
// https://team-sourcegraph.1password.com/vaults/dnrhbauihkhjs5ag6vszsme45a/allitems/zkdx6gpw4uqejs3flzj7ef5j4i.
var envLicenseGenerationPrivateKey = env.Get("SOURCEGRAPH_LICENSE_GENERATION_KEY", "", "the PEM-encoded form of the private key used to sign product license keys ("+license.GenerationPrivateKeyURL+")")

// licenseGenerationPrivateKey is the private key used to generate license keys.
var licenseGenerationPrivateKey = func() ssh.Signer {
	if envLicenseGenerationPrivateKey == "" {
		// Most Sourcegraph instances don't use/need this key. Generally only Sourcegraph.com and
		// local dev will have this key set.
		return nil
	}
	privateKey, err := ssh.ParsePrivateKey([]byte(envLicenseGenerationPrivateKey))
	if err != nil {
		log.Fatalf("Failed to parse private key in SOURCEGRAPH_LICENSE_GENERATION_KEY env var: %s.", err)
	}
	return privateKey
}()

// GenerateProductLicenseKey generates a product license key using the license generation private
// key configured in site configuration.
func GenerateProductLicenseKey(info license.Info) (licenseKey string, version int, err error) {
	if envLicenseGenerationPrivateKey == "" {
		const msg = "no product license generation private key was configured"
		if env.InsecureDev {
			// Show more helpful error message in local dev.
			return "", 0, errors.Errorf("%s (for testing by Sourcegraph staff: set the SOURCEGRAPH_LICENSE_GENERATION_KEY env var to the key obtained at %s)", msg, license.GenerationPrivateKeyURL)
		}
		return "", 0, errors.New(msg)
	}

	licenseKey, version, err = license.GenerateSignedKey(info, licenseGenerationPrivateKey)
	if err != nil {
		return "", 0, errors.Wrap(err, "generate signed key")
	}
	return licenseKey, version, nil
}
