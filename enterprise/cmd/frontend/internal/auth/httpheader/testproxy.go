// The testproxy command runs a simple HTTP proxy that wraps a Sourcegraph server running with the
// http-header auth provider to test the authentication HTTP proxy support.

// +build ignore

package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

var (
	addr           = flag.String("addr", ":4080", "HTTP listen address")
	urlStr         = flag.String("url", "http://localhost:3080", "proxy origin URL (Sourcegraph HTTP/HTTPS URL)") // CI:LOCALHOST_OK
	username       = flag.String("username", os.Getenv("USER"), "username to report to Sourcegraph")
	usernamePrefix = flag.String("usernamePrefix", "", "prefix to place in front of username in the auth header value")
	httpHeader     = flag.String("header", "X-Forwarded-User", "name of HTTP header to add to request")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	url, err := url.Parse(*urlStr)
	if err != nil {
		log.Fatalf("Error: Invalid -url: %s.", err)
	}
	if *username == "" {
		log.Fatal("Error: No -username specified.")
	}
	if *httpHeader == "" {
		log.Fatal("Error: No -header specified.")
	}
	headerVal := *usernamePrefix + *username
	log.Printf(`Listening on %s, forwarding requests to %s with added header "%s: %s"`, *addr, url, *httpHeader, headerVal)
	p := httputil.NewSingleHostReverseProxy(url)
	log.Fatalf("Server error: %s.", http.ListenAndServe(*addr, &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.Header.Set(*httpHeader, headerVal)
			r.Host = url.Host
			p.Director(r)
		},
	}))
}
