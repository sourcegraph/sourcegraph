package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Masterminds/semver"
)

type schemaDescription struct {
	version semver.Version
	files   map[string][]byte
}

func (sd *schemaDescription) Export(path string) error {
	for filename, b := range sd.files {
		f, err := os.Create(filepath.Join(path, fmt.Sprintf("v%s-%s", sd.version.String(), filename)))
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Write(b); err != nil {
			return err
		}
	}
	return nil
}
