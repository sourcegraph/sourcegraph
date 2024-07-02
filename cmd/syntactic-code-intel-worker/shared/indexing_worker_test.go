package shared

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/stretchr/testify/require"

	"github.com/bazelbuild/rules_go/go/runfiles"
	"github.com/sourcegraph/scip/bindings/go/scip"
	stores "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/jobstore"
	testutils "github.com/sourcegraph/sourcegraph/internal/codeintel/syntactic_indexing/testkit"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"

	"github.com/sourcegraph/sourcegraph/lib/iterator"
	"google.golang.org/protobuf/proto"
)

func TestIndexingWorker(t *testing.T) {
	/*
		The purpose of this test is to verify that the indexing worker code can successfully process a
		syntactic indexing record, which involves:
		- Streaming repository contents from Gitserver (faked)
		- Invoke scip-syntax CLI in tar streaming mode and pass repo contents to it (real)
		- Upload compressed SCIP index to specified upload store (which is faked)

		To confirm this works end to end, we invoke the worker's handler and then verify that:
		1. Upload store received a valid gzip compressed SCIP index with paths we expect
		2. Uploads table has now contains a record for this particular upload

		We don't verify the rest of the pipeline works - namely upload processing, as this should be handled
		by the upload processor's own tests.
	*/
	logger := logtest.Scoped(t)
	sqlDB := dbtest.NewDB(t)
	db := database.NewDB(logger, sqlDB)

	context := context.Background()
	observationCtx := observation.TestContextTB(t)

	config := IndexingWorkerConfig{}

	jobStore, err := jobstore.NewStoreWithDB(observationCtx, sqlDB)
	require.NoError(t, err)

	gitserverClient := gitserver.NewMockClient()
	uploadStore := testutils.NewMockUploadStore()

	// Note that this method will read the environment variables,
	// and the most important one - SCIP_SYNTAX_PATH - will be placed there
	// by Bazel. If you want to run this test separately as part of `go test`,
	// you need to handle the environment yourself.
	config.Load()

	// If we're running in Bazel test environment, resolve the path to CLI
	// correctly. Otherwise we will rely on default value, so no need to handle
	// else branch.
	if os.Getenv("BAZEL_TEST") != "" {
		fmt.Println("we're here")
		config.CliPath, _ = runfiles.Rlocation(os.Getenv("SCIP_SYNTAX_PATH"))
	}

	indexingWorker, err := NewIndexingHandler(
		context,
		observationCtx,
		jobStore,
		config,
		db,
		stores.NewCodeIntelDB(logger, sqlDB),
		gitserverClient,
		uploadStore,
	)
	require.NoError(t, err)

	contents := map[string]string{
		"/test/my/file.java": "package org.sourcegraph.winning;",
	}
	tar, err := createTarArchive(contents)
	require.NoError(t, err)

	gitserverClient.ArchiveReaderFunc.SetDefaultReturn(tar, nil)

	job := jobstore.SyntacticIndexingJob{
		ID:             1,
		Commit:         testutils.MakeCommit(1),
		RepositoryID:   1,
		RepositoryName: "tangy/tacos",
		State:          jobstore.Queued,
	}

	// This will ensure that the repo is created before the
	// record itself is inserted
	testutils.InsertSyntacticIndexingRecords(t, db, job)

	err = indexingWorker.Handle(context, logger, &job)
	require.NoError(t, err)

	allFilesIterator, err := uploadStore.List(context, "")
	require.NoError(t, err)
	allFiles, err := iterator.Collect(allFilesIterator)
	require.NoError(t, err)
	require.Len(t, allFiles, 1)

	uploadedContents, err := uploadStore.Get(context, allFiles[0])
	require.NoError(t, err)

	index, err := readGzippedSCIPIndex(t, uploadedContents)
	require.NoError(t, err)

	require.Equal(t, index.Metadata.ToolInfo.Name, "scip-syntax")
	require.Equal(t, index.Documents[0].RelativePath, "/test/my/file.java")
}

func readGzippedSCIPIndex(t *testing.T, reader io.Reader) (*scip.Index, error) {
	var buf bytes.Buffer
	gzipReader, err := gzip.NewReader(reader)
	require.NoError(t, err)

	_, err = io.Copy(&buf, gzipReader)
	require.NoError(t, err)

	scipIndex := scip.Index{}
	err = proto.Unmarshal(buf.Bytes(), &scipIndex)
	if err != nil {
		return nil, err
	}
	return &scipIndex, nil
}

func createTarArchive(files map[string]string) (io.ReadCloser, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for path, contents := range files {
		hdr := &tar.Header{
			Name: path,
			Mode: 0600,
			Size: int64(len(contents)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(contents)); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return io.NopCloser(&buf), nil
}
