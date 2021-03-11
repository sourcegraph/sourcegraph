// +build ignore

package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
)

func clean(base string) error {
	// Delete every Markdown file that we find, and track the directories that
	// exist.
	dirs := []string{}
	if err := filepath.Walk(base, func(fp string, info os.FileInfo, err error) error {
		if info.IsDir() {
			dirs = append(dirs, fp)
		} else if path.Ext(fp) == ".md" {
			return os.Remove(fp)
		}

		return nil
	}); err != nil {
		return errors.Wrap(err, "error walking Markdown files")
	}

	// Now iterate over the directories depth-first, removing the ones that are
	// empty.
	sort.Slice(dirs, func(i, j int) bool {
		return len(dirs[j]) < len(dirs[i])
	})
	for _, dir := range dirs {
		d, err := ioutil.ReadDir(dir)
		if err != nil {
			log.Fatal(err)
		}

		if len(d) == 0 {
			if err := os.Remove(dir); err != nil {
				return errors.Wrapf(err, "error removing directory %q", dir)
			}
		}
	}

	return nil
}

func build(base string) error {
	// Since we don't want to pollute the local go.mod or go.sum, but we also
	// need an isolated environment, we're going to set up an isolated directory
	// to build src-cli. Some day https://github.com/golang/go/issues/43684 will
	// have its solution merged and we might be able to avoid all of this with a
	// go:generate one-liner that calls `go install
	// github.com/sourcegraph/src-cli/cmd/src@main`, but we're not quite there
	// yet.

	dir, err := ioutil.TempDir("", "src-cli-doc-gen")
	if err != nil {
		return errors.Wrap(err, "creating temporary directory")
	}
	defer os.RemoveAll(dir)

	if err := os.Chdir(dir); err != nil {
		return errors.Wrap(err, "changing to temporary directory")
	}

	// We have a few fun things going on here, but by far the funnest is that
	// src-cli (and its dependencies) rely on a go.mod replacement of our
	// upstream YAML library with our own fork. Unfortunately, doing a simple
	// `go build` (or whatever) with the src-cli URL fails as a result, since
	// batch-change-utils will try to call a method that doesn't exist on the
	// upstream library.
	//
	// Since replacements only happen locally, we have to set up the same
	// replacement in a local go.mod. On the bright side, that means we don't
	// have to set GO111MODULE explicitly: this just looks like a normal Go
	// module to Go.
	//
	// If this breaks in future with an obscure looking compilation error, the
	// first thing you'll want to check is that any replacements in
	// https://github.com/sourcegraph/src-cli/blob/main/go.mod are reproduced
	// here as well.
	//
	// In summary, this is _hilariously_ cursed.
	if err := ioutil.WriteFile("go.mod", []byte(`module github.com/sourcegraph/sourcegraph/doc/cli/references

replace github.com/gosuri/uilive v0.0.4 => github.com/mrnugget/uilive v0.0.4-fix-escape

// See: https://github.com/ghodss/yaml/pull/65
replace github.com/ghodss/yaml => github.com/sourcegraph/yaml v1.0.1-0.20200714132230-56936252f152
	`), 0600); err != nil {
		return errors.Wrap(err, "setting up go.mod")
	}

	goGet := exec.Command("go", "get", "github.com/sourcegraph/src-cli/cmd/src")
	goGet.Env = append(os.Environ(), "GOBIN="+dir)
	if out, err := goGet.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "getting src-cli:\n%s\n", string(out))
	}

	if err := os.Chdir(base); err != nil {
		return errors.Wrap(err, "returning to the working directory")
	}

	src := path.Join(dir, "src")
	srcDoc := exec.Command(src, "doc", "-o", ".")
	if out, err := srcDoc.CombinedOutput(); err != nil {
		return errors.Wrapf(err, "running src doc:\n%s\n", string(out))
	}

	return nil
}

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("error getting working directory: %v", err)
	}

	if err := clean(wd); err != nil {
		log.Fatalf("error cleaning working directory: %v", err)
	}

	if err := build(wd); err != nil {
		log.Fatalf("error building documentation: %v", err)
	}
}
