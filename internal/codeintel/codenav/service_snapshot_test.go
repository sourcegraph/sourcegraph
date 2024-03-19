package codenav

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/sourcegraph/scip/bindings/go/scip"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

const sampleFile1 = `package food

type banana struct{}`

func TestSnapshotForDocument(t *testing.T) {
	// Set up mocks
	mockRepoStore := defaultMockRepoStore()
	mockLsifStore := NewMockLsifStore()
	mockUploadSvc := NewMockUploadService()
	mockGitserverClient := gitserver.NewMockClient()

	// Init service
	svc := newService(&observation.TestContext, mockRepoStore, mockLsifStore, mockUploadSvc, mockGitserverClient)

	mockUploadSvc.GetCompletedUploadsByIDsFunc.SetDefaultReturn([]shared.CompletedUpload{{}}, nil)
	mockRepoStore.GetFunc.SetDefaultReturn(&types.Repo{}, nil)
	mockGitserverClient.NewFileReaderFunc.SetDefaultReturn(io.NopCloser(bytes.NewReader([]byte(sampleFile1))), nil)
	mockLsifStore.SCIPDocumentFunc.SetDefaultReturn(&scip.Document{
		RelativePath: "burger.go",
		Occurrences: []*scip.Occurrence{{
			Range:       []int32{2, 4, 9},
			Symbol:      "scip-go gomod github.com/sourcegraph/banter v4.2.0 github.com/sourcegraph/banter/food/banana#",
			SymbolRoles: int32(scip.SymbolRole_Definition),
		}},
		Symbols: []*scip.SymbolInformation{{
			Symbol: "scip-go gomod github.com/sourcegraph/banter v4.2.0 github.com/sourcegraph/banter/food/banana#",
			Relationships: []*scip.Relationship{{
				Symbol:           "scip-go gomod github.com/golang/go go1.18 fmt/Banterer#",
				IsImplementation: true,
			}},
		}},
	}, nil)

	data, err := svc.SnapshotForDocument(context.Background(), 0, "deadbeef", "burger.go", 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 1 {
		t.Fatal("no snapshot data returned")
	}

	if data[0].DocumentOffset != 35 {
		t.Fatalf("unexpected document offset (want=%d,got=%d)", 35, data[0].DocumentOffset)
	}
}
