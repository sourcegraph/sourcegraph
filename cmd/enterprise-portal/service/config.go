package service

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	sams "github.com/sourcegraph/sourcegraph-accounts-sdk-go"
	"github.com/sourcegraph/sourcegraph/cmd/enterprise-portal/internal/routines/licenseexpiration"
	"github.com/sourcegraph/sourcegraph/internal/codygateway/codygatewayevents"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/cloudsql"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Config is the configuration for the Enterprise Portal.
type Config struct {
	DotComDB struct {
		cloudsql.ConnConfig

		PGDSNOverride *string

		IncludeProductionLicenses bool

		ImportInterval time.Duration
	}

	// If nil, no connection was configured.
	CodyGatewayEvents *codygatewayevents.ServiceBigQueryOptions

	SAMS SAMSConfig

	// Configuration specific to 'ENTERPRISE_SUBSCRIPTION_LICENSE_TYPE_KEY'
	LicenseKeys struct {
		// Signer is the private key used to generate license keys.
		Signer ssh.Signer
		// RequiredTags are the tags required on all licenses created in this
		// Enterprise Portal instance.
		RequiredTags []string
	}

	LicenseExpirationChecker licenseexpiration.Config
}

type SAMSConfig struct {
	sams.ConnConfig
	ClientID     string
	ClientSecret string
}

func (c *Config) Load(env *runtime.Env) {
	c.DotComDB.ConnConfig = cloudsql.ConnConfig{
		ConnectionName: env.GetOptional("DOTCOM_CLOUDSQL_CONNECTION_NAME",
			"Sourcegraph.com Cloud SQL connection name"),
		User:     env.GetOptional("DOTCOM_CLOUDSQL_USER", "Sourcegraph.com Cloud SQL user"),
		Database: env.Get("DOTCOM_CLOUDSQL_DATABASE", "sourcegraph", "Sourcegraph.com database"),
	}
	c.DotComDB.PGDSNOverride = env.GetOptional("DOTCOM_PGDSN_OVERRIDE",
		"For local dev: custom PostgreSQL DSN, overrides DOTCOM_CLOUDSQL_* options")
	c.DotComDB.IncludeProductionLicenses = env.GetBool("DOTCOM_INCLUDE_PRODUCTION_LICENSES", "false",
		"Include production licenses in API results")
	c.DotComDB.ImportInterval = env.GetInterval("DOTCOM_IMPORT_INTERVAL", "10m",
		"Interval at which to import data from Sourcegraph.com")

	c.SAMS.ConnConfig = sams.NewConnConfigFromEnv(env)
	c.SAMS.ClientID = env.Get("ENTERPRISE_PORTAL_SAMS_CLIENT_ID", "",
		"Sourcegraph Accounts Management System client ID")
	c.SAMS.ClientSecret = env.Get("ENTERPRISE_PORTAL_SAMS_CLIENT_SECRET", "",
		"Sourcegraph Accounts Management System client secret")

	codyGatewayEventsProjectID := env.GetOptional("CODY_GATEWAY_EVENTS_PROJECT_ID",
		"Project ID for Cody Gateway events ('telligentsourcegraph' or 'cody-gateway-dev')")
	codyGatewayEventsDataset := env.Get("CODY_GATEWAY_EVENTS_DATASET", "cody_gateway",
		"Dataset for Cody Gateway events")
	codyGatewayEventsTable := env.Get("CODY_GATEWAY_EVENTS_TABLE", "events",
		"Table for Cody Gateway events")
	if codyGatewayEventsProjectID != nil {
		c.CodyGatewayEvents = &codygatewayevents.ServiceBigQueryOptions{
			ProjectID:   pointers.DerefZero(codyGatewayEventsProjectID),
			Dataset:     codyGatewayEventsDataset,
			EventsTable: codyGatewayEventsTable,
		}
	}

	c.LicenseKeys.Signer = func() ssh.Signer {
		// We use a unconventional env name here to align with existing usages
		// of this key, for convenience.
		privateKey := env.GetOptional("SOURCEGRAPH_LICENSE_GENERATION_KEY",
			fmt.Sprintf("The PEM-encoded form of the private key used to sign product license keys (%s)",
				license.GenerationPrivateKeyURL))
		if privateKey == nil {
			// Not having this just disables the generation of new licenses, it
			// does not block startup.
			return nil
		}
		signer, err := ssh.ParsePrivateKey([]byte(*privateKey))
		if err != nil {
			env.AddError(errors.Wrap(err,
				"Failed to parse private key in SOURCEGRAPH_LICENSE_GENERATION_KEY env var"))
		}
		return signer
	}()
	c.LicenseKeys.RequiredTags = func() []string {
		tags := env.GetOptional("LICENSE_KEY_REQUIRED_TAGS",
			"Comma-delimited list of tags required on all license keys generated on this Enterprise Portal instance")
		if tags == nil {
			return nil
		}
		return strings.Split(*tags, ",")
	}()

	c.LicenseExpirationChecker.Interval = env.GetOptionalInterval(
		"LICENSE_EXPIRATION_CHECKER_INTERVAL",
		"Interval at which to run license expiration checks. If not set, checks are not run.")
	c.LicenseExpirationChecker.SlackWebhookURL = env.GetOptional(
		"LICENSE_EXPIRATION_CHECKER_SLACK_WEBHOOK_URL",
		"Destination webhook for expired licenses. If not set, messages are logged.",
	)
}
