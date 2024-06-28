package upgrades

// This file contains handler logic for appliances upgrades.

import (
	"errors"
	"fmt"
	"os"

	"github.com/Masterminds/semver"

	"github.com/sourcegraph/sourcegraph/internal/version"
)

// This is just for testing purposes.
func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: program <targetVersion>")
		os.Exit(1)
	}

	currentVersion := os.Args[1]
	targetVersion := os.Args[2]
	DetermineUpgradePolicy(currentVersion, targetVersion)
}

// TODO
// func ValidateDB()

// TODO
// Get current Version
// func GetCurrentVersion(ctx context.Context, obsvCtx *observation.Context) string {
// 	sqlDB, err := connections.RawNewFrontendDB(obsvCtx, "", "frontend")
// 	if err != nil {
// 		return errors.Errorf("failed to connect to frontend database: %s", err)
// 	}
// 	defer sqlDB.Close()

// 	db := database.NewDB(obsvCtx.Logger, sqlDB)
// 	upgradestore := upgradestore.New(db)

// 	currentVersion := upgradestore.GetServiceVersion(ctx)
// }

// Takes a target Version and determines if the upgrade requires downtime or not.
// Returns true if downtime is required and false if not, or on error.
// Return err on invalid target version.
func DetermineUpgradePolicy(currentVersion, targetVersion string) (downtime bool, err error) {
	current := semver.MustParse(currentVersion)
	target := semver.MustParse(targetVersion)

	// Rule out downgrades
	if target.Major() < current.Major() {
		fmt.Println("❌ Downgrade is not supported.")
		return false, errors.New("downgrade is not supported")
	} else if target.Major() == current.Major() && target.Minor() < current.Minor() {
		fmt.Println("❌ Downgrade is not supported.")
		return false, errors.New("downgrade is not supported")
	}

	// If there is a diff between major versions, the policy is MVU
	if target.Major() != current.Major() {
		// Check if the current version is the last minor version in the major release
		lastMinorInMajor, ok := version.LastMinorVersionInMajorRelease[int(current.Major())]
		if ok && int(current.Minor()) == lastMinorInMajor && target.Major() == current.Major()+1 && target.Minor() == 0 {
			fmt.Println("✅ Standard upgrade policy selected.")
			return false, nil
		}
		fmt.Println("✅ MVU upgrade policy selected.")
		return true, nil
	}
	// If there is a diff of greater than one between minor versions, the policy is MVU
	if target.Major() == current.Major() && target.Minor()-current.Minor() > 1 {
		fmt.Println("✅ MVU upgrade policy selected.")
		return true, nil
	}

	fmt.Println("✅ Standard upgrade policy selected.")
	return false, nil
}
