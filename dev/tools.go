//go:build tools
// +build tools

package main

import (
	// zoekt-* used in sourcegraph/server docker image build
	_ "github.com/sourcegraph/zoekt/cmd/zoekt-archive-index"
	_ "github.com/sourcegraph/zoekt/cmd/zoekt-git-index"
	_ "github.com/sourcegraph/zoekt/cmd/zoekt-sourcegraph-indexserver"
	_ "github.com/sourcegraph/zoekt/cmd/zoekt-webserver"

	// go-mockgen is used to codegen mockable interfaces, used in precise code intel tests
	_ "github.com/derision-test/go-mockgen/v2/cmd/go-mockgen"

	// used in schema pkg
	_ "github.com/sourcegraph/go-jsonschema/cmd/go-jsonschema-compiler"

	_ "golang.org/x/tools/cmd/goimports"
	// used in many places
	_ "golang.org/x/tools/cmd/stringer"

	// Used for cody-gateway to generate a GraphQL client
	_ "github.com/Khan/genqlient"

	// Used to generate schema
	_ "github.com/pseudomuto/protoc-gen-doc"
)
