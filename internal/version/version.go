package version

import (
	"expvar"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const devVersion = "0.0.0+dev"                              // version string for unreleased development builds
var devTimestamp = strconv.FormatInt(time.Now().Unix(), 10) // build timestamp for unreleased development builds

// version is configured at build time via ldflags like this:
// -ldflags "-X github.com/sourcegraph/sourcegraph/internal/version.version=1.2.3"
//
// The version may not be semver-compatible, e.g. `insiders` or `65769_2020-06-05_9bd91a3`.
var version = devVersion

var LastMinorVersionInMajorRelease = map[int]int{
	3: 43, // 3.43.0 -> 4.0.0
	4: 5,  // 4.5 -> 5.0.0,
}

func init() {
	exportedVersion := expvar.NewString("sourcegraph.version")
	exportedVersion.Set(version)
}

// Version returns the version string configured at build time.
func Version() string {
	return version
}

// IsDev reports whether the version string is an unreleased development build.
func IsDev(version string) bool {
	return version == devVersion
}

// Mock is used by tests to mock the result of Version and IsDev.
func Mock(mockVersion string) {
	version = mockVersion
}

// timestamp is the build timestamp configured at build time via ldflags like this:
// -ldflags "-X github.com/sourcegraph/sourcegraph/internal/version.timestamp=$UNIX_SECONDS"
var timestamp = devTimestamp

// mockTimeStamp is used by tests to mock the current build timestamp
func mockTimestamp(mockTimestamp string) {
	timestamp = mockTimestamp
}

// HowLongOutOfDate returns a time in months since this build of Sourcegraph was created. It is
// based on a constant baked into the Go binary at build time.
func HowLongOutOfDate(now time.Time) (int, error) {
	buildUnixTimestamp, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return 0, errors.Errorf("unable to parse version build timestamp: %w", err)
	}
	buildTime := time.Unix(buildUnixTimestamp, 0)

	if buildTime.After(now) {
		return 0, errors.New("version build timestamp is in the future")
	}
	daysSinceBuild := now.Sub(buildTime).Hours() / 24

	months := monthsFromDays(daysSinceBuild)
	if debug := os.Getenv("DEBUG_MONTHS_OUT_OF_DATE"); debug != "" {
		months, _ = strconv.Atoi(debug)
	}
	return months, nil
}

// monthsFromDays roughly determines the number of months given days
func monthsFromDays(days float64) int {
	const daysInAMonth = 30
	months := math.Floor(days / daysInAMonth)
	return int(months)
}
