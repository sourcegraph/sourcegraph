// Initially copy-pasta from https://github.com/sourcegraph/controller/blob/main/internal/cloudsqlproxy/proxy.go
package cloudsqlproxy

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sourcegraph/run"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/background"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// CloudSQLProxy is a cloud-sql-proxy instance
//
// It uses the identity of the service account to connect to the database
// and the proxy can handle the authentication for the database and refresh
// the credentials as needed.
type CloudSQLProxy struct {
	DBInstanceConnectionName  string
	ImpersonateServiceAccount string
	Port                      int

	// PermissionsHelpPageURL is the URL to show users for permissions issues.
	PermissionsHelpPageURL string
}

func NewCloudSQLProxy(dbConnection string, iamUserEmail string, port int, permissionsHelpPageURL string) (*CloudSQLProxy, error) {
	return &CloudSQLProxy{
		DBInstanceConnectionName:  dbConnection,
		ImpersonateServiceAccount: iamUserEmail,
		Port:                      port,
		PermissionsHelpPageURL:    permissionsHelpPageURL,
	}, nil
}

func (p *CloudSQLProxy) Start(ctx context.Context, timeoutSeconds int) error {
	bin, err := Path()
	if err != nil {
		return errors.Wrap(err, "failed to get path to the cloud-sql-proxy binary")
	}

	proxyCmd := fmt.Sprintf("%s -i -p %d --impersonate-service-account=%s  %s",
		bin, p.Port, p.ImpersonateServiceAccount, p.DBInstanceConnectionName)

	if timeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		defer cancel()

		std.Out.WriteWarningf("For security reasons, the current session will terminate in %d seconds. Use '-session.timeout' to increase the session duration.",
			timeoutSeconds)
		time.AfterFunc(time.Duration(timeoutSeconds)*time.Second, func() {
			select {
			case <-ctx.Done():
				return // nothing to do
			default:
				std.Out.WriteAlertf("The current session has timed out after %d seconds and will be terminated.",
					timeoutSeconds)
				cancel()
			}
		})
	}

	err = run.Cmd(ctx, proxyCmd).Run().StreamLines(func(line string) {
		std.Out.Write("  [cloud-sql-proxy] " + line)

		// Detect permission errors and present some additional help to the user.
		if strings.Contains(line, "IAM_PERMISSION_DENIED") ||
			// Sometimes, GCP just chooses to time out instead on permission
			// errors. Optimistically show help text in these cases.
			strings.Contains(line, "failed to connect to instance: context deadline exceeded") {

			// HACK: Sleep briefly to allow related output to be flushed
			// first, before presenting our help text, as the GCP error may
			// be multi-line. We do this safely in sg context by using
			// background.Run(...).
			background.Run(ctx, func(ctx context.Context, _ *std.Output) {
				select {
				case <-time.After(100 * time.Millisecond):
				case <-ctx.Done():
				}
				// Use our own output, instead of the aggregated background output,
				// to show our warning immediately.
				std.Out.WriteWarningf(
					"Possible permissions error detected - do you have the prerequisite Entitle permissions grant? See %s for more details.",
					p.PermissionsHelpPageURL)
			})
		}
	})
	if err != nil {
		if ctx.Err() == context.Canceled {
			return nil
		}
		return errors.Wrap(err, "failed to start cloud-sql-proxy")
	}

	return nil
}
