package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/google/go-github/v55/github"
	"github.com/jackc/pgx/v5"
	"github.com/urfave/cli/v2"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/db"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	connections "github.com/sourcegraph/sourcegraph/internal/database/connections/live"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/runner"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/schemas"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/store"
	"github.com/sourcegraph/sourcegraph/internal/database/postgresdsn"
	"github.com/sourcegraph/sourcegraph/internal/encryption"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/cliutil/exit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var (
	dbDatabaseNameFlag string

	dbCommand = &cli.Command{
		Name:  "db",
		Usage: "Interact with local Sourcegraph databases for development",
		UsageText: `
# Delete test databases
sg db delete-test-dbs

# Reset the Sourcegraph 'frontend' database
sg db reset-pg

# Reset the 'frontend' and 'codeintel' databases
sg db reset-pg -db=frontend,codeintel

# Reset all databases ('frontend', 'codeintel', 'codeinsights')
sg db reset-pg -db=all

# Reset the redis database
sg db reset-redis

# Create a site-admin user whose email and password are foo@sourcegraph.com and sourcegraph.
sg db add-user -username=foo

# Create an access token for the user created above.
sg db add-access-token -username=foo
`,
		Category: category.Dev,
		Subcommands: []*cli.Command{
			{
				Name:   "delete-test-dbs",
				Usage:  "Drops all databases that have the prefix `sourcegraph-test-`",
				Action: deleteTestDBsExec,
			},
			{
				Name:        "reset-pg",
				Usage:       "Drops, recreates and migrates the specified Sourcegraph database",
				Description: `If -db is not set, then the "frontend" database is used (what's set as PGDATABASE in env or the sg.config.yaml). If -db is set to "all" then all databases are reset and recreated.`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "db",
						Value:       db.DefaultDatabase.Name,
						Usage:       "The target database instance.",
						Destination: &dbDatabaseNameFlag,
					},
				},
				Action: dbResetPGExec,
			},
			{
				Name:      "reset-redis",
				Usage:     "Drops, recreates and migrates the specified Sourcegraph Redis database",
				UsageText: "sg db reset-redis",
				Action:    dbResetRedisExec,
			},
			{
				Name:        "update-user-external-services",
				Usage:       "Manually update a user's external services",
				Description: `Patches the table 'user_external_services' with a custom OAuth token for the provided user. Used in dev/test environments. Set PGDATASOURCE to a valid connection string to patch an external database.`,
				Action:      dbUpdateUserExternalAccount,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "sg.username",
						Value: "sourcegraph",
						Usage: "Username of the user account on Sourcegraph",
					},
					&cli.StringFlag{
						Name:  "extsvc.display-name",
						Value: "",
						Usage: "The display name of the GitHub instance connected to the Sourcegraph instance (as listed under Site admin > Manage code hosts)",
					},
					&cli.StringFlag{
						Name:  "github.username",
						Value: "sourcegraph",
						Usage: "Username of the account on the GitHub instance",
					},
					&cli.StringFlag{
						Name:  "github.token",
						Value: "",
						Usage: "GitHub token with a scope to read all user data",
					},
					&cli.StringFlag{
						Name:  "github.baseurl",
						Value: "",
						Usage: "The base url of the GitHub instance to connect to",
					},
					&cli.StringFlag{
						Name:  "github.client-id",
						Value: "",
						Usage: "The client ID of an OAuth app on the GitHub instance",
					},
					&cli.StringFlag{
						Name:  "oauth.token",
						Value: "",
						Usage: "OAuth token to patch for the provided user",
					},
				},
			},
			{
				Name:        "add-user",
				Usage:       "Create an admin sourcegraph user",
				Description: `Run 'sg db add-user -username bob' to create an admin user whose email is bob@sourcegraph.com. The password will be printed if the operation succeeds`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "username",
						Value: "sourcegraph",
						Usage: "Username for user",
					},
					&cli.StringFlag{
						Name:  "password",
						Value: "sourcegraphsourcegraph",
						Usage: "Password for user",
					},
				},
				Action: dbAddUserAction,
			},

			{
				Name:        "add-access-token",
				Usage:       "Create a sourcegraph access token",
				Description: `Run 'sg db add-access-token -username bob' to create an access token for the given username. The access token will be printed if the operation succeeds`,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "username",
						Value: "sourcegraph",
						Usage: "Username for user",
					},
					&cli.BoolFlag{
						Name:     "sudo",
						Value:    false,
						Usage:    "Set true to make a site-admin level token",
						Required: false,
					},
					&cli.StringFlag{
						Name:     "note",
						Value:    "",
						Usage:    "Note attached to the token",
						Required: false,
					},
				},
				Action: dbAddAccessTokenAction,
			},
		},
	}
)

