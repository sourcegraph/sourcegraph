package saml

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/beevik/etree"
	saml2 "github.com/russellhaering/gosaml2"
	"github.com/russellhaering/gosaml2/types"
	dsig "github.com/russellhaering/goxmldsig"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

const providerType = "saml"

type provider struct {
	config     schema.SAMLAuthProvider
	multiple   bool // whether there are multiple SAML auth providers
	httpClient httpcli.Doer

	mu         sync.Mutex
	samlSP     *saml2.SAMLServiceProvider
	refreshErr error
}

// ConfigID implements providers.Provider.
func (p *provider) ConfigID() providers.ConfigID {
	return providers.ConfigID{
		Type: providerType,
		ID:   providerConfigID(&p.config, p.multiple),
	}
}

// Config implements providers.Provider.
func (p *provider) Config() schema.AuthProviders {
	return schema.AuthProviders{Saml: &p.config}
}

// Refresh implements providers.Provider.
func (p *provider) Refresh(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.samlSP, p.refreshErr = getServiceProvider(ctx, &p.config, p.httpClient)
	return p.refreshErr
}

func (p *provider) ExternalAccountInfo(ctx context.Context, account extsvc.Account) (*extsvc.PublicAccountData, error) {
	return GetPublicExternalAccountData(ctx, &account.AccountData)
}

func providerIDQuery(pc *schema.SAMLAuthProvider, multiple bool) url.Values {
	if multiple {
		return url.Values{"pc": []string{providerConfigID(pc, multiple)}}
	}
	return url.Values{}
}

