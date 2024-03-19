//go:build tools
// +build tools

package main

import (
	// go-mockgen is used to codegen mockable interfaces, used in precise code intel tests
	_ "github.com/derision-test/go-mockgen/v2/cmd/go-mockgen"
)
