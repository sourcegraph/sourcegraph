// Initially copy-pasta from https://github.com/sourcegraph/controller/blob/main/internal/cloudsqlproxy/gen.go
package cloudsqlproxy

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

// CloudSQLProxyVersion can be found from https://github.com/GoogleCloudPlatform/cloud-sql-proxy/releases
const CloudSQLProxyVersion = "2.8.1"

// Init configures the cloud-sql-proxy binary for the current platform
// optionally downloading if requested
func Init(download bool) error {
	if download {
		if err := Download(); err != nil {
			return err
		}
		// If download succeeded, still continue and run the binary check
	}

	cloudSQLProxyPath, err := Path()
	if err != nil {
		return errors.Wrap(err, "failed to get path for current platform")
	}
	_, err = os.Stat(cloudSQLProxyPath)
	if err != nil && os.IsNotExist(err) {
		std.Out.WriteWarningf("cloud-sql-proxy binary not found at %q", cloudSQLProxyPath)

		std.Out.Promptf("Would you like me to install cloud-sql-proxy for you? [y/N]")
		var input string
		if _, err := fmt.Scan(&input); err != nil {
			return err
		}
		if input != "y" {
			// We should NOT install it, throw the error back at the user
			return errors.Wrap(err, "failed to find cloud-sql-proxy")
		}

		// Call self again, but this time, set download=true. If it fails again
		// hopefully the user doesn't say yes
		return Init(true)
	} else if err != nil {
		return errors.Wrapf(err, "failed to read %s binary", cloudSQLProxyPath)
	}
	return nil
}

// Path returns the path to the cloud-sql-proxy binary.
func Path() (string, error) {
	userCacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", errors.Wrap(err, "failed to get user cache dir")
	}

	appCacheDir := filepath.Join(userCacheDir, "sourcegraph", "bin", "cloud-sql-proxy", CloudSQLProxyVersion)
	if err := os.MkdirAll(appCacheDir, 0755); err != nil {
		return "", errors.Wrap(err, "failed to create application cache dir")
	}

	cloudSQLProxyPath := filepath.Join(appCacheDir, "cloud-sql-proxy")

	return cloudSQLProxyPath, nil
}

func Download() error {
	url := fmt.Sprintf("https://storage.googleapis.com/cloud-sql-connectors/cloud-sql-proxy/v%s/cloud-sql-proxy.%s.%s",
		CloudSQLProxyVersion, runtime.GOOS, runtime.GOARCH)
	pending := std.Out.Pending(output.Styledf(output.StylePending,
		"Downloading cloud-sql-proxy binary for current platform (goos: %s, goarch: %s, url: %s)",
		runtime.GOOS, runtime.GOARCH, url))
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		return errors.Wrapf(err,
			"failed to download cloud-sql-proxy binary for OS: %s, Arch: %s",
			runtime.GOOS, runtime.GOARCH)
	}
	defer resp.Body.Close()
	d, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response for current platform")
	}
	path, err := Path()
	if err != nil {
		return errors.Wrap(err, "failed to get path for current platform")
	}
	pending.Updatef("Saving cloud-sql-proxy binary to %q", path)
	err = os.WriteFile(path, d, 0755)
	if err != nil {
		return errors.Wrap(err, "failed to write cloud-sql-proxy binary")
	}
	pending.Complete(output.Emojif(output.EmojiSuccess,
		"cloud-sql-proxy binary saved to %q", path))
	return nil
}
