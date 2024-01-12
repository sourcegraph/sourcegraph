package rds

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds/rdsutils"
	"github.com/jackc/pgx/v5"
	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Updater implements the dbconn.ConnectionUpdater interface
// for getting an auth token for RDS IAM auth.
// It will retrieve an auth token from EC2 metadata service and
// update the connection config with the token as the password.
type Updater struct{}

func NewUpdater() *Updater {
	return &Updater{}
}

func (u *Updater) ShouldUpdate(cfg *pgx.ConnConfig) bool {
	logger := log.Scoped("rds")
	token, err := parseRDSAuthToken(cfg.Password)
	if err != nil {
		logger.Warn("Error parsing RDS auth token, refreshing", log.Error(err))
		return true
	}

	return token.isExpired(time.Now().UTC())
}

func (u *Updater) Update(cfg *pgx.ConnConfig) (*pgx.ConnConfig, error) {
	logger := log.Scoped("rds")
	if cfg.Password != "" {
		// only output the warning once, or it will emit a new entry on every connection
		sync.OnceFunc(func() {
			logger.Warn("'PG_CONNECTION_UPDATER' is 'EC2_ROLE_CREDENTIALS', but 'PGPASSWORD' is also set. Ignoring 'PGPASSWORD'.")
		})
	}

	logger.Debug("Updating RDS IAM auth token")
	token, err := authToken(cfg.Host, cfg.Port, cfg.User)
	if err != nil {
		return nil, errors.Wrap(err, "Error getting auth token for RDS IAM auth")
	}
	cfg.Password = token
	logger.Debug("Updated RDS IAM auth token")

	return cfg, nil
}

// authToken attempts to load the AWS credentials from the current environment
// and use the credentials to generate an auth token for RDS IAM auth.
// In production, this usually means you have an EC2 instance role attached to the
// pod service account, and the credentials is retrieved via IMDSv2.
func authToken(hostname string, port uint16, user string) (string, error) {
	instance, err := parseRDSHostname(hostname)
	if err != nil {
		return "", errors.Wrap(err, "Error parsing RDS hostname")
	}

	sess, err := session.NewSession()
	if err != nil {
		return "", errors.Wrap(err, "Error creating AWS session for RDS IAM auth")
	}
	creds := sess.Config.Credentials
	if creds == nil {
		return "", errors.New("No AWS credentials found from current session")
	}

	// no network call is made to get the auth token,
	// it's similar to how s3 signed url works
	// it uses the credentials to sign and attach the signature to the string
	// and it will be presented to RDS as database password for authentication
	authToken, err := rdsutils.BuildAuthToken(
		fmt.Sprintf("%s:%d", instance.hostname, port),
		instance.region, user, creds)
	if err != nil {
		return "", errors.Wrap(err, "Error building auth token for RDS IAM auth")
	}

	return authToken, nil
}

// rdsAuthToken represents the auth token for RDS IAM auth.
// Learn more from unit test cases
type rdsAuthToken struct {
	IssuedAt  time.Time
	ExpiresIn time.Duration
}

// parseRDSAuthToken parses the auth token from RDS IAM auth.
// Learn more from unit test cases
func parseRDSAuthToken(token string) (*rdsAuthToken, error) {
	u, err := url.Parse(fmt.Sprintf("https://%s", token))
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing RDS auth token")
	}

	// specific about the query string parameters can be found from:
	// https://docs.aws.amazon.com/AmazonS3/latest/API/sigv4-query-string-auth.html
	xAmzDate := u.Query().Get("X-Amz-Date")
	if xAmzDate == "" {
		return nil, errors.New("Missing X-Amz-Date in RDS auth token, <redacted>")
	}
	// e.g., 20160801T223241Z
	issuedAt, err := time.Parse("20060102T150405Z", xAmzDate)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing X-Amz-Date in RDS auth token, %q", xAmzDate)
	}

	xAmzExpires := u.Query().Get("X-Amz-Expires")
	if xAmzExpires == "" {
		return nil, errors.New("Missing X-Amz-Expires in RDS auth token, <redacted>")
	}
	expiresIn, err := strconv.Atoi(xAmzExpires)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing X-Amz-Expires in RDS auth token, %q", xAmzExpires)
	}

	return &rdsAuthToken{
		IssuedAt:  issuedAt,
		ExpiresIn: time.Duration(expiresIn) * time.Second,
	}, nil
}

// isExpired returns true if the token is expired
// with a 5 minutes grace period.
func (t *rdsAuthToken) isExpired(now time.Time) bool {
	// 300 secs buffer to avoid the token being expired when it is used
	return now.UTC().Add(-300 * time.Second).After(t.IssuedAt.Add(t.ExpiresIn))
}

type rdsInstance struct {
	region   string
	hostname string
}

// parseRDSHostname parses the RDS hostname and returns the region and instance name.
// It is in the form of <instance-name>.<account-id>.<region>.rds.amazonaws.com
// e.g., postgresmydb.123456789012.us-east-1.rds.amazonaws.com
func parseRDSHostname(name string) (*rdsInstance, error) {
	if !strings.HasSuffix(name, ".rds.amazonaws.com") {
		return nil, errors.Newf("not an RDS hostname, expecting '.rds.amazonaws.com' suffix, %q", name)
	}

	parts := strings.Split(name, ".")
	if len(parts) != 6 {
		return nil, errors.Newf("unexpected RDS hostname format, %q", name)
	}

	if parts[0] == "" {
		return nil, errors.Newf("unexpected instance name in RDS hostname format, %q", name)
	}

	if parts[1] == "" {
		return nil, errors.Newf("unexpected account ID in RDS hostname format, %q", name)
	}

	if parts[2] == "" {
		return nil, errors.Newf("unexpected region in RDS hostname format, %q", name)
	}

	return &rdsInstance{
		region:   parts[2],
		hostname: name,
	}, nil
}
