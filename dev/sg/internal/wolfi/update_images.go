package wolfi

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

// getAllImages returns a list of all image configs found in the baseImageDir
func getAllImages() (imageNames []string, err error) {
	// Iterate over *.yaml files in wolfi-images/
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return nil, err
	}
	imageDir := filepath.Join(repoRoot, baseImageDir)
	files, err := os.ReadDir(imageDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".yaml") {
			continue
		}

		imageName := strings.TrimSuffix(file.Name(), ".yaml")
		imageNames = append(imageNames, imageName)
	}

	return imageNames, nil
}

// UpdateAllImages runs UpdateImage for all images in the baseImageDir
func UpdateAllImages(ctx *cli.Context, opts BaseImageOpts) error {
	imageNames, err := getAllImages()
	if err != nil {
		return err
	}

	for _, imageName := range imageNames {
		bc, err := SetupBaseImageBuild(imageName, PackageRepoConfig{}, opts)
		if err != nil {
			return err
		}

		bc.UpdateImage(ctx)
	}

	return nil
}

// UpdateImage updates re-locks the set of packages for the given image by updating its lockfile
func (bc BaseImageConfig) UpdateImage(_ *cli.Context) error {
	// Update lockfile
	std.Out.WriteLine(output.Linef("🗝️ ", output.StylePending, fmt.Sprintf("Updating apko lockfile for %s", bc.ImageName)))
	if err := bc.ApkoLock(); err != nil {
		return err
	}

	return nil
}

// ApkoLock calls `apko lock` to generate a lockfile for the given image
func (bc BaseImageConfig) ApkoLock() error {
	localImageConfigPath := strings.TrimPrefix(bc.ImageConfigPath, bc.ImageConfigDir+"/")

	apkoArgs := []string{"run", "@rules_apko//apko", "lock", "--", localImageConfigPath}

	apkoFlags := []string{}
	if bc.RepositoryAppend != "" {
		apkoFlags = append(apkoFlags, "--repository-append", bc.RepositoryAppend)
	}
	if bc.KeyringAppend != "" {
		apkoFlags = append(apkoFlags, "--keyring-append", bc.KeyringAppend)
	}

	fmt.Printf("Running bazel %v %v\n", apkoArgs, apkoFlags)

	cmd := exec.Command("bazel", append(apkoArgs, apkoFlags...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = bc.ImageConfigDir
	err := cmd.Run()
	if err != nil {
		return errors.Wrap(err, "failed to build base image")
	}

	// Update hash in lockfile
	err = bc.updateApkoLockHash()
	if err != nil {
		return err
	}

	return nil
}

// getApkoConfigHash returns the SHA256 hash of an image's apko config file
func (bc BaseImageConfig) getApkoConfigHash() (apkoConfigHashHex string, err error) {
	apkoConfig, err := os.ReadFile(bc.ImageConfigPath)
	if err != nil {
		return "", err
	}

	apkoConfigHash := sha256.Sum256([]byte(apkoConfig))
	apkoConfigHashHex = hex.EncodeToString(apkoConfigHash[:])

	return apkoConfigHashHex, nil
}

// readLockFile returns the contents of an image's lockfile as a map
func (bc BaseImageConfig) readLockFile() (imageLockData map[string]interface{}, err error) {
	imageLock, err := os.ReadFile(bc.LockfilePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(imageLock, &imageLockData)
	if err != nil {
		return nil, err
	}

	return imageLockData, nil
}

// CheckApkoLockHashes runs CheckApkoLockHash for all images in baseImageDir
func CheckApkoLockHashes(imageNames []string) (allImagesMatch bool, mismatchedImages []string, err error) {
	if len(imageNames) == 0 {
		imageNames, err = getAllImages()
		if err != nil {
			return false, nil, err
		}
	}

	allImagesMatch = true

	for _, imageName := range imageNames {
		bc, err := SetupBaseImageBuild(imageName, PackageRepoConfig{}, BaseImageOpts{})
		if err != nil {
			return false, nil, err
		}

		imageSynced, err := bc.CheckApkoLockHash()
		if err != nil {
			return false, nil, err
		}

		if !imageSynced {
			allImagesMatch = false
			mismatchedImages = append(mismatchedImages, imageName)
		}
	}

	return allImagesMatch, mismatchedImages, nil
}

// CheckApkoLockHash checks whether the hash of an image's YAML file matches the hash stored in the corresponding lockfile
// This allows us to detect changes to the YAML file and re-run apko lock if necessary
func (bc BaseImageConfig) CheckApkoLockHash() (isMatch bool, err error) {
	apkoConfigHashHex, err := bc.getApkoConfigHash()
	if err != nil {
		return false, err
	}

	imageLockData, err := bc.readLockFile()
	if err != nil {
		// Lockfile doesn't exist
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	if val, exists := imageLockData["configHash"]; exists {
		if val == apkoConfigHashHex {
			isMatch = true
		}
	}

	return isMatch, nil
}

// updateApkoLockHash updates the hash of the image's YAML file that's stored in the corresponding lockfile.
// It should only be called after successfully calling ApkoLock()
func (bc BaseImageConfig) updateApkoLockHash() (err error) {
	apkoConfigHashHex, err := bc.getApkoConfigHash()
	if err != nil {
		return err
	}

	imageLockData, err := bc.readLockFile()
	if err != nil {
		return err
	}

	imageLockData["configHash"] = apkoConfigHashHex

	// Marshal the map back to json
	updatedFile, err := json.MarshalIndent(imageLockData, "", "  ")
	if err != nil {
		return err
	}

	// Write the updated json back to the file
	err = os.WriteFile(bc.LockfilePath, updatedFile, 0644)
	if err != nil {
		return err
	}

	return nil
}
