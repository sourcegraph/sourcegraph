package background

// Parse vulnerabilities from the GitHub Security Advisories (GHSA) database.
// GHSA uses the Open Source Vulnerability (OSV) format, with some custom extensions.

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/mapstructure"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/sentinel/shared"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const advisoryDatabaseURL = "https://github.com/github/advisory-database/archive/refs/heads/main.zip"

func ReadGitHubAdvisoryDB(ctx context.Context, useLocalCache bool) (vulns []shared.Vulnerability, err error) {
	if useLocalCache {
		zipReader, err := os.Open("main.zip")
		if err != nil {
			return nil, errors.New("unable to open zip file")
		}

		return ParseGitHubAdvisoryDB(zipReader)
	}

	resp, err := http.Get(advisoryDatabaseURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.Newf("unexpected status code %d", resp.StatusCode)
	}

	return ParseGitHubAdvisoryDB(resp.Body)
}

func ParseGitHubAdvisoryDB(ghsaReader io.Reader) ([]shared.Vulnerability, error) {
	content, err := io.ReadAll(ghsaReader)
	if err != nil {
		return nil, err
	}

	var vulns []shared.Vulnerability
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}

	for _, f := range zr.File {
		if filepath.Ext(f.Name) != ".json" {
			continue
		}

		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()

		var osvVuln OSV
		if err := json.NewDecoder(r).Decode(&osvVuln); err != nil {
			return nil, err
		}

		var g GHSA
		convertedVuln, err := osvToVuln(osvVuln, g)
		if err != nil {
			if _, ok := err.(GHSAUnreviewedError); ok {
				continue
			} else {
				return nil, err
			}
		}

		vulns = append(vulns, convertedVuln)
	}

	return vulns, nil
}

// GHSA-specific structs and handlers

type GHSADatabaseSpecific struct {
	Severity               string    `mapstructure:"severity" json:"severity"`
	GithubReviewed         bool      `mapstructure:"github_reviewed" json:"github_reviewed"`
	GithubReviewedAt       time.Time `json:"github_reviewed_at"`
	GithubReviewedAtString string    `mapstructure:"github_reviewed_at"`
	NvdPublishedAt         time.Time `json:"nvd_published_at"`
	NvdPublishedAtString   string    `mapstructure:"nvd_published_at"`
	CweIDs                 []string  `mapstructure:"cwe_ids" json:"cwe_ids"`
}

type GHSA int64

func (g GHSA) topLevelHandler(o OSV, v *shared.Vulnerability) (err error) {
	var databaseSpecific GHSADatabaseSpecific
	if err := mapstructure.Decode(o.DatabaseSpecific, &databaseSpecific); err != nil {
		return errors.Wrap(err, "cannot map DatabaseSpecific to GHSADatabaseSpecific")
	}

	// Only process reviewed GitHub vulnerabilities
	if !databaseSpecific.GithubReviewed {
		return GHSAUnreviewedError{"Vulnerability not reviewed"}
	}

	// mapstructure won't parse times, so do it manually
	if databaseSpecific.NvdPublishedAtString != "" {
		databaseSpecific.NvdPublishedAt, err = time.Parse(time.RFC3339, databaseSpecific.NvdPublishedAtString)
		if err != nil {
			fmt.Printf("Failed to parse NvdPublishedAtString: %s\n", err)
		}
	}
	if databaseSpecific.GithubReviewedAtString != "" {
		databaseSpecific.GithubReviewedAt, err = time.Parse(time.RFC3339, databaseSpecific.GithubReviewedAtString)
		if err != nil {
			fmt.Printf("Failed to parse GithubReviewedAtString: %s\n", err)
		}
	}

	v.DataSource = "https://github.com/advisories/" + o.ID
	v.Severity = databaseSpecific.Severity // Low, Medium, High, Critical // TODO: Override this with CVSS score if it exists
	v.CWEs = databaseSpecific.CweIDs

	// Ideally use NVD publish date; fall back on GitHub review date
	v.Published = databaseSpecific.NvdPublishedAt
	if v.Published.IsZero() {
		v.Published = databaseSpecific.GithubReviewedAt
	}

	return nil
}

func (g GHSA) affectedHandler(a OSVAffected, affectedPackage *shared.AffectedPackage) error {
	affectedPackage.Language = githubEcosystemToLanguage(a.Package.Ecosystem)
	affectedPackage.Namespace = "github:" + a.Package.Ecosystem

	return nil
}

type GHSAUnreviewedError struct {
	msg string
}

func (e GHSAUnreviewedError) Error() string {
	return e.msg
}

func githubEcosystemToLanguage(ecosystem string) (language string) {
	switch ecosystem {
	case "Go":
		language = "go"
	case "Hex":
		language = "erlang"
	case "Maven":
		language = "java"
	case "NuGet":
		language = ".net"
	case "Packagist":
		language = "php"
	case "Pub":
		language = "dart"
	case "PyPI":
		language = "python"
	case "RubyGems":
		language = "ruby"
	case "crates.io":
		language = "rust"
	case "npm":
		language = "Javascript"
	default:
		language = ""
	}

	return language
}
