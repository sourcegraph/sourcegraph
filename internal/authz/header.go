package authz

import (
	"bytes"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const (
	SchemeToken     = "token"      // Scheme for Authorization header with only an access token
	SchemeTokenSudo = "token-sudo" // Scheme for Authorization header with access token and sudo user
)

// errUnrecognizedScheme occurs when the Authorization header scheme (the first token) is not
// recognized.
var errUnrecognizedScheme = errors.Errorf("unrecognized HTTP Authorization request header scheme (supported values: %q, %q)", SchemeToken, SchemeTokenSudo)

// IsUnrecognizedScheme reports whether err indicates that the request's Authorization header scheme
// is unrecognized or unparseable (i.e., is neither "token" nor "token-sudo").
func IsUnrecognizedScheme(err error) bool {
	return errors.IsAny(err, errUnrecognizedScheme, errHTTPAuthParamsDuplicateKey, errHTTPAuthParamsNoEquals)
}

// ParseAuthorizationHeader parses the HTTP Authorization request header for supported credentials
// values.
//
// Two forms of the Authorization header's "credentials" token are supported (see [RFC 7235,
// Appendix C](https://tools.ietf.org/html/rfc7235#appendix-C):
//
//   - With only an access token: "token" 1*SP token68
//   - With a token as params:
//     "token" 1*SP "token" BWS "=" BWS quoted-string
//
// The returned values are derived directly from user input and have not been validated or
// authenticated.
func ParseAuthorizationHeader(headerValue string) (token, sudoUser string, err error) {
	scheme, token68, params, err := parseHTTPCredentials(headerValue)
	if err != nil {
		return "", "", err
	}

	if scheme != SchemeToken && scheme != SchemeTokenSudo {
		return "", "", errUnrecognizedScheme
	}

	if token68 != "" {
		switch scheme {
		case SchemeToken:
			return token68, "", nil
		case SchemeTokenSudo:
			return "", "", errors.New(`HTTP Authorization request header value must be of the following form: token="TOKEN",user="USERNAME"`)
		}
	}

	if !dotcom.SiteAdminCanViewAllUserData() && scheme == SchemeTokenSudo {
		return "", "", errors.New("use of access tokens with sudo scope is disabled")
	}

	token = params["token"]
	if token == "" {
		return "", "", errors.New("no token value in the HTTP Authorization request header")
	}
	sudoUser = params["user"]
	return token, sudoUser, nil
}

// ParseBearerHeader parses the HTTP Authorization request header for a bearer token.
func ParseBearerHeader(authHeader string) (string, error) {
	typ := strings.SplitN(authHeader, " ", 2)
	if len(typ) != 2 {
		return "", errors.New("token type missing in Authorization header")
	}
	if strings.ToLower(typ[0]) != "bearer" {
		return "", errors.Newf("invalid token type %s", typ[0])
	}

	return typ[1], nil
}

// parseHTTPCredentials parses the "credentials" token as defined in [RFC 7235 Appendix
// C](https://tools.ietf.org/html/rfc7235#appendix-C).
func parseHTTPCredentials(credentials string) (scheme, token68 string, params map[string]string, err error) {
	parts := strings.SplitN(credentials, " ", 2)
	scheme = parts[0]
	if len(parts) == 1 {
		return scheme, "", nil, nil
	}

	params, err = parseHTTPAuthParams(parts[1])
	if err == errHTTPAuthParamsNoEquals {
		// Likely just a token68.
		token68 = parts[1]
		return scheme, token68, nil, nil
	}
	if err != nil {
		return "", "", nil, err
	}

	return scheme, "", params, nil
}

// parseHTTPAuthParams extracts key/value pairs from a comma-separated list of auth-params as defined
// in [RFC 7235, Appendix C](https://tools.ietf.org/html/rfc7235#appendix-C) and returns a map.
//
// The resulting values are unquoted. The keys are matched case-insensitively, and each key MUST
// only occur once per challenge (according to [RFC 7235, Section
// 2.1](https://tools.ietf.org/html/rfc7235#section-2.1)).
func parseHTTPAuthParams(value string) (params map[string]string, err error) {
	// Implementation derived from
	// https://code.google.com/p/gorilla/source/browse/http/parser/parser.go.
	params = make(map[string]string)
	for _, pair := range parseHTTPHeaderList(strings.TrimSpace(value)) {
		i := strings.Index(pair, "=")
		if i < 0 || strings.HasSuffix(pair, "=") {
			return nil, errHTTPAuthParamsNoEquals
		}
		v := pair[i+1:]
		if v[0] == '"' && v[len(v)-1] == '"' {
			// Unquote it.
			v = v[1 : len(v)-1]
		}
		key := strings.ToLower(pair[:i])
		if _, seen := params[key]; seen {
			return nil, errHTTPAuthParamsDuplicateKey
		}
		params[key] = v
	}
	return params, nil
}

var (
	errHTTPAuthParamsDuplicateKey = errors.New("duplicate key in HTTP auth-params")
	errHTTPAuthParamsNoEquals     = errors.New("invalid HTTP auth-params list (parameter has no value)")
)

// parseHTTPHeaderList parses a "#rule" as defined in [RFC 2068 Section
// 2.1](https://tools.ietf.org/html/rfc2068#section-2.1).
func parseHTTPHeaderList(value string) []string {
	// Implementation derived from from
	// https://code.google.com/p/gorilla/source/browse/http/parser/parser.go which was ported from
	// urllib2.parse_http_list, from the Python standard library.

	var list []string
	var escape, quote bool
	b := new(bytes.Buffer)
	for _, r := range value {
		switch {
		case escape:
			b.WriteRune(r)
			escape = false
		case quote:
			if r == '\\' {
				escape = true
			} else {
				if r == '"' {
					quote = false
				}
				b.WriteRune(r)
			}
		case r == ',':
			list = append(list, strings.TrimSpace(b.String()))
			b.Reset()
		case r == '"':
			quote = true
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	// Append last part.
	if s := b.String(); s != "" {
		list = append(list, strings.TrimSpace(s))
	}
	return list
}
