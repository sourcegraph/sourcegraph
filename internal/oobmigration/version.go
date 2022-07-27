package oobmigration

import (
	"fmt"
	"strconv"

	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Version struct {
	Major int
	Minor int
}

func NewVersion(major, minor int) Version {
	return Version{
		Major: major,
		Minor: minor,
	}
}

var versionPattern = lazyregexp.New(`^v?(\d+)\.(\d+)(?:\.\d+)?$`)

// NewVersionFromString parses the major and minor version from the given string. If
// the string does not look like a parseable version, a false-valued flag is returned.
func NewVersionFromString(v string) (Version, bool) {
	if matches := versionPattern.FindStringSubmatch(v); len(matches) >= 3 {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])

		return NewVersion(major, minor), true
	}

	return Version{}, false
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

func (v Version) GitTag() string {
	return fmt.Sprintf("v%d.%d.0", v.Major, v.Minor)
}

// Next returns the next minor version immediately following the receiver.
func (v Version) Next() Version {
	lastMinorVersionInMajorRelease := map[int]int{
		// TODO: Determine last minor version in Sourcegraph v3; e.g., `3: 47, // 3.47.0 -> 4.0.0`
	}

	if minor, ok := lastMinorVersionInMajorRelease[v.Major]; ok && minor == v.Minor {
		// We're at terminal minor version for some major release
		// :tada:
		// Bump the major version and reset the minor version
		return NewVersion(v.Major+1, 0)
	}

	// Bump minor version
	return NewVersion(v.Major, v.Minor+1)
}

// UpgradeRange returns all minor versions in the closed interval [from, to].
// An error is returned if the interval would be empty.
func UpgradeRange(from, to Version) ([]Version, error) {
	if compareVersions(from, to) != VersionOrderBefore {
		return nil, errors.Newf("invalid range (from=%s > to=%s)", from, to)
	}

	var versions []Version
	for v := from; compareVersions(v, to) != VersionOrderAfter; v = v.Next() {
		versions = append(versions, v)
	}

	return versions, nil
}

type VersionOrder int

const (
	VersionOrderBefore VersionOrder = iota
	VersionOrderEqual
	VersionOrderAfter
)

// compareVersions returns the relationship between `a (op) b`.
func compareVersions(a, b Version) VersionOrder {
	for _, pair := range [][2]int{
		{a.Major, b.Major},
		{a.Minor, b.Minor},
	} {
		if pair[0] < pair[1] {
			return VersionOrderBefore
		}
		if pair[0] > pair[1] {
			return VersionOrderAfter
		}
	}

	return VersionOrderEqual
}

// pointIntersectsInterval returns true if point falls within the interval [lower, upper].
func pointIntersectsInterval(lower, upper, point Version) bool {
	return compareVersions(point, lower) != VersionOrderBefore && compareVersions(upper, point) != VersionOrderBefore
}
