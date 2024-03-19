package authbearer

import (
	"net/http"
	"strings"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func ExtractBearer(h http.Header) (string, error) {
	if authHeader := h.Get("Authorization"); authHeader != "" {
		return ExtractBearerContents(authHeader)
	}
	return "", nil
}

func ExtractBearerContents(s string) (string, error) {
	if s == "" {
		return "", errors.New("no token provided in Authorization header")
	}
	typ := strings.SplitN(s, " ", 2)
	if len(typ) != 2 {
		return "", errors.New("token type missing in Authorization header")
	}
	if strings.ToLower(typ[0]) != "bearer" {
		return "", errors.Newf("invalid token type %s in Authorization header", typ[0])
	}
	return typ[1], nil
}
