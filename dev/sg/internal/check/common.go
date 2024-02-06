package check

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/jackc/pgx/v4"
	"github.com/sourcegraph/run"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/sgconf"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/usershell"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/internal/database/postgresdsn"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// checkPostgresConnection succeeds connecting to the default user database works, regardless
// of if it's running locally or with docker.
func PostgresConnection(ctx context.Context) error {
	dsns, err := dsnCandidates()
	if err != nil {
		return err
	}
	var errs []error
	for _, dsn := range dsns {
		// If any of the candidates succeed, we're good
		if err := pingPG(ctx, dsn); err == nil {
			return nil
		} else {
			errs = append(errs, err)
		}
	}

	messages := []string{"failed all attempts to connect to Postgresql database"}
	for _, e := range errs {
		messages = append(messages, "\t"+e.Error())
	}
	return errors.New(strings.Join(messages, "\n"))
}

// Attempts to connect to the given Postgresql database.
// if the database is starting up, it will wait until it's ready.
func pingPG(ctx context.Context, dsn string) error {
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return errors.Wrapf(err, "failed to connect to Postgresql Database at %s", dsn)
	}
	defer conn.Close(ctx)

	for {
		err := conn.Ping(ctx)
		if err == nil {
			return nil
		}
		// If database is starting up we keep waiting
		if strings.Contains(err.Error(), "database system is starting up") {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		return errors.Wrapf(err, "failed to ping database at %s", dsn)
	}
}

func dsnCandidates() ([]string, error) {
	var candidates []string

	// most classic dsn
	baseURL := url.URL{Scheme: "postgres", Host: "127.0.0.1:5432"}

	env := func(key string) string { val, _ := os.LookupEnv(key); return val }
	add := func(dsn string) { candidates = append(candidates, dsn) }

	withUserPass := func(user, password string) func(dsn url.URL) {
		return func(dsn url.URL) {
			if password == "" {
				dsn.User = url.User(user)
			} else {
				dsn.User = url.UserPassword(user, password)
			}
		}
	}

	withPath := func(path string) func(dsn url.URL) {
		return func(dsn url.URL) {
			dsn.Path = path
		}
	}

	withSSL := func(sslmode string) func(dsn url.URL) {
		return func(dsn url.URL) {
			if sslmode != "" {
				qry := dsn.Query()
				qry.Set("sslmode", sslmode)
				dsn.RawQuery = qry.Encode()
			}
		}
	}

	withHost := func(host, port string) func(dsn url.URL) {
		return func(dsn url.URL) {
			if host == "" {
				return
			}
			if port == "" {
				port = "5432"
			}
			dsn.Host = fmt.Sprintf("%s:%s", host, port)
		}
	}

	addURL := func(modifiers ...func(dsn url.URL)) {
		dsn := baseURL
		for _, modifier := range modifiers {
			modifier(dsn)
		}
		add(dsn.String())
	}

	// best case scenario
	add(env("PGDATASOURCE"))

	// homebrew dsn
	if uinfo, err := user.Current(); err == nil {
		addURL(
			withUserPass(uinfo.Username, ""),
			withPath("postgres"),
		)
	}

	// classic docker dsn
	addURL(withUserPass("postgres", "postgres"))
	// other classic docker dsn
	addURL(withUserPass("postgres", "password"))

	// env based dsn
	username, ok := os.LookupEnv("PGUSER")
	if !ok {
		uinfo, err := user.Current()
		if err != nil {
			return nil, err
		}
		username = uinfo.Username
	}

	addURL(
		withUserPass(username, env("PGPASSWORD")),
		withHost(env("PGHOST"), env("PGPORT")),
		withSSL(env("PGSSLMODE")),
	)

	return candidates, nil
}

func SourcegraphDatabase(getConfig func() (*sgconf.Config, error)) CheckFunc {
	return func(ctx context.Context) error {
		// This check runs only in the `sourcegraph/sourcegraph` repository, so
		// we try to parse the globalConf and use its `Env` to configure the
		// Postgres connection.
		config, err := getConfig()
		if err != nil {
			return err
		}
		if config == nil {
			return errors.New("failed to read sg.config.yaml. This step of `sg setup` needs to be run in the `sourcegraph` repository")
		}

		getEnv := func(key string) string {
			// First look into process env, emulating the logic in makeEnv used
			// in internal/run/run.go
			val, ok := os.LookupEnv(key)
			if ok {
				return val
			}
			// Otherwise check in globalConf.Env
			return config.Env[key]
		}

		dsn := postgresdsn.New("", "", getEnv)

		if err := pingPG(ctx, dsn); err != nil {
			return errors.Wrapf(err, "failed to connect to Sourcegraph Postgres database at %s. Please check the settings in sg.config.yml (see https://docs.sourcegraph.com/dev/background-information/sg#changing-database-configuration)", dsn)
		}
		return nil
	}
}

