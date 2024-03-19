package main

import (
	"github.com/sourcegraph/sourcegraph/cmd/telemetry-gateway/service"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
)

func main() {
	runtime.Start(&service.Service{})
}
