package symbols

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/inconshreveable/log15"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/protocol"
	"github.com/sourcegraph/sourcegraph/cmd/symbols/internal/sqliteutil"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
)

func BenchmarkSearch(b *testing.B) {
	sqliteutil.MustRegisterSqlite3WithPcre()

	log15.Root().SetHandler(log15.LvlFilterHandler(log15.LvlError, log15.Root().GetHandler()))

	service := Service{
		FetchTar:  testutil.FetchTarFromGithub,
		NewParser: NewParser,
		Path:      "/tmp/symbols-cache",
	}
	if err := service.Start(); err != nil {
		b.Fatal(err)
	}

	ctx := context.Background()
	b.ResetTimer()

	indexTests := []protocol.SearchArgs{
		{Repo: "github.com/sourcegraph/go-langserver", CommitID: "391a062a7d9977510e7e883e412769b07fed8b5e"},
		{Repo: "github.com/moby/moby", CommitID: "6e5c2d639f67ae70f54d9f2285f3261440b074aa"},
	}

	queryTests := []protocol.SearchArgs{
		{Repo: "github.com/sourcegraph/go-langserver", CommitID: "391a062a7d9977510e7e883e412769b07fed8b5e", Query: "^sortedImportRecord$", First: 10},
		{Repo: "github.com/sourcegraph/go-langserver", CommitID: "391a062a7d9977510e7e883e412769b07fed8b5e", Query: "1234doesnotexist1234", First: 1},
		{Repo: "github.com/moby/moby", CommitID: "6e5c2d639f67ae70f54d9f2285f3261440b074aa", Query: "^fsCache$", First: 10},
		{Repo: "github.com/moby/moby", CommitID: "6e5c2d639f67ae70f54d9f2285f3261440b074aa", Query: "1234doesnotexist1234", First: 1},
	}

	runIndexTest := func(test protocol.SearchArgs) {
		b.Run(fmt.Sprintf("indexing %s@%s", path.Base(string(test.Repo)), test.CommitID[:3]), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				tempFile, err := ioutil.TempFile("", "")
				if err != nil {
					b.Fatal(err)
				}
				defer os.Remove(tempFile.Name())
				err = service.writeAllSymbolsToNewDB(ctx, tempFile.Name(), test.Repo, test.CommitID)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	runQueryTest := func(test protocol.SearchArgs) {
		b.Run(fmt.Sprintf("searching %s@%s %s", path.Base(string(test.Repo)), test.CommitID[:3], test.Query), func(b *testing.B) {
			_, err := service.search(ctx, test)
			if err != nil {
				b.Fatal(err)
			}
			b.ResetTimer()
			for n := 0; n < b.N; n++ {
				_, err := service.search(ctx, test)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	for _, test := range indexTests {
		runIndexTest(test)
	}

	for _, test := range queryTests {
		runQueryTest(test)
	}
}
