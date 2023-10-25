package git

import (
	"os"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
)

const typeFile = "sg.type"

// SetRepositoryType sets the type of the repository.
func SetRepositoryType(dir common.GitDir, typ string) error {
	return os.WriteFile(dir.Path(typeFile), []byte(typ), os.ModePerm)
}

// GetRepositoryType returns the type of the repository.
// If not set, will return an empty string.
func GetRepositoryType(dir common.GitDir) (string, error) {
	c, err := os.ReadFile(dir.Path(typeFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	return string(c), nil
}
