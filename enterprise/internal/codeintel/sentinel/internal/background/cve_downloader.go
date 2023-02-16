package background

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/sentinel/internal/store"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func NewCVEDownloader(store store.Store, metrics *Metrics, interval time.Duration) goroutine.BackgroundRoutine {
	cveDownloader := &CveDownloader{
		store: store,
	}

	return goroutine.NewPeriodicGoroutine(
		context.Background(),
		"codeintel.sentinel-cve-downloader", "TODO",
		interval,
		goroutine.HandlerFunc(func(ctx context.Context) error {
			_, err := cveDownloader.handle(ctx, metrics, false)
			return err
		}),
	)
}

type CveDownloader struct {
	store store.Store
}

const advisoryDatabaseURL = "https://github.com/github/advisory-database/archive/refs/heads/main.zip"

type Vulnerability struct {
	// Data that's consistent across all instances of a vulnerability
	SGVulnID               int
	ID                     string
	Summary                string
	Description            string
	CPEs                   []string
	CWEs                   []string
	RelatedVulnerabilities []string
	DataSource             string
	URLs                   []string
	Severity               string
	CVSSVector             string
	CVSSScore              string
	PublishedAt            time.Time
	PackagesAffected       []PackagesAffected
}

// Data that varies across instances of a vulnerability
// We could consider flattening this when storing in the db
type PackagesAffected struct {
	PackageName       string
	Language          string
	Namespace         string
	VersionConstraint []string
	Fixed             bool
	FixedIn           string
}

type GHSAVulnerability struct {
	SchemaVersion string    `json:"schema_version"`
	ID            string    `json:"id"`
	Modified      time.Time `json:"modified"`
	Published     time.Time `json:"published"`
	Aliases       []string  `json:"aliases"`
	Summary       string    `json:"summary"`
	Details       string    `json:"details"`
	Severity      []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	Affected []struct {
		Package struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
		} `json:"package"`
		Ranges []struct {
			Type   string              `json:"type"`
			Events []map[string]string `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
	References []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"references"`
	DatabaseSpecific struct {
		CWEIDs           []string  `json:"cwe_ids"`
		Severity         string    `json:"severity"`
		GitHubReviewed   bool      `json:"github_reviewed"`
		GitHubReviewedAt time.Time `json:"github_reviewed_at"`
		NVDPublishedAt   time.Time `json:"nvd_published_at"`
	} `json:"database_specific"`
}

func (matcher *CveDownloader) handle(ctx context.Context, metrics *Metrics, useLocalCache bool) (vulns []Vulnerability, err error) {
	return handleGithub(ctx, metrics, useLocalCache)
}

func handleGithub(ctx context.Context, metrics *Metrics, useLocalCache bool) (vulns []Vulnerability, err error) {
	var ghsaReader io.ReadCloser

	if useLocalCache {
		zipReader, err := os.Open("main.zip")
		if err != nil {
			return nil, errors.New("Unable to open zip file")
		}

		ghsaReader = zipReader
	} else {
		resp, err := http.Get(advisoryDatabaseURL)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, errors.Newf("unexpected status code %d", resp.StatusCode)
		}

		ghsaReader = resp.Body
	}

	// Open zip file rather than fetching it

	content, err := io.ReadAll(ghsaReader)
	if err != nil {
		return nil, err
	}

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

		var ghsaVuln GHSAVulnerability
		if err := json.NewDecoder(r).Decode(&ghsaVuln); err != nil {
			return nil, err
		}

		convertedVuln, err := ghsaToVuln(ghsaVuln)
		if err != nil {
			if _, ok := err.(GHSAUnreviewedError); ok {
				continue
			} else {
				return nil, err
			}
		}

		vulns = append(vulns, convertedVuln)
	}

	// TODO - insert into DB
	return vulns, nil
}

type GHSAUnreviewedError struct {
	msg string
}

func (e GHSAUnreviewedError) Error() string {
	return e.msg
}

// Convert a GHSAVulnerability to one or more Vulnerabilities
// A GHSA vuln may result in multiple vulns as we flatten its structure
func ghsaToVuln(g GHSAVulnerability) (vuln Vulnerability, err error) {

	// Only process vulns that GitHub has reviewed
	if !g.DatabaseSpecific.GitHubReviewed {
		return Vulnerability{}, GHSAUnreviewedError{"Vulnerability not reviewed"}
	}

	// Set up base vulnerability with common properties
	v := Vulnerability{
		ID:                     g.ID,
		Summary:                g.Summary,
		Description:            g.Details,
		PublishedAt:            g.DatabaseSpecific.NVDPublishedAt,
		CWEs:                   g.DatabaseSpecific.CWEIDs,
		RelatedVulnerabilities: g.Aliases,
		DataSource:             "https://github.com/advisories/" + g.ID,
		Severity:               g.DatabaseSpecific.Severity,
	}

	if len(g.Severity) > 0 && g.Severity[0].Score != "" {
		v.CVSSVector = g.Severity[0].Score
	} else {
		// fmt.Printf("No CVSS vector for %s - %v\n", v.ID, v.RelatedVulnerabilities)
	}

	var urls []string
	for _, ref := range g.References {
		urls = append(urls, ref.URL)
	}
	v.URLs = urls

	// g.Affected contains an array of packages that are affected by this vulnerability
	// Each package may also contain an array of version ranges that indicate when the vulnerability was
	//	introduced or resolved
	var pas []PackagesAffected
	for _, affected := range g.Affected {
		// Information that will be the same for all instances
		var affectedBase PackagesAffected
		affectedBase.PackageName = affected.Package.Name
		affectedBase.Namespace = "github:" + affected.Package.Ecosystem
		affectedBase.Language = githubEcosystemToLanguage(affected.Package.Ecosystem)

		if len(affected.Ranges) == 0 {
			pas = append(pas, affectedBase)
			continue
		}

		// Process version ranges affecting this pacakge
		for _, affectedRange := range affected.Ranges {
			a := affectedBase

			if len(affectedRange.Events) == 0 {
				continue
			}

			// Events can be: introduced, fixed, last_affected
			for _, event := range affectedRange.Events {
				for eventKey, eventValue := range event {
					switch eventKey {
					case "introduced":
						a.VersionConstraint = append(a.VersionConstraint, ">="+eventValue)
					case "fixed":
						a.VersionConstraint = append(a.VersionConstraint, "<"+eventValue)
						a.Fixed = true
						a.FixedIn = eventValue // In existing data, there is never >1 fixed entry per affected package
					case "last_affected":
						a.VersionConstraint = append(a.VersionConstraint, "<="+eventValue)
						// TODO: Does this actually mean it's fixed? Can we tell which version it's fixed in?
						// a.Fixed = true
					}
				}

			}

			pas = append(pas, a)
		}

		v.PackagesAffected = pas
	}

	vuln = v

	return
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
