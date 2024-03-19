package main

import (
	"context"
	"os"
	"strconv"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/audit"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

func main() {
	args := os.Args

	// binary name, audit logs count
	if len(args) != 2 {
		os.Exit(-1)
	}

	ctx := context.Background()

	callbacks := log.Init(log.Resource{
		Name:       "Audit Resource",
		Namespace:  "Audit Integration Testing",
		Version:    "",
		InstanceID: "",
	})

	defer callbacks.Sync()

	logger := log.Scoped("test")

	logsCount, err := strconv.Atoi(os.Args[1])
	if err != nil {
		os.Exit(-1)
	}

	// audit log depends on site config, but a mock is sufficient
	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{
		Log: &schema.Log{
			AuditLog: &schema.AuditLog{
				InternalTraffic: true,
				GitserverAccess: true,
				GraphQL:         true,
				SeverityLevel:   "INFO",
			},
		},
	}})
	defer conf.Mock(nil)

	for range logsCount {
		audit.Log(ctx, logger, audit.Record{
			Entity: "integration test",
			Action: "sampling testing",
			Fields: nil,
		})
	}
}
