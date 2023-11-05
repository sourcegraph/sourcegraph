// Package osscmd defines entrypoint functions for the OSS build of Sourcegraph's single-binary
// distribution. It is invoked by all OSS commands' main functions.
package osscmd

import (
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/service/svcmain"
)

var config = svcmain.Config{}

// MainOSS is called from the `main` function of the `cmd/sourcegraph` command.
func MainOSS(services []service.Service, args []string) {
	svcmain.Main(services, config, args)
}

// SingleServiceMainOSS is called from the `main` function of a command in the OSS build
// to start a single service (such as frontend or gitserver).
func SingleServiceMainOSS(service service.Service) {
	svcmain.SingleServiceMain(service, config)
}
