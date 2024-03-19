package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Masterminds/semver"
	"github.com/google/go-github/v55/github"
	"github.com/grafana/regexp"
	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"golang.org/x/oauth2"
)

//go:embed gcs_versions.json
var gcsVersionsRaw []byte

var gcsFilenames = []string{
	"internal_database_schema.json",
	"internal_database_schema.codeintel.json",
	"internal_database_schema.codeinsights.json",
}
var githubFilenames = []string{
	"schema.json",
	"schema.codeintel.json",
	"schema.codeinsights.json",
}

var gcsVersions []semver.Version

func init() {
	if err := json.Unmarshal(gcsVersionsRaw, &gcsVersions); err != nil {
		panic("invalid JSON for gcs_versions.json: " + err.Error())
	}
}

func main() {
	ctx := context.Background()

	usage := func() {
		fmt.Println("Current version argument is required.")
		fmt.Println("usage: airgappedgen vX.Y.Z <path to folder>")
		os.Exit(1)
	}

	if len(os.Args) < 3 {
		usage()
	}
	currentVersionRaw := os.Args[1]
	if currentVersionRaw == "" {
		usage()
	}

	currentVersion, err := semver.NewVersion(currentVersionRaw)
	if err != nil {
		panic(err)
	}

	exportPath := os.Args[2]
	if exportPath == "" {
		usage()
	}

	schemasGCS, err := downloadGCSVersions(ctx, gcsVersions)
	if err != nil {
		panic(err)
	}

	githubVersions, err := listRemoteTaggedVersions(ctx, currentVersion)
	if err != nil {
		panic(err)
	}

	schemasGitHub, err := downloadRemoteTaggedVersions(ctx, githubVersions)
	if err != nil {
		panic(err)
	}

	for _, sd := range append(schemasGCS, schemasGitHub...) {
		if err := sd.Export(exportPath); err != nil {
			panic(err)
		}
	}
}

func downloadRemoteTaggedVersions(_ context.Context, versions []semver.Version) ([]*schemaDescription, error) {
	urlFmt := "https://raw.githubusercontent.com/sourcegraph/sourcegraph/v%s/internal/database/%s"

	p := pool.NewWithResults[*schemaDescription]().WithMaxGoroutines(5).WithErrors()

	for _, version := range versions {
		version := version
		p.Go(func() (*schemaDescription, error) {
			sd := schemaDescription{
				version: version,
				files:   map[string][]byte{},
			}

			for _, filename := range githubFilenames {
				url := fmt.Sprintf(urlFmt, version.String(), filename)
				resp, err := http.Get(url)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to download remote schema %q from GCS", versions[0].String())
				}
				defer resp.Body.Close()

				if resp.StatusCode >= 500 {
					return nil, fmt.Errorf("server error, remote schema %q: %s", url, resp.Status)
				}
				if resp.StatusCode == 404 {
					return nil, fmt.Errorf("server error, remote schema %q not found: %s", url, resp.Status)
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to read response body")
				}
				sd.files[fmt.Sprintf("internal_database_%s", filename)] = b
			}
			return &sd, nil
		})
	}
	return p.Wait()
}

func downloadGCSVersions(_ context.Context, versions []semver.Version) ([]*schemaDescription, error) {
	urlFmt := "https://storage.googleapis.com/sourcegraph-assets/migrations/drift/v%s-%s"

	p := pool.NewWithResults[*schemaDescription]().WithMaxGoroutines(5).WithErrors()

	for _, version := range versions {
		version := version
		p.Go(func() (*schemaDescription, error) {
			sd := schemaDescription{
				version: version,
				files:   map[string][]byte{},
			}

			for _, filename := range gcsFilenames {
				url := fmt.Sprintf(urlFmt, version.String(), filename)
				resp, err := http.Get(url)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to download remote schema %q from GCS", versions[0].String())
				}
				defer resp.Body.Close()

				if resp.StatusCode >= 500 {
					return nil, fmt.Errorf("server error downloading remote schema %q: %s", url, resp.Status)
				}
				if resp.StatusCode == 404 {
					// Oldest versions doesn't have all schemas, just the frontend, so we're fine skipping them.
					continue
				}

				b, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to read response body")
				}
				sd.files[filename] = b

			}
			return &sd, nil
		})
	}
	return p.Wait()
}

func listRemoteTaggedVersions(ctx context.Context, currentVersion *semver.Version) ([]semver.Version, error) {
	var ghc *github.Client
	if tok := os.Getenv("GH_TOKEN"); tok != "" {
		ghc = github.NewClient(oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: os.Getenv("GH_TOKEN")},
		)))
	} else {
		ghc = github.NewClient(http.DefaultClient)
	}

	// Unauthenticated requests can take a very long time if we get throttled.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	allTags := []string{}
	page := 0
	for {
		tags, resp, err := ghc.Repositories.ListTags(ctx, "sourcegraph", "sourcegraph", &github.ListOptions{Page: page})
		if err != nil {
			return nil, errors.Wrap(err, "failed to list tags from GitHub")
		}
		if resp.NextPage == 0 {
			break
		}
		page = resp.NextPage

		for _, tag := range tags {
			// If the tag is not a Sourcegraph release, like an old tag for App, we skip it.
			if !isTagSourcegraphRelease(tag.GetName()) {
				continue
			}

			// Now we're sure it's a proper tag, let's parse it.
			versionTag, err := semver.NewVersion(tag.GetName())
			if err != nil {
				return nil, errors.Wrapf(err, "list remote release tags: %w")
			}

			// If the tag is relevant to the version we're releasing, include it.
			if isTagAfterGCS(versionTag) && isTagPriorToCurrentRelease(versionTag, currentVersion) {
				allTags = append(allTags, tag.GetName())
			}
		}
	}

	allVersions := []semver.Version{}
	for _, tag := range allTags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid semver tag: %w")
		}
		allVersions = append(allVersions, *v)
	}
	return allVersions, nil
}

var versionRegexp = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

// isTagSourcegraphRelease returns true if the tag we're looking at is a Sourcegraph release.
func isTagSourcegraphRelease(tag string) bool {
	return versionRegexp.MatchString(tag)
}

// isTagAfterGCS returns true if the tag we're looking at has been released at the time we started
// committing to Git all schemas descriptions. The versions mentioned in ./gcs_versions.json were not
// but the tags are still there, so we can't fetch those from GitHub as they'll be missing at that
// point in time.
func isTagAfterGCS(versionFromTag *semver.Version) bool {
	return versionFromTag.GreaterThan(&gcsVersions[len(gcsVersions)-1])
}

// isTagPriorToCurrentRelease returns true if the tag we're looking at has been release prior to the
// current version we're releasing. This is to avoid embedding 5.3.0 schemas into a 5.2.X patch release
// that gets released AFTER 5.3.0, typically to share a bug fix to customers still running on 5.2.X-1.
func isTagPriorToCurrentRelease(versionFromTag *semver.Version, currentVersion *semver.Version) bool {
	// We include versions that are:
	// - released after than the latest gcs version
	// - before the current version we're releasing.
	// Basically, if we release 5.2.X after 5.3.0 is out, we don't want to include the schemas for 5.3.0
	// because from the POV of the migrator, they don't exist yet.
	return versionFromTag.LessThan(currentVersion)
}
