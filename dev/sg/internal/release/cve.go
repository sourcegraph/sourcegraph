package release

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/grafana/regexp"
	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/bk"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

var cvePattern = regexp.MustCompile(`<\w+>(CVE-\d+-\d+)<\/\w+>`)

var buildNumberFlag = cli.StringFlag{
	Name:     "buildNumber",
	Usage:    "The buildkite build number to check for CVEs",
	Required: true,
	Aliases:  []string{"b"},
}

var referenceUriFlag = cli.StringFlag{
	Name:     "uri",
	Usage:    "A reference url that contains approved CVEs. Often a link to a handbook page eg: https://handbook.sourcegraph.com/departments/security/tooling/trivy/4-2-0/.",
	Required: true,
	Aliases:  []string{"u"},
}

func findUnapprovedCVEs(all []string, referenceDocument string) []string {
	var unapproved []string
	for _, cve := range all {
		if !strings.Contains(referenceDocument, cve) {
			unapproved = append(unapproved, cve)
		}
	}
	return unapproved
}

func extractCVEs(pattern *regexp.Regexp, document string) []string {
	var found []string
	matches := pattern.FindAllStringSubmatch(document, -1)
	for _, match := range matches {
		cve := strings.TrimSpace(match[1])
		found = append(found, cve)
	}
	return found
}

func downloadUrl(uri string, w io.Writer) (err error) {
	std.Out.WriteLine(output.Styledf(output.StylePending, "Downloading url: %s", uri))
	resp, err := http.Get(uri)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
func cveCheck(cmd *cli.Context) error {
	std.Out.WriteLine(output.Styledf(output.StylePending, "Checking release for approved CVEs..."))

	referenceUrl := referenceUriFlag.Get(cmd)
	buildNumber := buildNumberFlag.Get(cmd)

	return CveCheck(cmd.Context, buildNumber, referenceUrl, false) // TODO(@jhchabran)
}

func CveCheck(ctx context.Context, buildNumber, referenceUrl string, verbose bool) error {
	client, err := bk.NewClient(ctx, std.Out)
	if err != nil {
		return errors.Wrap(err, "bk.NewClient")
	}

	artifacts, err := client.ListArtifactsByBuildNumber(ctx, "sourcegraph", buildNumber)
	if err != nil {
		return errors.Wrap(err, "unable to list artifacts by build number")
	}

	var refBuf bytes.Buffer
	err = downloadUrl(referenceUrl, &refBuf)
	if err != nil {
		return errors.Wrap(err, "unable to download reference url")
	}
	if refBuf.Len() == 0 {
		return errors.New("provided reference url does not have any contents")
	}

	var foundCVE []string
	var unapprovedCVE []string

	for _, artifact := range artifacts {
		name := *artifact.Filename
		url := *artifact.DownloadURL

		if strings.HasSuffix(*artifact.Filename, "security-report.html") {
			std.Out.WriteLine(output.Styledf(output.StylePending, "Checking security report: %s %s", name, url))

			var buf bytes.Buffer
			err = client.DownloadArtifact(artifact, &buf)
			if err != nil {
				return errors.Newf("failed to download security artifact %q at %s: %w", name, url, err)
			}

			foundCVE = append(foundCVE, extractCVEs(cvePattern, buf.String())...)
		}
	}
	unapprovedCVE = findUnapprovedCVEs(foundCVE, refBuf.String())

	std.Out.WriteLine(output.Styledf(output.StyleBold, "Found %d CVEs in the build", len(foundCVE)))
	if verbose {
		for _, s := range foundCVE {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "%s", s))
		}
	}
	if len(unapprovedCVE) > 0 {
		std.Out.WriteLine(output.Styledf(output.StyleWarning, "Unable to match CVEs"))
		for _, s := range unapprovedCVE {
			std.Out.WriteLine(output.Styledf(output.StyleWarning, "%s", s))
		}
	} else {
		std.Out.WriteLine(output.Styledf(output.StyleSuccess, "All CVEs approved!"))
	}

	return nil
}
