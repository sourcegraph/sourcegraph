package saml

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlidp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/external/session"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/auth/providers"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

const (
	testSAMLSPCert = `-----BEGIN CERTIFICATE-----
MIIC+zCCAeOgAwIBAgIQFkK4RCQNFkAFzj8dJHnXJjANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMB4XDTE4MTAyMDA4NTQ1NVoXDTI4MTAxNzA4NTQ1
NVowEjEQMA4GA1UEChMHQWNtZSBDbzCCASIwDQYJKoZIhvcNAQEBBQADggEPADCC
AQoCggEBAKMD2RmTnAI+1+s+hiakkdbOXHoEwRoG45yeCV5z8A7TnZtF238kReBN
JSOUTvgrvg5WbfG8ULSariepAI45BH3yYoNOBXe0biVsCB+0h6szeV1+N6y9wj0j
ns/AOOV6ec/GbUZufF+XeJmVX/kRoOthUCEWhCGn/ZCa9VNcr2u/EhCZhvk6JcY9
p/gu2YYJepihYpkrzzHwlC+ye+AfPX0/LiZQLGM8ciiziXden8DqEhskkg5HqnPl
hwscqI6qlYIcUFw5QB3xA738N4/92Uj7Jstf05ESFDf6zbUTn/hSsLXivNHI0G4P
4gsVy5Y5pygrw3b3FuodJbuVtLU9cwMCAwEAAaNNMEswDgYDVR0PAQH/BAQDAgWg
MBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQCMAAwFgYDVR0RBA8wDYIL
ZXhhbXBsZS5jb20wDQYJKoZIhvcNAQELBQADggEBADQ/UgbXlW7zPwWswJSlbgph
yjepD5dJ/My1ByIM2GSSYlvnLGq9tSOwUWZ0fZY/G8WOowNSBQlUVPT7tS11j7Ce
BdrImHWpDleZYyagk08vaU059LJFI4EM8Vzn570h3dxdXkSoIGjqNAfywat591k3
K8llPk2wrQ8fv3KA7tNNmJW+Ee1DHIAui53aFe3sHmp7JN1tE6HlqrSLIDymSd28
tOfJ1Y9kOvUF7DY8pkSVDukO9wsy0X568hfJOz4PQe/1LHJ1YxlomTCkyVV8xtW7
hbnEyvPo2yr/SHbk4Fz1yXP20cBm8vO2DmKlI0kaKGQw1Rybl8NQw+OPdb/V6pM=
-----END CERTIFICATE-----`

	testSAMLSPKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAowPZGZOcAj7X6z6GJqSR1s5cegTBGgbjnJ4JXnPwDtOdm0Xb
fyRF4E0lI5RO+Cu+DlZt8bxQtJquJ6kAjjkEffJig04Fd7RuJWwIH7SHqzN5XX43
rL3CPSOez8A45Xp5z8ZtRm58X5d4mZVf+RGg62FQIRaEIaf9kJr1U1yva78SEJmG
+Tolxj2n+C7Zhgl6mKFimSvPMfCUL7J74B89fT8uJlAsYzxyKLOJd16fwOoSGySS
Dkeqc+WHCxyojqqVghxQXDlAHfEDvfw3j/3ZSPsmy1/TkRIUN/rNtROf+FKwteK8
0cjQbg/iCxXLljmnKCvDdvcW6h0lu5W0tT1zAwIDAQABAoIBAC/t4LYhbVxHp+p1
zrGr72lN8Wi63x/M6L1SxgRsaCej1pIhvwCp5JWneQT2BSX4jn/er6LEsKH5XL0y
doRahVSWoJpkpTzl4wDDu7u+s6kFkGiJxMrYXDTntTj2FoR6Nzh86gIsWAsvGPln
LvmnUj4CtbGU0jKnFumedgUVmko+QmalghYrkc7dReprGJ6EDWNLGb0ASG9/R7iQ
NOKu17nK9W3yCWJc48SR8y9HWUEUqtKbsshJ6PewNNttsSC3JjeGuiH5fRmXLi6L
wXr2l1AAPGRWbI8djrm7DFLa1s8pfJKkTV0YUDHFNBXny0h+oUGwC+KCQNsfE3t8
GbKqdKkCgYEA0A3dFKuxzm9QbZRcmGwZbHTNfWb7EMlknNq9wqZLgbTK77P1bhXW
l0YP7HuNZnKToMt3UrM5tYFzk28a73p4Ur8+va23lbtwwBeZ5qsZ/vAfI9b2GTj5
AchOI5Xtwf8eWT3OIdHbe4W5hkyb/siPdkRJ1zXfDn8XN0+XIB7NeT0CgYEAyJTn
xjtxMq3Jab5tycZ7ZVC9Y5tpd6phb6KLdLNGNcNKSPvC2EichgLH3Awckpe92HvD
wujlQnlKod42/aPxVcOOnwqN2PMfzPXK91pcOt9QyWFzRLFtvkFB9Dd4vz5uhdWF
CpSBDN87PpFEOuJJApy78e7hfZ5YpxWGK7N24T8CgYEAxtT4+9A6VTc8ffzToTdt
9KCL4dSRDDHr3ZuOzn9umb7WUs6BN3vXYSqr/Sz2rXnCbGEG4Bo4hKX6dmQwMb2x
UCNFKrDiSk6gKnRjuHa8mU+R8wZ0mxY/otxzEL8wQb42msLeRKPyRdI+w4JjctLp
h/UrPGlXitsarNl7bE8Dv2ECgYBXOgog9rCfbVvtlFaCLMJ0qMvziR4wX/PHbFRh
B6U8tBSV8IYnMEyBKqxnUQ0L4tk4T3ouRMGOStjd05jubGEC/uwC1cAh3Hiz1R/S
uYTqRTsImExcTxx+ZDqeTZFA+ZFuuhAFLdeBFYLaDqoxQT6m2CoTZ+K/kiDTaFTU
pFLKWQKBgQCNCeMVMkNwJtMeR/vKUEOPZLKFZihPrORi8F5v0f+qrcg3bKDKd1KI
6kocCulZR3uvEFHUAoNyMNwCZs6YyIK9zEN3/Pb46ThnJNNMXv0CG+W8df/Vbnd0
REijBJT1tjS4dXBULokRuI2640iWll8KX1KQDzqo3l++JRGqoP57Sg==
-----END RSA PRIVATE KEY-----`
)

var (
	idpCert = func() *x509.Certificate {
		b, _ := pem.Decode([]byte(`-----BEGIN CERTIFICATE-----
MIIC/DCCAeSgAwIBAgIRANzQVAHz24KrifhcM9kaVcYwDQYJKoZIhvcNAQELBQAw
EjEQMA4GA1UEChMHQWNtZSBDbzAeFw0xODEwMjAwODU1NDNaFw0yODEwMTcwODU1
NDNaMBIxEDAOBgNVBAoTB0FjbWUgQ28wggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw
ggEKAoIBAQCYO2FXgBvsrHyDzFFjUxNuF5OtPYOw+yjGnMR7r1CU93ZaSFN0z9Ux
Qv6rHAmoGwLp+dJjYZ2g/km6/TnONVGjSDh8TxCEH+cP5kIyRN4L3MPW9tsZ36Fd
5yqNbVCrxp3gJKAHmcUHYONzQ6WxxOCEBkvVknysstG8hXhbOcElXrSIyRVPQuEu
TBQSAJahFbQYCKFU93rlO142hgPJDkHibz8PhLgEU7v3Eo23JrOSKNUysXnp5hLT
RhOyQyWNpXA91wwsTwETOD2KlDKDHIcpKEdMSWhRQya6S2z49RVKYpZmATW4Qq2l
hDWWuyG7/IaheuyPum+BF0FFmDKfQY83AgMBAAGjTTBLMA4GA1UdDwEB/wQEAwIF
oDATBgNVHSUEDDAKBggrBgEFBQcDATAMBgNVHRMBAf8EAjAAMBYGA1UdEQQPMA2C
C2V4YW1wbGUuY29tMA0GCSqGSIb3DQEBCwUAA4IBAQBvdi9Gz1qADI0F/oC/7fh/
TjevChAO3XsuGz53yqeM0z49yHJooCV3jzgBrjX82DAhk/nQ0kF+YZummoPG1fqf
rycmsq0yD7Gy/do8sW7XSvpQpkbiBb9yg7rP1eVEfn+vDv0ZS0F3mqbfXl1v7FQ0
PwPtE49OaO7rb3FbLPBtEXocvGjvga8SRmuT3/5oCI46AKldENL6+CKEEWWUIuW8
HeKPQIRYzcqi1dy88nRk44DkCyNxe/h7X/MGt4Mu9HjDH7lDJs0038sdghsX74ET
gN6hZwBR6U2UoJHDytj/+KtSL/XGZSwTgOFyFyMZROcqUPWRwl7Zk3dOKy+3T2Bi
-----END CERTIFICATE-----
`))
		c, _ := x509.ParseCertificate(b.Bytes)
		return c
	}()

	idpKey = func() crypto.PrivateKey {
		b, _ := pem.Decode([]byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAmDthV4Ab7Kx8g8xRY1MTbheTrT2DsPsoxpzEe69QlPd2WkhT
dM/VMUL+qxwJqBsC6fnSY2GdoP5Juv05zjVRo0g4fE8QhB/nD+ZCMkTeC9zD1vbb
Gd+hXecqjW1Qq8ad4CSgB5nFB2Djc0OlscTghAZL1ZJ8rLLRvIV4WznBJV60iMkV
T0LhLkwUEgCWoRW0GAihVPd65TteNoYDyQ5B4m8/D4S4BFO79xKNtyazkijVMrF5
6eYS00YTskMljaVwPdcMLE8BEzg9ipQygxyHKShHTEloUUMmukts+PUVSmKWZgE1
uEKtpYQ1lrshu/yGoXrsj7pvgRdBRZgyn0GPNwIDAQABAoIBAQCXS7zM5+fY6vy9
SJ1C59gRvKDqto5hoNy/uCKXAoBF7UPVKri3Ca/Ky9irWqxGRMI6pC1y1BuDW/cP
Pojq5pcCfs6UzUeO6N4OMTxtFYDRrVF+Hc1YA6gu2YazFIfukPFrSTs7Epp9YM/t
SLgu24p/7HoGAxah1P8aLFSX5eiOJ+8t8byYOrKLp3Rn67lC9Y+9LX4X6GHlBMDc
WHYupi3ZA7Q59dXQCJHFNG/hk17AMtB8lFra9rUid8teX8ZJKJQ26hU2O0UMujjM
mFlCdmvc97lJ4LhjrWHv/9yacf90bViHIkL52Yux1jNt/jl3/7CyBwHbau4b0qoZ
QkM4WIihAoGBAMlzsUeJxBCbUyMd/6TiLn60SDn0HMf4ZVdGqqxkhopVmsvRTn+P
wu9YHWFPwXwVL3wdtuBjsEA9nMKWWMQKbQUZhm1Y+AQIVpVNQqesgyLctVoIUBNY
fglvKrs8JuRuwMpE2P/3lXMsxtV9AyCpxxXhya8KqJa2jcMB/Lr+lx+fAoGBAMFz
16yHU+Zo6OOvy7T2xh67btwOrS2kIzhGO6gcK7qabkGGeaKLShnMYEmFGp4EaHTf
OVie+SU0OWF/e5bgFWC+fm6jWyhO0xPRbvs2P+l2KtnT2UBT9IgjhrVUIzp+Vn7t
cjfb32m7km1kZZ48ySP9cH/4/xnT6XEC33PoNwlpAoGAG1t+w7xNyAOP8sDsKrQc
pFBPTq98CRwOhx+tpeOw8bBWaT9vbZtUWbSZqNFv8S3fWPegEjD3ioHTfAl23Iid
7Ydd3hOq+sE3IOdxGdwvotheOG/QkBAAbb+PCgZNMdBolg9reLdisFVwWyWy+wiT
ZMFY5lCIPI9mCQmIDMzuMPkCgYBFJKJxh+z07YpP1wV4KLunQFbfUF+VcJUmB/RK
ocb/azL9OJNBBYf2sJW5sVlSIUE0hJR6mFd0dLYNowMJag46Bdwqrzhlr8bBzplc
MIenahTmxlFgLKG6Bvie1vPAdGd19mhcjrnLkL9FWhz38cHymyMammSTVqqZOe2j
/9usAQKBgQCT//j6XflAr20gb+mNcoJqVxRTFtSsZa23kJnZ3Sfs3R8XXu5NcZEZ
ODI9ZpZ9tg8oK9EB5vqMFTTnyjpar7F2jqFWtUmNju/rGlrQCZx0we+EAW/R2hFP
YGYu4Z+SyXTsv/Ys5VGWuuCJO32RuRBeC4eJCmpyH0mqPhIBZmV4Jw==
-----END RSA PRIVATE KEY-----
`))
		k, _ := x509.ParsePKCS1PrivateKey(b.Bytes)
		return k
	}()
)