func (p *provider) getCachedInfoAndError() (*providers.Info, error) {
	info := providers.Info{
		DisplayName: p.config.DisplayName,
		AuthenticationURL: (&url.URL{
			Path:     path.Join(auth.AuthURLPrefix, "saml", "login"),
			RawQuery: providerIDQuery(&p.config, p.multiple).Encode(),
		}).String(),
	}
	if info.DisplayName == "" {
		info.DisplayName = "SAML"
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.refreshErr
	if err != nil {
		err = errors.WithMessage(err, "failed to initialize SAML Service Provider")
	} else if p.samlSP == nil {
		err = errors.New("SAML Service Provider is not yet initialized")
	}
	if p.samlSP != nil {
		info.ServiceID = p.samlSP.IdentityProviderIssuer
		info.ClientID = p.samlSP.ServiceProviderIssuer
	}
	return &info, err
}

// CachedInfo implements providers.Provider.
func (p *provider) CachedInfo() *providers.Info {
	info, _ := p.getCachedInfoAndError()
	return info
}

func getServiceProvider(ctx context.Context, pc *schema.SAMLAuthProvider, httpClient httpcli.Doer) (*saml2.SAMLServiceProvider, error) {
	c, err := readProviderConfig(pc)
	if err != nil {
		return nil, err
	}

	idpMetadata, err := readIdentityProviderMetadata(ctx, c, httpClient)
	if err != nil {
		return nil, err
	}
	{
		if c.identityProviderMetadataURL != nil {
			traceLog(fmt.Sprintf("Identity Provider metadata: %s", c.identityProviderMetadataURL), string(idpMetadata))
		}
	}

	metadata, err := unmarshalEntityDescriptor(idpMetadata)
	if err != nil {
		return nil, errors.WithMessage(err, "parsing SAML Identity Provider metadata")
	}

	sp := saml2.SAMLServiceProvider{
		IdentityProviderSSOURL:  metadata.IDPSSODescriptor.SingleSignOnServices[0].Location,
		IdentityProviderIssuer:  metadata.EntityID,
		NameIdFormat:            getNameIDFormat(pc),
		SkipSignatureValidation: pc.InsecureSkipAssertionSignatureValidation,
		ValidateEncryptionCert:  true,
		AllowMissingAttributes:  true,
	}

	idpCertStore := &dsig.MemoryX509CertificateStore{Roots: []*x509.Certificate{}}
	for _, kd := range metadata.IDPSSODescriptor.KeyDescriptors {
		for i, xcert := range kd.KeyInfo.X509Data.X509Certificates {
			if xcert.Data == "" {
				return nil, errors.Errorf("SAML Identity Provider metadata certificate %d is empty", i)
			}
			certData, err := base64.StdEncoding.DecodeString(xcert.Data)
			if err != nil {
				return nil, errors.WithMessage(
					err,
					fmt.Sprintf("decoding SAML Identity Provider metadata certificate %d", i),
				)
			}
			idpCert, err := x509.ParseCertificate(certData)
			if err != nil {
				return nil, errors.WithMessage(
					err,
					fmt.Sprintf("parsing SAML Identity Provider metadata certificate %d X.509 data", i),
				)
			}
			idpCertStore.Roots = append(idpCertStore.Roots, idpCert)
		}
	}
	sp.IDPCertificateStore = idpCertStore

	// The SP's signing and encryption keys.
	if c.keyPair != nil {
		sp.SPKeyStore = dsig.TLSCertKeyStore(*c.keyPair)
		sp.SignAuthnRequests = pc.SignRequests == nil || *pc.SignRequests
	} else if pc.SignRequests != nil && *pc.SignRequests {
		// If the SP private key isn't specified, then the IdP must not care to validate.
		return nil, errors.New("signRequests is true for SAML Service Provider but no private key and cert are given")
	}

	// pc.Issuer's default of ${externalURL}/.auth/saml/metadata already applied (in withConfigDefaults).
	sp.ServiceProviderIssuer = pc.ServiceProviderIssuer
	if pc.ServiceProviderIssuer == "" {
		return nil, errors.New(
			"invalid SAML Service Provider configuration: issuer is empty (and default issuer could not be derived from empty externalURL)",
		)
	}
	externalURL, err := url.Parse(conf.Get().ExternalURL)
	if err != nil {
		return nil, errors.WithMessage(err, "parsing external URL for SAML Service Provider")
	}
	sp.AssertionConsumerServiceURL = externalURL.ResolveReference(&url.URL{Path: path.Join(authPrefix, "acs")}).String()
	sp.AudienceURI = sp.ServiceProviderIssuer

	return &sp, nil
}

// entitiesDescriptor represents the SAML EntitiesDescriptor object.
type entitiesDescriptor struct {
	XMLName             xml.Name       `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntitiesDescriptor"`
	ID                  *string        `xml:",attr,omitempty"`
	ValidUntil          *time.Time     `xml:"validUntil,attr,omitempty"`
	CacheDuration       *time.Duration `xml:"cacheDuration,attr,omitempty"`
	Name                *string        `xml:",attr,omitempty"`
	Signature           *etree.Element
	EntitiesDescriptors []entitiesDescriptor     `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntitiesDescriptor"`
	EntityDescriptors   []types.EntityDescriptor `xml:"urn:oasis:names:tc:SAML:2.0:metadata EntityDescriptor"`
}

// unmarshalEntityDescriptor unmarshals from an XML root <EntityDescriptor> or <EntitiesDescriptor>
// element. If the latter, it returns the first <EntityDescriptor> child that has an
// IDPSSODescriptor.
//
// Taken from github.com/crewjam/saml.
func unmarshalEntityDescriptor(data []byte) (*types.EntityDescriptor, error) {
	var entity *types.EntityDescriptor
	if err := xml.Unmarshal(data, &entity); err != nil {
		// This comparison is ugly, but it is how the error is generated in encoding/xml.
		if err.Error() != "expected element type <EntityDescriptor> but have <EntitiesDescriptor>" {
			return nil, err
		}
		var entities *entitiesDescriptor
		if err := xml.Unmarshal(data, &entities); err != nil {
			return nil, err
		}
		for i, e := range entities.EntityDescriptors {
			if e.IDPSSODescriptor != nil {
				entity = &entities.EntityDescriptors[i]
				break
			}
		}
		if entity == nil {
			return nil, errors.New("no entity found with IDPSSODescriptor")
		}
	}
	return entity, nil
}

type providerConfig struct {
	keyPair *tls.Certificate

	// Exactly 1 of these is set:
	identityProviderMetadataURL *url.URL
	identityProviderMetadata    []byte
}

func readProviderConfig(pc *schema.SAMLAuthProvider) (*providerConfig, error) {
	var c providerConfig

	if pc.ServiceProviderCertificate != "" && pc.ServiceProviderPrivateKey != "" {
		keyPair, err := tls.X509KeyPair([]byte(pc.ServiceProviderCertificate), []byte(pc.ServiceProviderPrivateKey))
		if err != nil {
			return nil, err
		}
		keyPair.Leaf, err = x509.ParseCertificate(keyPair.Certificate[0])
		if err != nil {
			return nil, err
		}
		c.keyPair = &keyPair
	}

	// Allow specifying either URL to SAML Identity Provider metadata XML file, or the XML
	// file contents directly.
	switch {
	case pc.IdentityProviderMetadataURL != "" && pc.IdentityProviderMetadata != "":
		return nil, errors.New(
			"invalid SAML configuration: set either identityProviderMetadataURL or identityProviderMetadata, not both",
		)

	case pc.IdentityProviderMetadataURL != "":
		var err error
		c.identityProviderMetadataURL, err = url.Parse(pc.IdentityProviderMetadataURL)
		if err != nil {
			return nil, errors.Wrap(err, "parsing SAML Identity Provider metadata URL")
		}

	case pc.IdentityProviderMetadata != "":
		c.identityProviderMetadata = []byte(pc.IdentityProviderMetadata)

	default:
		return nil, errors.New(
			"invalid SAML configuration: must provide the SAML metadata, using either identityProviderMetadataURL (URL where XML file is available) or identityProviderMetadata (XML file contents)",
		)
	}

	return &c, nil
}

func readIdentityProviderMetadata(ctx context.Context, c *providerConfig, httpClient httpcli.Doer) ([]byte, error) {
	if c.identityProviderMetadata != nil {
		return c.identityProviderMetadata, nil
	}

	req, err := http.NewRequest("GET", c.identityProviderMetadataURL.String(), nil)
	if err != nil {
		return nil, errors.WithMessage(err, "bad URL")
	}

	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, errors.WithMessage(err, "fetching SAML Identity Provider metadata")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf(
			"non-200 HTTP response for SAML Identity Provider metadata URL: %s",
			c.identityProviderMetadataURL,
		)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.WithMessage(err, "reading SAML Identity Provider metadata")
	}
	return data, nil
}
