package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sourcegraph/sourcegraph/dev/codeintel-qa/internal"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type uploadMeta struct {
	id       string
	repoName string
	commit   string
	root     string
}

// uploadAll uploads the dumps for the commits present in the given commitsByRepo map.
// Uploads are performed concurrently given the limiter instance as well as the set of
// flags supplied by the user. This function returns a slice of uploadMeta containing
// the graphql identifier of the uploaded resources.
func uploadAll(ctx context.Context, extensionAndCommitsByRepo map[string][]internal.ExtensionCommitAndRoot, limiter *internal.Limiter) ([]uploadMeta, error) {
	n := 0
	for _, commits := range extensionAndCommitsByRepo {
		n += len(commits)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, n)
	uploadCh := make(chan uploadMeta, n)

	for repoName, extensionAndCommits := range extensionAndCommitsByRepo {
		for _, extensionCommitAndRoot := range extensionAndCommits {
			commit := extensionCommitAndRoot.Commit
			extension := extensionCommitAndRoot.Extension
			root := extensionCommitAndRoot.Root

			wg.Add(1)

			go func(file string) {
				defer wg.Done()

				if err := limiter.Acquire(ctx); err != nil {
					errCh <- err
					return
				}
				defer limiter.Release()

				fmt.Printf("[%5s] %s Uploading index for %s@%s:%s\n", internal.TimeSince(start), internal.EmojiLightbulb, repoName, commit[:7], root)

				cleanedRoot := strings.ReplaceAll(root, "_", "/")
				graphqlID, err := upload(ctx, internal.MakeTestRepoName(repoName), commit, file, cleanedRoot)
				if err != nil {
					errCh <- err
					return
				}

				fmt.Printf("[%5s] %s Finished uploading index %s for %s@%s:%s\n", internal.TimeSince(start), internal.EmojiSuccess, graphqlID, repoName, commit[:7], cleanedRoot)

				uploadCh <- uploadMeta{
					id:       graphqlID,
					repoName: repoName,
					commit:   commit,
					root:     cleanedRoot,
				}
			}(fmt.Sprintf("%s.%s.%s.%s", strings.Replace(repoName, "/", ".", 1), commit, root, extension))
		}
	}

	go func() {
		wg.Wait()
		close(errCh)
		close(uploadCh)
	}()

	for err := range errCh {
		return nil, err
	}

	uploads := make([]uploadMeta, 0, n)
	for upload := range uploadCh {
		uploads = append(uploads, upload)
	}

	return uploads, nil
}

// upload invokes `src code-intel upload` on the host and returns the graphql identifier of
// the uploaded resource.
func upload(ctx context.Context, repoName, commit, file, root string) (string, error) {
	argMap := map[string]string{
		"root":   root,
		"repo":   repoName,
		"commit": commit,
		"file":   file,
	}

	args := make([]string, 0, len(argMap))
	for k, v := range argMap {
		args = append(args, fmt.Sprintf("-%s=%s", k, v))
	}

	tempDir, err := os.MkdirTemp("", "codeintel-qa")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	src, err := os.Open(filepath.Join(indexDir, file))
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(filepath.Join(tempDir, file), os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(dst, src); err != nil {
		_ = dst.Close()
		return "", err
	}
	if err := dst.Close(); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, srcPath, append([]string{"code-intel", "upload", "-json"}, args...)...)
	cmd.Dir = tempDir
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("SRC_ENDPOINT=%s", internal.SourcegraphEndpoint))
	cmd.Env = append(cmd.Env, fmt.Sprintf("SRC_ACCESS_TOKEN=%s", internal.SourcegraphAccessToken))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("failed to upload index for %s@%s:%s: %s", repoName, commit, root, output))
	}

	resp := struct {
		UploadURL string `json:"uploadUrl"`
	}{}
	if err := json.Unmarshal(output, &resp); err != nil {
		return "", err
	}

	parts := strings.Split(resp.UploadURL, "/")
	return parts[len(parts)-1], nil
}
