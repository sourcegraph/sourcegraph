package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/run"

	"github.com/sourcegraph/sourcegraph/dev/codeintel-qa/internal"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func main() {
	if err := mainErr(context.Background()); err != nil {
		fmt.Printf("%s error: %s\n", internal.EmojiFailure, err.Error())
		os.Exit(1)
	}
}

const (
	relativeReposDir   = "dev/codeintel-qa/testdata/repos"
	relativeIndexesDir = "dev/codeintel-qa/testdata/indexes"
	numNavTestRoots    = 100
)

var navTestRoots = func() (roots []string) {
	for p := range numNavTestRoots {
		roots = append(roots, fmt.Sprintf("proj%d/", p+1))
	}

	return roots
}()

var repositoryMeta = []struct {
	org      string
	name     string
	indexer  string
	revision string
	roots    []string
}{
	{org: "go-nacelle", name: "config", indexer: "scip-go", revision: "72304c5497e662dcf50af212695d2f232b4d32be", roots: []string{""}},
	{org: "go-nacelle", name: "log", indexer: "scip-go", revision: "b380f4731178f82639695e2a69ae6ec2b8b6dbed", roots: []string{""}},
	{org: "go-nacelle", name: "nacelle", indexer: "scip-go", revision: "05cf7092f82bddbbe0634fa8ca48067bd219a5b5", roots: []string{""}},
	{org: "go-nacelle", name: "process", indexer: "scip-go", revision: "ffadb09a02ca0a8aa6518cf6c118f85ccdc0306c", roots: []string{""}},
	{org: "go-nacelle", name: "service", indexer: "scip-go", revision: "ca413da53bba12c23bb73ecf3c7e781664d650e0", roots: []string{""}},
	{org: "sourcegraph-testing", name: "nav-test", indexer: "scip-go", revision: "9156747cf1787b8245f366f81145d565f22c6041", roots: navTestRoots},
}

func mainErr(ctx context.Context) error {
	if err := cloneAll(ctx); err != nil {
		return err
	}

	if err := indexAll(ctx); err != nil {
		return err
	}

	return nil
}

func cloneAll(ctx context.Context) error {
	p := pool.New().WithErrors()

	for _, meta := range repositoryMeta {
		org, name := meta.org, meta.name
		p.Go(func() error { return clone(ctx, org, name) })
	}

	return p.Wait()
}

func clone(ctx context.Context, org, name string) error {
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return err
	}
	reposDir := filepath.Join(repoRoot, relativeReposDir)

	if ok, err := internal.FileExists(filepath.Join(reposDir, name)); err != nil {
		return err
	} else if ok {
		fmt.Printf("Repository %q already cloned\n", name)
		return nil
	}
	fmt.Printf("Cloning %q\n", name)

	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		return err
	}

	if err := run.Bash(ctx, "git", "clone", fmt.Sprintf("https://github.com/%s/%s.git", org, name)).Dir(reposDir).Run().Wait(); err != nil {
		return err
	}
	fmt.Printf("Finished cloning %q\n", name)
	return nil
}

func indexAll(ctx context.Context) error {
	repoRoot, err := root.RepositoryRoot()
	if err != nil {
		return err
	}
	reposDir := filepath.Join(repoRoot, relativeReposDir)
	indexesDir := filepath.Join(repoRoot, relativeIndexesDir)

	if err := os.MkdirAll(indexesDir, os.ModePerm); err != nil {
		return err
	}

	p := pool.New().WithErrors()

	for _, meta := range repositoryMeta {
		org, name, indexer, revision, roots := meta.org, meta.name, meta.indexer, meta.revision, meta.roots
		pair, ok := indexFunMap[indexer]
		if !ok {
			panic(fmt.Sprintf("unknown language %q", indexer))
		}

		p.Go(func() error {
			for _, root := range roots {
				cleanRoot := root
				if cleanRoot == "" {
					cleanRoot = "/"
				}
				cleanRoot = strings.ReplaceAll(cleanRoot, "/", "_")

				revision := revision
				targetFile := filepath.Join(indexesDir, fmt.Sprintf("%s.%s.%s.%s.%s", org, name, revision, cleanRoot, pair.Extension))

				if err := pair.IndexFunc(ctx, reposDir, targetFile, name, revision, root); err != nil {
					return errors.Wrapf(err, "failed to index %s@%s", name, revision)
				}
			}

			return nil
		})
	}

	return p.Wait()
}

type IndexerPair struct {
	Extension string
	IndexFunc func(context.Context, string, string, string, string, string) error
}

var indexFunMap = map[string]IndexerPair{
	"scip-go": {"scip", indexGoWithSCIP},
}

func indexGoWithSCIP(ctx context.Context, reposDir, targetFile, name, revision, root string) error {
	return indexGeneric(ctx, reposDir, targetFile, name, revision, func(repoCopyDir string) error {
		repoRoot := "."
		if root != "" {
			// If we're applying a root then we look _one back_ for the repository root
			// NOTE: we make the assumption that roots are single-directory for integration suite
			repoRoot = ".."
		}

		// --repository-root=. is necessary here as the temp dir might be within a strange
		// nest of symlinks on MacOS, which confuses the repository root detection in scip-go.
		if err := run.Bash(ctx, "scip-go", fmt.Sprintf("--repository-root=%s", repoRoot), "-o", targetFile).Dir(filepath.Join(repoCopyDir, root)).Run().Wait(); err != nil {
			return err
		}

		return nil
	})
}

func indexGeneric(ctx context.Context, reposDir, targetFile, name, revision string, index func(repoCopyDir string) error) error {
	if ok, err := internal.FileExists(targetFile); err != nil {
		return err
	} else if ok {
		fmt.Printf("Index for %s@%s already exists\n", name, revision)
		return nil
	}
	fmt.Printf("Indexing %s@%s\n", name, revision)

	tempDir, err := os.MkdirTemp("", "codeintel-qa")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	repoDir := filepath.Join(reposDir, name)
	repoCopyDir := filepath.Join(tempDir, name)

	if err := run.Bash(ctx, "cp", "-r", repoDir, tempDir).Run().Wait(); err != nil {
		return err
	}
	if err := run.Bash(ctx, "git", "checkout", revision).Dir(repoCopyDir).Run().Wait(); err != nil {
		return err
	}

	if err := index(repoCopyDir); err != nil {
		return err
	}

	fmt.Printf("Finished indexing %s@%s\n", name, revision)
	return nil
}
