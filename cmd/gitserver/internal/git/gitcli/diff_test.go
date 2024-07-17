package gitcli

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestGitCLIBackend_RawDiff(t *testing.T) {
	var f1Diff = []byte(`diff --git f f
index a29bdeb434d874c9b1d8969c40c42161b03fafdc..c0d0fb45c382919737f8d0c20aaf57cf89b74af8 100644
--- f
+++ f
@@ -1 +1,2 @@
 line1
+line2
`)
	var f2Diff = []byte(`diff --git f2 f2
new file mode 100644
index 0000000000000000000000000000000000000000..8a6a2d098ecaf90105f1cf2fa90fc4608bb08067
--- /dev/null
+++ f2
@@ -0,0 +1 @@
+line2
`)
	ctx := context.Background()

	// Prepare repo state:
	backend := BackendWithRepoCommands(t,
		"echo line1 > f",
		"git add f",
		"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		"git tag testbase",
		"echo line2 >> f",
		"git add f",
		"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
	)

	t.Run("streams diff", func(t *testing.T) {
		r, err := backend.RawDiff(ctx, "testbase", "HEAD", git.GitDiffComparisonTypeOnlyInHead)
		require.NoError(t, err)
		diff, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		require.Equal(t, string(f1Diff), string(diff))
	})
	t.Run("streams diff intersection", func(t *testing.T) {
		r, err := backend.RawDiff(ctx, "testbase", "HEAD", git.GitDiffComparisonTypeIntersection)
		require.NoError(t, err)
		diff, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		require.Equal(t, string(f1Diff), string(diff))
	})
	t.Run("streams diff for path", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f",
			"git add f",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
			"git tag testbase",
			"echo line2 >> f2",
			"git add f2",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		)

		r, err := backend.RawDiff(ctx, "testbase", "HEAD", git.GitDiffComparisonTypeOnlyInHead, "f2")
		require.NoError(t, err)
		diff, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		// We expect only a diff for f2, not for f.
		require.Equal(t, string(f2Diff), string(diff))
	})
	t.Run("not found revspec", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f",
			"git add f",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
			"git tag test",
		)

		_, err := backend.RawDiff(ctx, "unknown", "test", git.GitDiffComparisonTypeOnlyInHead)
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))

		_, err = backend.RawDiff(ctx, "test", "unknown", git.GitDiffComparisonTypeOnlyInHead)
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})
	t.Run("files outside repository", func(t *testing.T) {
		// We use git-diff-tree, but with git-diff you can diff any files on disk
		// which is dangerous. So we have this safeguard test here in place to
		// make sure we don't regress on that.
		r, err := backend.RawDiff(ctx, "testbase", "HEAD", git.GitDiffComparisonTypeOnlyInHead, "/dev/null", "/etc/hosts")
		require.NoError(t, err)
		_, err = io.ReadAll(r)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is outside repository at")
	})
	// Verify that if the context is canceled, the reader returns an error.
	t.Run("context cancelation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		t.Cleanup(cancel)

		r, err := backend.RawDiff(ctx, "testbase", "HEAD", git.GitDiffComparisonTypeOnlyInHead)
		require.NoError(t, err)

		cancel()

		_, err = io.ReadAll(r)
		require.Error(t, err)
		require.True(t, errors.Is(err, context.Canceled), "unexpected error: %v", err)

		require.True(t, errors.Is(r.Close(), context.Canceled), "unexpected error: %v", err)
	})
}