func dbAddUserAction(cmd *cli.Context) error {
	ctx := cmd.Context
	logger := log.Scoped("dbAddUserAction")

	// Read the configuration.
	conf, _ := getConfig()
	if conf == nil {
		return errors.New("failed to read sg.config.yaml. This command needs to be run in the `sourcegraph` repository")
	}

	// Connect to the database.
	conn, err := connections.EnsureNewFrontendDB(&observation.TestContext, postgresdsn.New("", "", conf.GetEnv), "frontend")
	if err != nil {
		return err
	}

	db := database.NewDB(logger, conn)
	return db.WithTransact(ctx, func(tx database.DB) error {
		username := cmd.String("username")
		password := cmd.String("password")

		// Create the user, generating an email based on the username.
		email := fmt.Sprintf("%s@sourcegraph.com", username)
		user, err := tx.Users().Create(ctx, database.NewUser{
			Username:        username,
			Email:           email,
			EmailIsVerified: true,
			Password:        password,
		})
		if err != nil {
			return err
		}

		// Make the user site admin.
		err = tx.Users().SetIsSiteAdmin(ctx, user.ID, true)
		if err != nil {
			return err
		}

		// Report back the new user information.
		std.Out.WriteSuccessf(
			fmt.Sprintf("User '%[1]s%[3]s%[2]s' (%[1]s%[4]s%[2]s) has been created and its password is '%[1]s%[5]s%[6]s'.",
				output.StyleOrange,
				output.StyleSuccess,
				username,
				email,
				password,
				output.StyleReset,
			),
		)

		return nil
	})
}

func dbAddAccessTokenAction(cmd *cli.Context) error {
	ctx := cmd.Context
	logger := log.Scoped("dbAddAccessTokenAction")

	// Read the configuration.
	conf, _ := getConfig()
	if conf == nil {
		return errors.New("failed to read sg.config.yaml. This command needs to be run in the `sourcegraph` repository")
	}

	// Connect to the database.
	conn, err := connections.EnsureNewFrontendDB(&observation.TestContext, postgresdsn.New("", "", conf.GetEnv), "frontend")
	if err != nil {
		return err
	}

	db := database.NewDB(logger, conn)
	return db.WithTransact(ctx, func(tx database.DB) error {
		username := cmd.String("username")
		sudo := cmd.Bool("sudo")
		note := cmd.String("note")

		scopes := []string{"user:all"}
		if sudo {
			scopes = []string{"site-admin:sudo"}
		}

		// Fetch user
		user, err := tx.Users().GetByUsername(ctx, username)
		if err != nil {
			return err
		}

		// Generate the token
		_, token, err := tx.AccessTokens().Create(ctx, user.ID, scopes, note, user.ID)
		if err != nil {
			return err
		}

		// Print token
		std.Out.WriteSuccessf("New token created: %q", token)
		return nil
	})
}