// newSAMLIDPServer returns a new running SAML IDP server. It is the caller's
// responsibility to call Close().
func newSAMLIDPServer(t *testing.T) (*httptest.Server, *samlidp.Server) {
	h := http.NewServeMux()
	srv := httptest.NewServer(h)

	srvURL, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	idpServer, err := samlidp.New(samlidp.Options{
		URL:         *srvURL,
		Key:         idpKey,
		Certificate: idpCert,
		Store:       &samlidp.MemoryStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	h.Handle("/", idpServer)

	return srv, idpServer
}

func TestMiddleware(t *testing.T) {
	idpHTTPServer, idpServer := newSAMLIDPServer(t)
	defer idpHTTPServer.Close()

	defer licensing.TestingSkipFeatureChecks()()

	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			ExperimentalFeatures: &schema.ExperimentalFeatures{},
			ExternalURL:          "http://example.com",
		},
	})
	defer conf.Mock(nil)

	config := withConfigDefaults(&schema.SAMLAuthProvider{
		Type:                        "saml",
		IdentityProviderMetadataURL: idpServer.IDP.MetadataURL.String(),
		ServiceProviderCertificate:  testSAMLSPCert,
		ServiceProviderPrivateKey:   testSAMLSPKey,
	})

	cli, err := httpcli.NewFactory(nil).Client()
	require.NoError(t, err)

	mockGetProviderValue = &provider{config: *config, httpClient: cli}
	defer func() { mockGetProviderValue = nil }()
	providers.MockProviders = []providers.Provider{mockGetProviderValue}
	defer func() { providers.MockProviders = nil }()

	cleanup := session.ResetMockSessionStore(t)
	defer cleanup()

	providerID := providerConfigID(&mockGetProviderValue.config, true)

	// Mock user
	mockedExternalID := "testuser_id"
	const mockedUserID = 123
	auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (newUserCreated bool, userID int32, safeErrMsg string, err error) {
		if op.ExternalAccount.ServiceType == "saml" && op.ExternalAccount.ServiceID == idpServer.IDP.MetadataURL.String() && op.ExternalAccount.ClientID == "http://example.com/.auth/saml/metadata" && op.ExternalAccount.AccountID == mockedExternalID {
			return false, mockedUserID, "", nil
		}
		return false, 0, "safeErr", errors.Errorf("account %v not found in mock", op.ExternalAccount)
	}
	defer func() { auth.MockGetAndSaveUser = nil }()

	users := dbmocks.NewStrictMockUserStore()
	users.GetByIDFunc.SetDefaultHook(func(ctx context.Context, id int32) (*types.User, error) {
		return &types.User{ID: id, CreatedAt: time.Now()}, nil
	})

	db := dbmocks.NewStrictMockDB()
	db.UsersFunc.SetDefaultReturn(users)

	// Set up the test handler.
	authedHandler := http.NewServeMux()
	authedHandler.Handle("/.api/", Middleware(db).API(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid := actor.FromContext(r.Context()).UID; uid != mockedUserID && uid != 0 {
			t.Errorf("got actor UID %d, want %d", uid, mockedUserID)
		}
	})))
	authedHandler.Handle("/", Middleware(db).App(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte("This is the home"))
		case "/page":
			_, _ = w.Write([]byte("This is a page"))
		case "/require-authn":
			actr := actor.FromContext(r.Context())
			if actr.UID == 0 {
				t.Errorf("in authn expected-endpoint, no actor was set; expected actor with UID %d", mockedUserID)
			} else if actr.UID != mockedUserID {
				t.Errorf("in authn expected-endpoint, actor with incorrect UID was set; %d != %d", actr.UID, mockedUserID)
			}
			_, _ = w.Write([]byte("Authenticated"))
		default:
			http.Error(w, "", http.StatusNotFound)
		}
	})))

	// doRequest simulates a request to our authed handler (i.e., the SAML Service Provider).
	//
	// authed sets an authed actor in the request context to simulate an authenticated request.
	doRequest := func(method, urlStr, body string, cookies []*http.Cookie, authed bool, form url.Values) *http.Response {
		req := httptest.NewRequest(method, urlStr, bytes.NewBufferString(body))
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		if form != nil {
			req.PostForm = form
			req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
		}
		if authed {
			req = req.WithContext(actor.WithActor(context.Background(), &actor.Actor{UID: mockedUserID}))
		}
		respRecorder := httptest.NewRecorder()
		authedHandler.ServeHTTP(respRecorder, req)
		return respRecorder.Result()
	}

	var (
		authnRequest    saml.AuthnRequest
		authnCookies    []*http.Cookie
		authnRequestURL string
	)
	t.Run("unauthenticated homepage visit, no sign-out cookie -> IDP SSO URL", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/", "", nil, false, nil)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
		locURL, err := url.Parse(resp.Header.Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(locURL.String(), idpServer.IDP.SSOURL.String()) {
			t.Error("wrong redirect URL")
		}

		// save cookies and Authn request
		authnCookies = unexpiredCookies(resp)
		authnRequestURL = locURL.String()
		deflatedSAMLRequest, err := base64.StdEncoding.DecodeString(locURL.Query().Get("SAMLRequest"))
		if err != nil {
			t.Fatal(err)
		}
		if err := xml.NewDecoder(flate.NewReader(bytes.NewBuffer(deflatedSAMLRequest))).Decode(&authnRequest); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("unauthenticated homepage visit, sign-out cookie present -> sg login", func(t *testing.T) {
		cookie := &http.Cookie{Name: auth.SignOutCookie, Value: "true"}

		resp := doRequest("GET", "http://example.com/", "", []*http.Cookie{cookie}, false, nil)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got response code %v, want %v", resp.StatusCode, want)
		}
	})
	t.Run("unauthenticated API visit -> pass through", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.api/foo", "", nil, false, nil)
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Errorf("wrong response code: got %v, want %v", got, want)
		}
	})

	var loggedInCookies []*http.Cookie

	t.Run("get SP metadata and register SP with IDP", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.auth/saml/metadata?pc="+providerID, "", nil, false, nil)
		service := samlidp.Service{}
		if err := xml.NewDecoder(resp.Body).Decode(&service.Metadata); err != nil {
			t.Fatal(err)
		}
		serviceMetadataBytes, err := xml.Marshal(service.Metadata)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest("PUT", idpHTTPServer.URL+"/services/id", bytes.NewBuffer(serviceMetadataBytes))
		if err != nil {
			t.Fatal(err)
		}
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("could not register SP with IDP, error: %s, resp: %v", err, resp)
		}
		defer resp.Body.Close()
		if want := http.StatusNoContent; resp.StatusCode != want {
			t.Errorf("got HTTP %d, want %d", resp.StatusCode, want)
		}
	})

	t.Run("get SAML assertion from IDP and post the assertion to the SP ACS URL", func(t *testing.T) {
		authnReq, err := http.NewRequest("GET", authnRequestURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		idpAuthnReq, err := saml.NewIdpAuthnRequest(&idpServer.IDP, authnReq)
		if err != nil {
			t.Fatal(err)
		}
		if err := idpAuthnReq.Validate(); err != nil {
			t.Fatal(err)
		}
		samlSession := saml.Session{
			ID:         "session-id",
			CreateTime: time.Now(),
			ExpireTime: time.Now().Add(24 * time.Hour),
			Index:      "index",

			NameID:    "testuser_id",
			UserName:  "testuser_username",
			UserEmail: "testuser@email.com",
		}
		if err := (saml.DefaultAssertionMaker{}).MakeAssertion(idpAuthnReq, &samlSession); err != nil {
			t.Fatal(err)
		}
		if err := idpAuthnReq.MakeResponse(); err != nil {
			t.Fatal(err)
		}
		doc := etree.NewDocument()
		doc.SetRoot(idpAuthnReq.ResponseEl)
		responseBuf, err := doc.WriteToBytes()
		if err != nil {
			t.Fatal(err)
		}
		samlResponse := base64.StdEncoding.EncodeToString(responseBuf)
		reqParams := url.Values{}
		reqParams.Set("SAMLResponse", samlResponse)
		reqParams.Set("RelayState", idpAuthnReq.RelayState)
		resp := doRequest("POST", "http://example.com/.auth/saml/acs", "", authnCookies, false, reqParams)
		if want := http.StatusFound; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
		if got, want1, want2 := resp.Header.Get("Location"), "http://example.com/?signin=SAML", "/?signin=SAML"; got != want1 && got != want2 {
			t.Errorf("got redirect location %v, want %v or %v", got, want1, want2)
		}

		// save the cookies from the login response
		loggedInCookies = unexpiredCookies(resp)
	})
	t.Run("authenticated request to home page", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/", "", loggedInCookies, true, nil)
		respBody, _ := io.ReadAll(resp.Body)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
		if got, want := string(respBody), "This is the home"; got != want {
			t.Errorf("got response body %v, want %v", got, want)
		}
	})
	t.Run("authenticated request to sub page", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/page", "", loggedInCookies, true, nil)
		respBody, _ := io.ReadAll(resp.Body)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
		if got, want := string(respBody), "This is a page"; got != want {
			t.Errorf("got response body %v, want %v", got, want)
		}
	})
	t.Run("verify actor gets set in request context", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/require-authn", "", loggedInCookies, true, nil)
		if want := http.StatusOK; resp.StatusCode != want {
			t.Errorf("got status code %v, want %v", resp.StatusCode, want)
		}
	})
	t.Run("verify actor gets set in API request context", func(t *testing.T) {
		resp := doRequest("GET", "http://example.com/.api/foo", "", loggedInCookies, true, nil)
		if got, want := resp.StatusCode, http.StatusOK; got != want {
			t.Errorf("wrong status code: got %v, want %v", got, want)
		}
	})
}

