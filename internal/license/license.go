// Package license provides license key generation and verification.
//
// License keys are generated and signed using Sourcegraph's private key. Sourcegraph instances must
// be able to verify the license key offline, so all license information (such as the max user
// count) is encoded in the license itself.
//
// Key rotation, license key revocation, etc., are not implemented.
package license

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// GenerationPrivateKeyURL is the URL where Sourcegraph staff can find the private key for
// generating licenses.
//
// NOTE: If you change this, use text search to replace other instances of it (in source code
// comments).
const GenerationPrivateKeyURL = "https://team-sourcegraph.1password.com/vaults/dnrhbauihkhjs5ag6vszsme45a/allitems/zkdx6gpw4uqejs3flzj7ef5j4i"

// Info contains information about a license key. In the signed license key that Sourcegraph
// provides to customers, this value is signed but not encrypted. This value is not secret, and
// anyone with a license key can view (but not forge) this information.
//
// NOTE: If you change these fields, you MUST handle backward compatibility. Existing licenses that
// were generated with the old fields must still work until all customers have added the new
// license. Increment (encodedInfo).Version and modify version() implementation when you make
// backward-incompatbile changes.
type Info struct {
	// Tags denote features/restrictions (e.g., "starter" or "dev")
	Tags []string `json:"t"`
	// UserCount is the number of users that this license is valid for
	UserCount uint `json:"u"`
	// CreatedAt is the date this license was created at. May be zero for
	// licenses version less than 3.
	CreatedAt time.Time `json:"c"`
	// ExpiresAt is the date when this license expires
	ExpiresAt time.Time `json:"e"`
	// SalesforceSubscriptionID is the optional Salesforce subscription ID to link licenses
	// to Salesforce subscriptions
	SalesforceSubscriptionID *string `json:"sf_sub_id,omitempty"`
	// SalesforceOpportunityID is the optional Salesforce opportunity ID to link licenses
	// to Salesforce opportunities
	SalesforceOpportunityID *string `json:"sf_opp_id,omitempty"`
}

// IsExpired reports whether the license has expired.
func (l Info) IsExpired() bool {
	return l.ExpiresAt.Before(time.Now())
}

// IsExpiringSoon reports whether the license will expire within the next 7 days.
func (l Info) IsExpiringSoon() bool {
	return l.ExpiresAt.Add(-7 * 24 * time.Hour).Before(time.Now())
}

// HasTag reports whether tag is in l's list of tags.
func (l Info) HasTag(tag string) bool {
	for _, t := range l.Tags {
		// NOTE: Historically, our web form have accidentally submitted tags with
		//  surrounding spaces.
		if tag == strings.TrimSpace(t) {
			return true
		}
	}
	return false
}

func (l *Info) String() string {
	if l == nil {
		return "nil license"
	}
	return fmt.Sprintf("license(tags=%v, userCount=%d, expiresAt=%s)", l.Tags, l.UserCount, l.ExpiresAt)
}

// ParseTagsInput parses a string of comma-separated tags. It removes whitespace around tags and
// removes empty tags before returning the list of tags.
func ParseTagsInput(tagsStr string) []string {
	if tagsStr == "" {
		return nil
	}
	tags := strings.Split(tagsStr, ",")
	return SanitizeTagsList(tags)
}

// SanitizeTagsList removes whitespace around tags and removes empty tags before
// returning the list of tags.
func SanitizeTagsList(tags []string) []string {
	sTags := make([]string, 0)
	for _, tag := range tags {
		if tag := strings.TrimSpace(tag); tag != "" {
			sTags = append(sTags, tag)
		}
	}
	return sTags
}

type encodedInfo struct {
	Version int     `json:"v"` // version number of the license key info format (not Sourcegraph product/build version)
	Nonce   [8]byte `json:"n"` // random nonce so that licenses with identical Info values
	Info
}

func (l Info) Version() int {
	// Before version 2, SalesforceSubscriptionID was not yet added.
	if l.SalesforceSubscriptionID == nil {
		return 1
	}
	// Before version 3, CreatedAt was not yet added.
	if l.CreatedAt.IsZero() {
		return 2
	}
	return 3
}

func (l Info) encode() ([]byte, error) {
	e := encodedInfo{Version: l.Version(), Info: l}
	if _, err := rand.Read(e.Nonce[:8]); err != nil {
		return nil, err
	}
	return json.Marshal(e)
}

//nolint:unused // used in tests
func (l *Info) decode(data []byte) error {
	var e encodedInfo
	if err := json.Unmarshal(data, &e); err != nil {
		return err
	}
	if e.Version != e.Info.Version() {
		return errors.Errorf("license key format is version %d, expected version %d", e.Version, e.Info.Version())
	}
	*l = e.Info
	return nil
}

type signedKey struct {
	Signature   *ssh.Signature `json:"sig"`
	EncodedInfo []byte         `json:"info"`
}

// GenerateSignedKey generates a new signed license key with the given license information, using
// the private key for the signature.
func GenerateSignedKey(info Info, privateKey ssh.Signer) (licenseKey string, version int, err error) {
	encodedInfo, err := info.encode()
	if err != nil {
		return "", 0, errors.Wrap(err, "encode")
	}
	sig, err := privateKey.Sign(rand.Reader, encodedInfo)
	if err != nil {
		return "", 0, errors.Wrap(err, "sign")
	}
	signedKeyData, err := json.Marshal(signedKey{Signature: sig, EncodedInfo: encodedInfo})
	if err != nil {
		return "", 0, errors.Wrap(err, "marshal")
	}
	return base64.RawURLEncoding.EncodeToString(signedKeyData), info.Version(), nil
}

// ParseSignedKey parses and verifies the signed license key. If parsing or verification fails, a
// non-nil error is returned.
func ParseSignedKey(text string, publicKey ssh.PublicKey) (info *Info, signature string, err error) {
	// Ignore whitespace, in case the license key was (e.g.) wrapped in an email message.
	text = strings.Map(func(c rune) rune {
		if unicode.IsSpace(c) {
			return -1 // drop
		}
		return c
	}, text)

	signedKeyData, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil {
		return nil, "", err
	}
	var signedKey signedKey
	if err := json.Unmarshal(signedKeyData, &signedKey); err != nil {
		return nil, "", err
	}
	if err := json.Unmarshal(signedKey.EncodedInfo, &info); err != nil {
		return nil, "", err
	}
	return info, string(signedKey.Signature.Blob), publicKey.Verify(signedKey.EncodedInfo, signedKey.Signature)
}