func TestGitCLIBackend_ChangedFiles(t *testing.T) {
	ctx := context.Background()

	t.Run("added, modified, renamed, and deleted files", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f1",
			"echo line1 > f2",
			"echo line1 > f3",
			"echo line1 > oldname",
			"echo line1 > 'file with space'",
			"echo line1 > 'file_with_😊_emoji'",
			"git add f1 f2 f3 oldname",
			"git commit -m base --author='Foo Author <foo@sourcegraph.com>'",

			"git tag testbase",

			"echo line1 > 'file with space'",
			"echo line1 > 'file_with_😊_emoji'",
			"git add 'file with space' 'file_with_😊_emoji'",
			"echo line2 >> f1",
			"echo line2 >> f2",
			"git add f1 f2",
			"git rm f3",
			"echo line1 > f4",
			"git add f4",
			"mkdir d1",
			"echo line1 > d1/f1",
			"git add d1/f1",
			"git mv oldname newname",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		)

		expectedChanges := []gitdomain.PathStatus{
			{Path: "d1/f1", Status: gitdomain.StatusAdded},
			{Path: "f4", Status: gitdomain.StatusAdded},
			{Path: "newname", Status: gitdomain.StatusAdded},
			{Path: "f1", Status: gitdomain.StatusModified},
			{Path: "f2", Status: gitdomain.StatusModified},
			{Path: "f3", Status: gitdomain.StatusDeleted},
			{Path: "oldname", Status: gitdomain.StatusDeleted},
			{Path: "file with space", Status: gitdomain.StatusAdded},
			{Path: "file_with_😊_emoji", Status: gitdomain.StatusAdded},
		}

		for _, tc := range []struct {
			name       string
			base, head string
		}{
			{
				name: "base is a tag",
				base: "testbase",
				head: "HEAD",
			},
			{
				name: "base is empty (implies parent of HEAD)",
				base: "",
				head: "HEAD",
			},
			{
				name: "base is explicit parent of HEAD",
				base: "HEAD^",
				head: "HEAD",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				iterator, err := backend.ChangedFiles(ctx, tc.base, tc.head)
				require.NoError(t, err)
				defer iterator.Close()

				var changes []gitdomain.PathStatus
				for {
					change, err := iterator.Next()
					if err == io.EOF {
						break
					}

					require.NoError(t, err)
					changes = append(changes, change)
				}

				sort.Slice(changes, func(i, j int) bool {
					return cmpPathStatus(changes[i], changes[j])
				})
				sort.Slice(expectedChanges, func(i, j int) bool {
					return cmpPathStatus(expectedChanges[i], expectedChanges[j])
				})

				if diff := cmp.Diff(expectedChanges, changes, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("unexpected changes (-want +got):\n%s", diff)
				}
			})
		}
	})

	t.Run("no changes", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f",
			"git add f",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
			"git tag testbase",
		)

		iterator, err := backend.ChangedFiles(ctx, "testbase", "HEAD")
		require.NoError(t, err)
		defer iterator.Close()

		_, err = iterator.Next()
		require.Equal(t, io.EOF, err)
	})

	t.Run("invalid base", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f",
			"git add f",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		)

		_, err := backend.ChangedFiles(ctx, "invalid", "HEAD")
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	t.Run("invalid head", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f",
			"git add f",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
			"git tag testbase",
		)

		_, err := backend.ChangedFiles(ctx, "testbase", "invalid")
		require.Error(t, err)
		require.True(t, errors.HasType[*gitdomain.RevisionNotFoundError](err))
	})

	t.Run("empty base and single commit", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f",
			"git add f",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",
		)

		iterator, err := backend.ChangedFiles(ctx, "", "HEAD")
		require.NoError(t, err)
		defer iterator.Close()

		var actualChanges []gitdomain.PathStatus
		for {
			change, err := iterator.Next()
			if err == io.EOF {
				break
			}

			require.NoError(t, err)
			actualChanges = append(actualChanges, change)
		}

		expectedChanges := []gitdomain.PathStatus{
			{Path: "f", Status: gitdomain.StatusAdded},
		}

		if diff := cmp.Diff(expectedChanges, actualChanges, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("unexpected changes (-want +got):\n%s", diff)
		}

	})

	t.Run("changed type", func(t *testing.T) {
		// Prepare repo state:
		backend := BackendWithRepoCommands(t,
			"echo line1 > f1",
			"git add -A",
			"git commit -m foo --author='Foo Author <foo@sourcegraph.com>'",

			"mv f1 f2",
			"ln -s f2 f1",
			"git add -A",
			"git commit -m bar --author='Bar Author <foo@sourcegraph.com>'",
		)

		iterator, err := backend.ChangedFiles(ctx, "", "HEAD")
		require.NoError(t, err)
		defer iterator.Close()

		var actualChanges []gitdomain.PathStatus
		for {
			change, err := iterator.Next()
			if err == io.EOF {
				break
			}

			require.NoError(t, err)
			actualChanges = append(actualChanges, change)
		}

		expectedChanges := []gitdomain.PathStatus{
			{Path: "f1", Status: gitdomain.StatusTypeChanged},
			{Path: "f2", Status: gitdomain.StatusAdded},
		}

		if diff := cmp.Diff(expectedChanges, actualChanges, cmpopts.EquateEmpty()); diff != "" {
			t.Errorf("unexpected changes (-want +got):\n%s", diff)
		}

	})
}