func TestAllowSignin(t *testing.T) {
	allowedGroups := []string{"foo"}
	providerConfig := schema.SAMLAuthProvider{
		AllowGroups: allowedGroups,
	}
	mockProvider := &provider{
		config: providerConfig,
	}

	t.Run("Sign in is allowed if allowGroups is not configured", func(t *testing.T) {
		p := &provider{
			config: schema.SAMLAuthProvider{},
		}
		result := allowSignin(p, make(map[string]bool))
		if !result {
			t.Errorf("Expected allowSigning to be true, got %v", result)
		}
	})
	t.Run("Sign in is allowed if user belongs to a group", func(t *testing.T) {
		groups := make(map[string]bool)
		groups["foo"] = true
		result := allowSignin(mockProvider, groups)
		if !result {
			t.Errorf("Expected allowSigning to be true, got %v", result)
		}
	})
	t.Run("Sign in is not allowed if user does not belong to any group in allowGroups", func(t *testing.T) {
		groups := make(map[string]bool)
		groups["bar"] = true
		groups["baz"] = true
		result := allowSignin(mockProvider, groups)
		if result {
			t.Errorf("Expected allowSigning to be false, got %v", result)
		}
	})
	t.Run("Sign in is not allowed if allowGroups is empty", func(t *testing.T) {
		p := &provider{
			config: schema.SAMLAuthProvider{
				AllowGroups: []string{},
			},
		}
		result := allowSignin(p, make(map[string]bool))
		if result {
			t.Errorf("Expected allowSigning to be false, got %v", result)
		}
	})
	t.Run("Sign in is not allowed if user groups are empty", func(t *testing.T) {
		result := allowSignin(mockProvider, make(map[string]bool))
		if result {
			t.Errorf("Expected allowSigning to be false, got %v", result)
		}
	})
}

// unexpiredCookies returns the list of unexpired cookies set by the response
func unexpiredCookies(resp *http.Response) (cookies []*http.Cookie) {
	for _, cookie := range resp.Cookies() {
		if cookie.RawExpires == "" || cookie.Expires.After(time.Now()) {
			cookies = append(cookies, cookie)
		}
	}
	return
}