var Redis = Retry(checkRedisConnection, 5, 500*time.Millisecond)

func checkRedisConnection(context.Context) error {
	conn, err := redis.Dial("tcp", ":6379", redis.DialConnectTimeout(5*time.Second))
	if err != nil {
		return errors.Wrap(err, "failed to connect to Redis at 127.0.0.1:6379")
	}

	if _, err := conn.Do("SET", "sg-setup", "was-here"); err != nil {
		return errors.Wrap(err, "failed to write to Redis at 127.0.0.1:6379")
	}

	retval, err := redis.String(conn.Do("GET", "sg-setup"))
	if err != nil {
		return errors.Wrap(err, "failed to read from Redis at 127.0.0.1:6379")
	}

	if retval != "was-here" {
		return errors.New("failed to test write in Redis")
	}
	return nil
}

var Git = Combine(InPath("git"), checkGitVersion(">= 2.42.0"))

func checkGitVersion(versionConstraint string) func(context.Context) error {
	return func(ctx context.Context) error {
		out, err := usershell.Command(ctx, "git version").StdOut().Run().String()
		if err != nil {
			return errors.Wrapf(err, "failed to run 'git version'")
		}

		elems := strings.Split(out, " ")
		if len(elems) != 3 && len(elems) != 5 {
			return errors.Newf("unexpected output from git: %s", out)
		}

		trimmed := strings.TrimSpace(elems[2])
		return Version("git", trimmed, versionConstraint)
	}
}

// func checkPostgresVersion(dsn, versionConstraint string) func(context.Context) error {
// 	return func(ctx context.Context) error {
// 		out, err := usershell.Command(ctx, `psql -t -c "select version()"`).StdOut().Run().String()
// 		if err != nil {
// 			return errors.Wrapf(err, "failed to get postgres version")
// 		}

// 		version := majorMinorVersionRegex.FindString(out)
// 		if version == "" {
// 			return errors.Newf("unexpected output from postgres: %s", out)
// 		}

// 		return Version("postgres", version, versionConstraint)
// 	}
// }

func getToolVersionConstraint(ctx context.Context, tool string) (string, error) {
	tools, err := root.Run(run.Cmd(ctx, "cat .tool-versions")).Lines()
	if err != nil {
		return "", errors.Wrap(err, "Read .tool-versions")
	}
	var version string
	for _, t := range tools {
		parts := strings.Split(t, " ")
		if parts[0] == tool {
			version = parts[1]
			break
		}
	}
	if version == "" {
		return "", errors.Newf("tool %q not found in .tool-versions", tool)
	}
	return fmt.Sprintf("~> %s", version), nil
}

var PNPM = Combine(InPath("pnpm"), checkPnpmVersion)

func checkPnpmVersion(ctx context.Context) error {
	if err := InPath("pnpm")(ctx); err != nil {
		return err
	}

	constraint, err := getPackageManagerConstraint("pnpm")
	if err != nil {
		return err
	}

	cmd := "pnpm --version"
	data, err := usershell.Command(ctx, cmd).StdOut().Run().String()
	if err != nil {
		return errors.Wrapf(err, "failed to run %q", cmd)
	}
	trimmed := strings.TrimSpace(data)
	if len(trimmed) == 0 {
		return errors.Newf("no output from %q", cmd)
	}

	return Version("pnpm", trimmed, constraint)
}

func getPackageManagerConstraint(tool string) (string, error) {
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return "", errors.Wrap(err, "Failed to determine repository root location")
	}

	jsonFile, err := os.Open(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		return "", errors.Wrap(err, "Open package.json")
	}
	defer jsonFile.Close()

	jsonData, err := io.ReadAll(jsonFile)
	if err != nil {
		return "", errors.Wrap(err, "Read package.json")
	}

	data := struct {
		PackageManager string `json:"packageManager"`
	}{}

	if err := json.Unmarshal(jsonData, &data); err != nil {
		return "", errors.Wrap(err, "Unmarshal package.json")
	}

	var version string
	parts := strings.Split(data.PackageManager, "@")
	if parts[0] == tool {
		version = parts[1]
	}

	if version == "" {
		return "", errors.Newf("pnpm version is not found in package.json")
	}

	return fmt.Sprintf("~> %s", version), nil
}

var Go = Combine(InPath("go"), checkGoVersion)

func checkGoVersion(ctx context.Context) error {
	if err := InPath("go")(ctx); err != nil {
		return err
	}

	constraint, err := getToolVersionConstraint(ctx, "golang")
	if err != nil {
		return err
	}

	cmd := "go version"
	data, err := usershell.Command(ctx, cmd).StdOut().Run().String()
	if err != nil {
		return errors.Wrapf(err, "failed to run %q", cmd)
	}
	parts := strings.Split(strings.TrimSpace(data), " ")
	if len(parts) == 0 {
		return errors.Newf("no output from %q", cmd)
	}

	return Version("go", strings.TrimPrefix(parts[2], "go"), constraint)
}