const NUL byte = 0

//go:embed fixtures/git-diff-java-langserver/output.hex
var goJavaLangserverDiff []byte

func TestGitDiffIterator(t *testing.T) {
	testCases := []struct {
		name string

		output          []byte
		expectedChanges []gitdomain.PathStatus
	}{
		{
			name: "added, modified, and deleted files",
			output: combineBytes(
				[]byte("A"), []byte{NUL}, []byte("added1.json"), []byte{NUL},
				[]byte("M"), []byte{NUL}, []byte("modified1.json"), []byte{NUL},
				[]byte("D"), []byte{NUL}, []byte("deleted1.json"), []byte{NUL},
				[]byte("A"), []byte{NUL}, []byte("added2.json"), []byte{NUL},
				[]byte("M"), []byte{NUL}, []byte("modified2.json"), []byte{NUL},
				[]byte("D"), []byte{NUL}, []byte("deleted2.json"), []byte{NUL},
				[]byte("A"), []byte{NUL}, []byte("added3.json"), []byte{NUL},
				[]byte("M"), []byte{NUL}, []byte("modified3.json"), []byte{NUL},
				[]byte("D"), []byte{NUL}, []byte("deleted3.json"), []byte{NUL},
			),
			expectedChanges: []gitdomain.PathStatus{
				{Path: "added1.json", Status: gitdomain.StatusAdded},
				{Path: "added2.json", Status: gitdomain.StatusAdded},
				{Path: "added3.json", Status: gitdomain.StatusAdded},
				{Path: "modified1.json", Status: gitdomain.StatusModified},
				{Path: "modified2.json", Status: gitdomain.StatusModified},
				{Path: "modified3.json", Status: gitdomain.StatusModified},
				{Path: "deleted1.json", Status: gitdomain.StatusDeleted},
				{Path: "deleted2.json", Status: gitdomain.StatusDeleted},
				{Path: "deleted3.json", Status: gitdomain.StatusDeleted},
			},
		},
		{
			name:   "empty output",
			output: []byte{},
		},
		{
			name: "realworld-example output from git diff java-langserver: ensure that status text doesn't point to mutable slice",
			// There was a bug where the status text was fetched via scanner.Bytes() which doesn't allocate a new slice.
			// This caused the status text to be possibly overwritten by the next call to scanner.Scan() when reading the file path.
			//
			// This test uses the output of a real-world git diff command to ensure that we don't run into the above issue again.
			output: goJavaLangserverDiff,
			expectedChanges: []gitdomain.PathStatus{
				{Path: ".gitmodules", Status: gitdomain.StatusModified},
				{Path: ".idea/compiler.xml", Status: gitdomain.StatusModified},
				{Path: ".idea/vcs.xml", Status: gitdomain.StatusModified},
				{Path: "pom.xml", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/App2.java", Status: gitdomain.StatusDeleted},
				{Path: "src/main/java/com/sourcegraph/langserver/LSPWebSocketServer.java", Status: gitdomain.StatusDeleted},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/JavacLanguageServer.java", Status: gitdomain.StatusDeleted},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/LanguageService.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/compiler/PositionCalculator.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/files/HTTPUtil.java", Status: gitdomain.StatusDeleted},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/files/RemoteFileContentProvider.java", Status: gitdomain.StatusDeleted},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/maven/MavenUtil.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/workspace/JavaConfigWorkspace.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/workspace/MavenWorkspace.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/workspace/Workspace.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/workspace/WorkspaceSourceFileProvider.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/langserver/langservice/workspace/Workspaces.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/lsp/Controller.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/lsp/NoopMessenger.java", Status: gitdomain.StatusDeleted},
				{Path: "src/main/java/com/sourcegraph/lsp/domain/structures/TextDocumentIdentifier.java", Status: gitdomain.StatusModified},
				{Path: "src/main/java/com/sourcegraph/utils/LanguageUtils.java", Status: gitdomain.StatusModified},
				{Path: "src/test/resources/AndroidViewAnimations", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/AppIntro", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/EventBus", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/EventBus/EventBus/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/EventBus/EventBusAnnotationProcessor/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/EventBus/EventBusPerformance/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/EventBus/EventBusTest/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/EventBus/EventBusTestSubscriberInJar/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/EventBus/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/MPAndroidChart/MPChartExample/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/MPAndroidChart/MPChartLib/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/MPAndroidChart/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/benchmarks/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/build-tools/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/client/benchmark/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/client/client-benchmark-noop-api-plugin/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/client/rest-high-level/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/client/rest/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/client/sniffer/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/client/test/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/client/transport/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/core/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/distribution/deb/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/distribution/integ-test-zip/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/distribution/rpm/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/distribution/tar/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/distribution/tools/java-version-checker/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/distribution/zip/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/docs/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/aggs-matrix-stats/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/ingest-common/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/lang-expression/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/lang-mustache/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/lang-painless/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/percolator/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/reindex/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/repository-url/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/modules/transport-netty4/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/analysis-icu/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/analysis-kuromoji/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/analysis-phonetic/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/analysis-smartcn/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/analysis-stempel/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/analysis-ukrainian/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/discovery-azure-classic/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/discovery-ec2/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/discovery-file/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/discovery-gce/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/ingest-attachment/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/ingest-geoip/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/ingest-user-agent/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/jvm-example/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/mapper-murmur3/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/mapper-size/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/repository-azure/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/repository-gcs/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/repository-hdfs/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/repository-s3/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/plugins/store-smb/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/backwards-5.0/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/evil-tests/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/multi-cluster-search/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/no-bootstrap-tests/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/rolling-upgrade/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-client/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-http/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-ingest-disabled/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-ingest-with-all-dependencies/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-multinode/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-plugins/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-reindex-with-painless/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/smoke-test-tribe-node/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/qa/vagrant/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/rest-api-spec/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/test/fixtures/example-fixture/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/test/fixtures/hdfs-fixture/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/test/framework/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/elasticsearch/test/logger-usage/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/gradle_custom_repo/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/ion/ion-sample/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/ion/ion/AndroidAsync/AndroidAsync/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/ion/ion/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/ion/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/json-patch/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/leakcanary/leakcanary-analyzer/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/leakcanary/leakcanary-android-no-op/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/leakcanary/leakcanary-android/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/leakcanary/leakcanary-sample/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/leakcanary/leakcanary-watcher/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/leakcanary/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/mockito/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/mockito/subprojects/android/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/mockito/subprojects/extTest/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/mockito/subprojects/inline/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/mockito/subprojects/testng/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/picasso/picasso-pollexor/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/picasso/picasso-sample/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/picasso/picasso/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/picasso/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/annotations/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/compat/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/core-ui/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/core-utils/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/customtabs/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/design/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/exifinterface/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/fragment/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/frameworks/support/samples/SupportLeanbackShowcase/app/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/frameworks/support/samples/SupportLeanbackShowcase/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/graphics/drawable/animated/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/graphics/drawable/static/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/media-compat/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/percent/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/recommendation/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/Support13Demos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/Support4Demos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/Support7Demos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/SupportDesignDemos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/SupportLeanbackDemos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/SupportLeanbackShowcase/app/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/SupportLeanbackShowcase/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/SupportPercentDemos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/SupportPreferenceDemos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/samples/SupportTransitionDemos/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/transition/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v13/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v14/preference/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v17/leanback/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v17/preference-leanback/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v4/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v7/appcompat/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v7/cardview/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v7/gridlayout/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v7/mediarouter/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v7/palette/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v7/preference/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/platform_frameworks_support/v7/recyclerview/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/priam/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/priam/priam-agent/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/priam/priam-cass-extensions/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/priam/priam-dse-extensions/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/priam/priam-web/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/priam/priam/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/third-party/aosp-dexutils/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/third-party/bsdiff-util/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-android/tinker-android-anno/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-android/tinker-android-lib/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-android/tinker-android-loader/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-build/tinker-patch-cli/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-build/tinker-patch-gradle-plugin/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-build/tinker-patch-lib/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-commons/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-sample-android/app/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/GRADLE2POM/tinker/tinker-sample-android/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/JSON-java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/MPAndroidChart", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/_EffectivePomTestData/one/effective-pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/_EffectivePomTestData/one/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/_EffectivePomTestData/two/effective-pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/_EffectivePomTestData/two/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/android-sdk", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/axis2-java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/commons-io", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/commons-lang", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/cup-of-joe", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/dropwizard", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/dropwizard-snippets", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/elasticsearch", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/expJavaConfigs/MPAndroidChart", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/expJavaConfigs/leakcanary", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/ANT/test-artifact/test-artifact/content/src/main/java/com/sourcegraph/main/Main.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/ANT/test-artifact/test-artifact/content/src/main/java/com/sourcegraph/util/Util.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/ANT/test-artifact/test-artifact/files.txt", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/GRADLE/test-artifact/content/src/main/java/com/sourcegraph/main/Main.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/GRADLE/test-artifact/content/src/main/java/com/sourcegraph/util/Util.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/GRADLE/test-artifact/files.txt", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/MAVEN/test-artifact/content/src/main/java/com/sourcegraph/main/Main.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/MAVEN/test-artifact/content/src/main/java/com/sourcegraph/util/Util.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/fileOverlay/MAVEN/test-artifact/files.txt", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/glide", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/gradle_custom_repo/build.gradle", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/guava", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/hackthon", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/ion", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-7", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-annotation-arguments", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-broken-pom/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-broken-pom/src/main/java/com/sourcegraph/broken/Main.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-broken-pom/sub/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-design-patterns", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-fake-guava", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-parent-pom", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-pom-with-broken-dependencies/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-pom-with-broken-dependencies/src/main/java/com/sourcegraph/simple/Main.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-pom-with-deps", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-pom-with-deps-javaconfig", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-pom-with-system-dependencies/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-pom-with-system-dependencies/src/main/java/com/sourcegraph/simple/Main.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-simple", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-simple-javaconfig", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-simple-pom", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-with-langtools/.gitignore", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-with-langtools/pom.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/java-with-langtools/src/main/java/com/sourcegraph/langtools/Main.java", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/jdom", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/json-patch", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/leakcanary", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/log4j", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/logback-test.xml", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/lombok", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/mockito", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/mockito-javaconfig", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/multi-module", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/openjdk8", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/openjdk8-langtools", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/picasso", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/platform_frameworks_support", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/priam", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/servlet-api", Status: gitdomain.StatusAdded},
				{Path: "src/test/resources/tinker", Status: gitdomain.StatusAdded},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			rc := io.NopCloser(bytes.NewReader(testCase.output))
			iterator := newChangedFilesIterator(rc)
			defer func() {
				err := iterator.Close()
				if err != nil {
					t.Fatalf("unexpected error closing iterator: %s", err)
				}
			}()

			var changes []gitdomain.PathStatus
			for {
				change, err := iterator.Next()
				if err == io.EOF {
					break
				}

				if err != nil {
					t.Fatalf("unexpected error parsing git diff output: %s", err)
				}

				changes = append(changes, change)
			}

			sort.Slice(testCase.expectedChanges, func(i, j int) bool {
				return cmpPathStatus(testCase.expectedChanges[i], testCase.expectedChanges[j])
			})
			sort.Slice(changes, func(i, j int) bool {
				return cmpPathStatus(changes[i], changes[j])
			})

			if diff := cmp.Diff(testCase.expectedChanges, changes); diff != "" {
				t.Errorf("unexpected changes (-want +got):\n%s", diff)
			}
		})
	}

	t.Run("uneven pairs", func(t *testing.T) {
		input := combineBytes(
			[]byte("A"), []byte{NUL}, []byte("file1.txt"), []byte{NUL},
			[]byte("M"), []byte{NUL}, []byte("file2.txt"), []byte{NUL},
			[]byte("D"), []byte{NUL},
		)
		rc := io.NopCloser(bytes.NewReader(input))
		iter := newChangedFilesIterator(rc)

		_, err := iter.Next()
		require.NoError(t, err)
		_, err = iter.Next()
		require.NoError(t, err)
		_, err = iter.Next()
		require.EqualError(t, err, "uneven pairs")
	})

	t.Run("unknown file status", func(t *testing.T) {
		input := combineBytes(
			[]byte("A"), []byte{NUL}, []byte("file1.txt"), []byte{NUL},
			[]byte("X"), []byte{NUL}, []byte("file2.txt"), []byte{NUL},
		)
		rc := io.NopCloser(bytes.NewReader(input))
		iter := newChangedFilesIterator(rc)

		_, err := iter.Next()
		require.NoError(t, err)
		_, err = iter.Next()
		require.Error(t, err, `encountered unknown file status "X" for file "file2.txt`)
	})

	t.Run("close", func(t *testing.T) {
		input := combineBytes(
			[]byte("A"), []byte{NUL}, []byte("file1.txt"), []byte{NUL},
			[]byte("M"), []byte{NUL}, []byte("file2.txt"), []byte{NUL},
			[]byte("D"), []byte{NUL}, []byte("file3.txt"), []byte{NUL},
		)
		rc := io.NopCloser(bytes.NewReader(input))
		iter := newChangedFilesIterator(rc)

		err := iter.Close()
		require.NoError(t, err)

		// Verify that Next returns io.EOF after the iterator is closed
		_, err = iter.Next()
		require.Equal(t, io.EOF, err)
	})

	t.Run("close iterator during iteration", func(t *testing.T) {
		input := combineBytes(
			[]byte("A"), []byte{NUL}, []byte("file1.txt"), []byte{NUL},
			[]byte("M"), []byte{NUL}, []byte("file2.txt"), []byte{NUL},
			[]byte("D"), []byte{NUL}, []byte("file3.txt"), []byte{NUL},
		)
		rc := io.NopCloser(bytes.NewReader(input))
		iter := newChangedFilesIterator(rc)

		// Iterate over the first change
		_, err := iter.Next()
		require.NoError(t, err)

		// Close the iterator during iteration
		err = iter.Close()
		require.NoError(t, err)

		// Verify that Next returns io.EOF after the iterator is closed
		_, err = iter.Next()
		require.Equal(t, io.EOF, err)
	})
}

func combineBytes(bss ...[]byte) (combined []byte) {
	for _, bs := range bss {
		combined = append(combined, bs...)
	}

	return combined
}

func cmpPathStatus(x, y gitdomain.PathStatus) bool {
	if x.Path == y.Path {
		return x.Status < y.Status
	}

	return x.Path < y.Path
}
