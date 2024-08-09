//go:build ignore
// +build ignore

// Command generate-license generates a signed Sourcegraph license key.
//
// # REQUIREMENTS
//
// You must provide a private key to sign the license.
//
// To generate licenses that are valid for customer instances, you must use the private key at
// https://team-sourcegraph.1password.com/vaults/dnrhbauihkhjs5ag6vszsme45a/allitems/zkdx6gpw4uqejs3flzj7ef5j4i.
//
// To create a test private key that will NOT generate valid licenses, use:
//
//	openssl genrsa -out /tmp/key.pem 2048
//
// EXAMPLE
//
//	go run generate-license.go -private-key key.pem -tags=dev -users=100 -expires=8784h
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log" //nolint:logging // TODO move all logging to sourcegraph/log
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/sourcegraph/sourcegraph/internal/license"
)

var (
	privateKeyFile = flag.String("private-key", "", fmt.Sprintf("file containing private key to sign license (e.g. the production key at %s)", license.GenerationPrivateKeyURL))
	tags           = flag.String("tags", "", "comma-separated string tags to include in this license (e.g., \"starter,dev\")")
	users          = flag.Uint("users", 0, "maximum number of users allowed by this license (0 = no limit)")
	expires        = flag.Duration("expires", 0, "time until license expires (0 = no expiration)")
)

func main() {
	flag.Parse()
	log.SetFlags(0)

	log.Println("# License info (encoded and signed in license key)")
	info := license.Info{
		Tags:      license.ParseTagsInput(*tags),
		UserCount: *users,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().UTC().Round(time.Second).Add(*expires),
	}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(string(b))
	log.Println()

	log.Println("# License key")
	if *privateKeyFile == "" {
		log.Fatal("A private key file must be explicitly indicated, but was not.")
	}
	b, err = os.ReadFile(*privateKeyFile)
	if err != nil {
		log.Fatalf("Unable to read private key: %v\n", err)
	}
	privateKey, err := ssh.ParsePrivateKey(b)
	if err != nil {
		log.Fatal(err)
	}
	licenseKey, _, err := license.GenerateSignedKey(info, privateKey)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(licenseKey)
}