var Node = Combine(InPath("node"), checkNodeVersion, CommandOutputContains(`node -e "console.log(\"foobar\")"`, "foobar"))

func checkNodeVersion(ctx context.Context) error {
	constraint, err := getToolVersionConstraint(ctx, "nodejs")
	if err != nil {
		return err
	}

	cmd := "node --version"
	data, err := usershell.Run(ctx, cmd).Lines()
	if err != nil {
		return errors.Wrapf(err, "failed to run %q", cmd)
	}
	trimmed := strings.TrimSpace(data[len(data)-1])
	if len(trimmed) == 0 {
		return errors.Newf("no output from %q", cmd)
	}

	return Version("nodejs", trimmed, constraint)
}

var Rust = Combine(InPath("cargo"), checkRustVersion)

func checkRustVersion(ctx context.Context) error {
	constraint, err := getToolVersionConstraint(ctx, "rust")
	if err != nil {
		return err
	}

	cmd := "cargo --version"
	data, err := usershell.Command(ctx, cmd).StdOut().Run().String()
	if err != nil {
		return errors.Wrapf(err, "failed to run %q", cmd)
	}
	parts := strings.Split(strings.TrimSpace(data), " ")
	if len(parts) == 0 {
		return errors.Newf("no output from %q", cmd)
	}

	return Version("cargo", parts[1], constraint)
}

var Docker = WrapErrMessage(
	Combine(InPath("docker"), CommandExitCode("docker info", 0)),
	`Docker needs to be running.
If Docker is installed and the check fails, you might need to restart your terminal and 'sg setup --fix'`)

var ASDF = CommandOutputContains("asdf", "version")

var Python = Combine(InPath("python"), checkPythonVersion)

func checkPythonVersion(ctx context.Context) error {
	constraint, err := getToolVersionConstraint(ctx, "python")
	if err != nil {
		return err
	}

	cmd := "python --version"
	data, err := usershell.Command(ctx, cmd).StdOut().Run().String()
	if err != nil {
		return errors.Wrapf(err, "failed to run %q", cmd)
	}
	parts := strings.Split(strings.TrimSpace(data), " ")
	if len(parts) == 0 {
		return errors.Newf("no output from %q", cmd)
	}

	return Version("python", parts[1], constraint)
}

var Bazelisk = WrapErrMessage(Combine(InPath("bazel"), CommandOutputContains("bazel version", "Bazelisk version")), "sg setup --fix")

func Caddy(_ context.Context) error {
	certPath, err := caddySourcegraphCertificatePath()
	if err != nil {
		return errors.Wrap(err, "failed to determine path where proxy stores certificates")
	}

	ok, err := pathExists(certPath)
	if !ok || err != nil {
		return errors.New("sourcegraph.test certificate not found. highly likely it's not trusted by system")
	}

	rawCert, err := os.ReadFile(certPath)
	if err != nil {
		return errors.Wrap(err, "could not read certificate")
	}

	cert, err := pemDecodeSingleCert(rawCert)
	if err != nil {
		return errors.Wrap(err, "decoding cert failed")
	}

	if trusted(cert) {
		return nil
	}
	return errors.New("doesn't look like certificate is trusted")
}

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// caddyAppDataDir returns the location of the sourcegraph.test certificate
// that Caddy created or would create.
//
// It's copy&pasted&modified from here: https://sourcegraph.com/github.com/caddyserver/caddy@9ee68c1bd57d72e8a969f1da492bd51bfa5ed9a0/-/blob/storage.go?L114
func caddySourcegraphCertificatePath() (string, error) {
	if basedir := os.Getenv("XDG_DATA_HOME"); basedir != "" {
		return filepath.Join(basedir, "caddy"), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var appDataDir string
	switch runtime.GOOS {
	case "darwin":
		appDataDir = filepath.Join(home, "Library", "Application Support", "Caddy")
	case "linux":
		appDataDir = filepath.Join(home, ".local", "share", "caddy")
	default:
		return "", errors.Newf("unsupported OS: %s", runtime.GOOS)
	}

	return filepath.Join(appDataDir, "pki", "authorities", "local", "root.crt"), nil
}

func trusted(cert *x509.Certificate) bool {
	chains, err := cert.Verify(x509.VerifyOptions{})
	return len(chains) > 0 && err == nil
}

func pemDecodeSingleCert(pemDER []byte) (*x509.Certificate, error) {
	pemBlock, _ := pem.Decode(pemDER)
	if pemBlock == nil {
		return nil, errors.Newf("no PEM block found")
	}
	if pemBlock.Type != "CERTIFICATE" {
		return nil, errors.Newf("expected PEM block type to be CERTIFICATE, but got '%s'", pemBlock.Type)
	}
	return x509.ParseCertificate(pemBlock.Bytes)
}