func dbUpdateUserExternalAccount(cmd *cli.Context) error {
	logger := log.Scoped("dbUpdateUserExternalAccount")
	ctx := cmd.Context
	username := cmd.String("sg.username")
	serviceName := cmd.String("extsvc.display-name")
	ghUsername := cmd.String("github.username")
	token := cmd.String("github.token")
	baseurl := cmd.String("github.baseurl")
	clientID := cmd.String("github.client-id")
	oauthToken := cmd.String("oauth.token")

	// Read the configuration.
	conf, _ := getConfig()
	if conf == nil {
		return errors.New("failed to read sg.config.yaml. This command needs to be run in the `sourcegraph` repository")
	}

	// Connect to the database.
	conn, err := connections.EnsureNewFrontendDB(&observation.TestContext, postgresdsn.New("", "", conf.GetEnv), "frontend")
	if err != nil {
		return err
	}
	db := database.NewDB(logger, conn)

	// Find the service
	services, err := db.ExternalServices().List(ctx, database.ExternalServicesListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to list services")
	}
	var service *types.ExternalService
	for _, s := range services {
		if s.DisplayName == serviceName {
			service = s
		}
	}
	if service == nil {
		return errors.Newf("cannot find service whose display name is %q", serviceName)
	}

	// Get URL from the external service config
	serviceConfigString, err := service.Config.Decrypt(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to decrypt external service config")
	}
	serviceConfigMap := make(map[string]any)
	if err = json.Unmarshal([]byte(serviceConfigString), &serviceConfigMap); err != nil {
		return errors.Wrap(err, "failed to unmarshal service config JSON")
	}
	if serviceConfigMap["url"] == nil {
		return errors.New("failed to find url in external service config")
	}
	// Add trailing slash to the URL if missing
	serviceID, err := url.JoinPath(serviceConfigMap["url"].(string), "/")
	if err != nil {
		return errors.Wrap(err, "failed to create external service ID url")
	}

	// Find the user
	user, err := db.Users().GetByUsername(ctx, username)
	if err != nil {
		return errors.Wrap(err, "failed to get user")
	}

	ghc, err := githubClient(ctx, baseurl, token)
	if err != nil {
		return errors.Wrap(err, "failed to authenticate on the github instance")
	}

	ghUser, _, err := ghc.Users.Get(ctx, ghUsername)
	if err != nil {
		return errors.Wrap(err, "failed to fetch github user")
	}

	authData, err := newAuthData(oauthToken)
	if err != nil {
		return errors.Wrap(err, "failed to generate oauth data")
	}

	logger.Info("Writing external account to the DB")

	_, err = db.UserExternalAccounts().Upsert(
		ctx,
		&extsvc.Account{
			UserID: user.ID,
			AccountSpec: extsvc.AccountSpec{
				ServiceType: strings.ToLower(service.Kind),
				ServiceID:   serviceID,
				ClientID:    clientID,
				AccountID:   fmt.Sprintf("%d", ghUser.GetID()),
			},
			AccountData: extsvc.AccountData{
				AuthData: authData,
				Data:     nil,
			},
		},
	)
	return err
}

type authdata struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Expiry      string `json:"expiry"`
}

func newAuthData(accessToken string) (*encryption.JSONEncryptable[any], error) {
	raw, err := json.Marshal(authdata{
		AccessToken: accessToken,
		TokenType:   "bearer",
		Expiry:      "0001-01-01T00:00:00Z",
	})
	if err != nil {
		return nil, err
	}

	return extsvc.NewUnencryptedData(raw), nil
}

func githubClient(ctx context.Context, baseurl string, token string) (*github.Client, error) {
	tc := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))

	baseURL, err := url.Parse(baseurl)
	if err != nil {
		return nil, err
	}
	baseURL.Path = "/api/v3"

	gh, err := github.NewClient(tc).WithEnterpriseURLs(baseURL.String(), baseURL.String())
	if err != nil {
		return nil, err
	}
	return gh, nil
}

