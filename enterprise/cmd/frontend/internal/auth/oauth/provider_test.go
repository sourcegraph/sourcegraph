package oauth

import (
	"net/url"
	"testing"
)

func TestCanRedirect(t *testing.T) {
	tc := map[string]bool{
		"https://evilhost.com/nasty-stuff":  false,
		"/search?foo=bar":                   true,
		"http://example.com/search?foo=bar": true,
		"http://localhost:1111/oh-dear":     false,
	}
	for tURL, expected := range tc {
		t.Run(tURL, func(t *testing.T) {
			got := canRedirect(url.PathEscape(tURL))
			if got != expected {
				t.Errorf("Expected %t got %t", expected, got)
			}
		})
	}
}
