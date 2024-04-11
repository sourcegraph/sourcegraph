// gitserverproxy is the coordinator and router for gitserver shards.
package main

import (
	"github.com/sourcegraph/sourcegraph/cmd/gitserverproxy/shared"
	"github.com/sourcegraph/sourcegraph/internal/sanitycheck"
	"github.com/sourcegraph/sourcegraph/internal/service/svcmain"
)

func main() {
	sanitycheck.Pass()
	svcmain.SingleServiceMain(shared.Service)
}