func dbResetRedisExec(ctx *cli.Context) error {
	// Read the configuration.
	config, _ := getConfig()
	if config == nil {
		return errors.New("failed to read sg.config.yaml. This command needs to be run in the `sourcegraph` repository")
	}

	// Connect to the redis database.
	endpoint := config.GetEnv("REDIS_ENDPOINT")
	conn, err := redis.Dial("tcp", endpoint, redis.DialConnectTimeout(5*time.Second))
	if err != nil {
		return errors.Wrapf(err, "failed to connect to Redis at %s", endpoint)
	}

	// Drop everything in redis
	_, err = conn.Do("flushall")
	if err != nil {
		return errors.Wrap(err, "failed to run command on redis")
	}

	return nil
}

func deleteTestDBsExec(ctx *cli.Context) error {
	config, err := dbtest.GetDSN()
	if err != nil {
		return err
	}
	dsn := config.String()

	db, err := dbconn.ConnectInternal(log.Scoped("sg"), dsn, "", "")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if closeErr := db.Close(); closeErr != nil {
				err = errors.Append(err, closeErr)
			}
		}
	}()

	names, err := basestore.ScanStrings(db.QueryContext(ctx.Context, `SELECT datname FROM pg_database WHERE datname LIKE 'sourcegraph-test-%'`))
	if err != nil {
		return err
	}

	for _, name := range names {
		_, err := db.ExecContext(ctx.Context, fmt.Sprintf(`DROP DATABASE %q`, name))
		if err != nil {
			return err
		}

		std.Out.WriteLine(output.Linef(output.EmojiOk, output.StyleReset, fmt.Sprintf("Deleted %s", name)))
	}

	std.Out.WriteLine(output.Linef(output.EmojiSuccess, output.StyleSuccess, fmt.Sprintf("%d databases deleted.", len(names))))
	return nil
}

func dbResetPGExec(ctx *cli.Context) error {
	// Read the configuration.
	config, _ := getConfig()
	if config == nil {
		return errors.New("failed to read sg.config.yaml. This command needs to be run in the `sourcegraph` repository")
	}

	var (
		dsnMap      = map[string]string{}
		schemaNames []string
	)

	if dbDatabaseNameFlag == "all" {
		schemaNames = schemas.SchemaNames
	} else {
		schemaNames = strings.Split(dbDatabaseNameFlag, ",")
	}

	for _, name := range schemaNames {
		if name == "frontend" {
			dsnMap[name] = postgresdsn.New("", "", config.GetEnv)
		} else {
			dsnMap[name] = postgresdsn.New(strings.ToUpper(name), "", config.GetEnv)
		}
	}

	std.Out.WriteNoticef("This will reset database(s) %s%s%s. Are you okay with this?",
		output.StyleOrange, strings.Join(schemaNames, ", "), output.StyleReset)
	if ok := getBool(); !ok {
		return exit.NewEmptyExitErr(1)
	}

	for _, dsn := range dsnMap {
		var (
			db  *pgx.Conn
			err error
		)

		db, err = pgx.Connect(ctx.Context, dsn)
		if err != nil {
			return errors.Wrap(err, "failed to connect to Postgres database")
		}

		_, err = db.Exec(ctx.Context, "DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
		if err != nil {
			std.Out.WriteFailuref("Failed to drop schema 'public': %s", err)
			return err
		}

		if err := db.Close(ctx.Context); err != nil {
			return err
		}
	}

	storeFactory := func(db *sql.DB, migrationsTable string) connections.Store {
		return connections.NewStoreShim(store.NewWithDB(&observation.TestContext, db, migrationsTable))
	}
	r, err := connections.RunnerFromDSNs(std.Out.Output, log.Scoped("migrations.runner"), dsnMap, "sg", storeFactory)
	if err != nil {
		return err
	}

	operations := make([]runner.MigrationOperation, 0, len(schemaNames))
	for _, schemaName := range schemaNames {
		operations = append(operations, runner.MigrationOperation{
			SchemaName: schemaName,
			Type:       runner.MigrationOperationTypeUpgrade,
		})
	}

	if err := r.Run(ctx.Context, runner.Options{
		Operations: operations,
	}); err != nil {
		return err
	}

	std.Out.WriteSuccessf("Database(s) reset!")
	return nil
}
