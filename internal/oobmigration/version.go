package oobmigration

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const devVersionFlag = "+dev"

type Version struct {
	Major int
	Minor int
	Dev   bool // Indicates whether the version string comes with the dev flag.
}

func NewVersion(major, minor int) Version {
	return Version{
		Major: major,
		Minor: minor,
	}
}

func newDevVersion(major, minor int) Version {
	return Version{
		Major: major,
		Minor: minor,
		Dev:   true,
	}
}

var versionPattern = lazyregexp.New(`^v?(\d+)\.(\d+)(?:\.(\d+))?(?:-\w*)?(?:\+[\w.]*)?$`)

// NewVersionFromString parses the major and minor version from the given string. If
// the string does not look like a parseable version, a false-valued flag is returned.
func NewVersionFromString(v string) (Version, bool) {
	version, _, ok := NewVersionAndPatchFromString(v)
	return version, ok
}

// NewVersionAndPatchFromString parses the major and minor version from the given
// string. If the string does not look like a parseable version, a false-valued
// flag is returned. If the input string also supplies a patch version, it is
// returned. If a patch is not supplied this value is zero.
func NewVersionAndPatchFromString(v string) (Version, int, bool) {
	newVersion := NewVersion
	if strings.HasSuffix(v, devVersionFlag) {
		v = strings.TrimSuffix(v, devVersionFlag)
		newVersion = newDevVersion
	}

	matches := versionPattern.FindStringSubmatch(v)
	if len(matches) < 3 {
		return Version{}, 0, false
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])

	if len(matches) == 3 {
		return newVersion(major, minor), 0, true
	}

	patch, _ := strconv.Atoi(matches[3])
	return newVersion(major, minor), patch, true
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

func (v Version) GitTag() string {
	return v.GitTagWithPatch(0)
}

func (v Version) GitTagWithPatch(patch int) string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, patch)
}

// Next returns the next minor version immediately following the receiver.
func (v Version) Next() Version {
	if minor, ok := version.LastMinorVersionInMajorRelease[v.Major]; ok && minor == v.Minor {
		// We're at terminal minor version for some major release
		// :tada:
		// Bump the major version and reset the minor version
		return NewVersion(v.Major+1, 0)
	}

	// Bump minor version
	return NewVersion(v.Major, v.Minor+1)
}

// Previous returns the previous minor version immediately preceding the receiver.
func (v Version) Previous() (Version, bool) {
	if v.Minor == 0 {
		minor, ok := version.LastMinorVersionInMajorRelease[v.Major-1]
		return NewVersion(v.Major-1, minor), ok
	}

	return NewVersion(v.Major, v.Minor-1), true
}

// UpgradeRange returns all minor versions in the closed interval [from, to].
// An error is returned if the interval would be empty.
func UpgradeRange(from, to Version) ([]Version, error) {
	if CompareVersions(from, to) != VersionOrderBefore {
		return nil, errors.Newf("invalid range (from=%s >= to=%s)", from, to)
	}

	var versions []Version
	for v := from; CompareVersions(v, to) != VersionOrderAfter; v = v.Next() {
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

// CompareVersions returns the relationship between `a (op) b` in the form of VersionOrder iota.
//
// Ex: CompareVersions(5.2.x, 5.3.x) returns VersionOrderBefore because 5.2.x is before 5.3.x.
func CompareVersions(a, b Version) VersionOrder {
	for _, pair := range [2][2]int{
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

// SortVersions sorts the given version slice in ascending order.
func SortVersions(vs []Version) {
	sort.Slice(vs, func(i, j int) bool {
		if vs[i].Major == vs[j].Major {
			return vs[i].Minor < vs[j].Minor
		}

		return vs[i].Major < vs[j].Major
	})
}

// pointIntersectsInterval returns true if point falls within the interval [lower, upper].
func pointIntersectsInterval(lower, upper, point Version) bool {
	return CompareVersions(point, lower) != VersionOrderBefore && CompareVersions(upper, point) != VersionOrderBefore
}
