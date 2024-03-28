package lsifstore

import (
	"context"
	"testing"

	"github.com/sourcegraph/log/logtest"
	"github.com/sourcegraph/scip/bindings/go/scip"

	codeintelshared "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

func TestInsertMetadata(t *testing.T) {
	logger := logtest.Scoped(t)
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, codeIntelDB)
	ctx := context.Background()

	if err := store.InsertMetadata(ctx, 42, ProcessedMetadata{
		TextDocumentEncoding: "UTF8",
		ToolName:             "scip-test",
		ToolVersion:          "0.1.0",
		ToolArguments:        []string{"-p", "src"},
		ProtocolVersion:      1,
	}); err != nil {
		t.Fatalf("failed to insert metadata: %s", err)
	}
}

func TestInsertSharedDocumentsConcurrently(t *testing.T) {
	logger := logtest.Scoped(t)
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := newInternal(&observation.TestContext, codeIntelDB)
	ctx := context.Background()

	tx1, err := store.Transact(ctx)
	if err != nil {
		t.Fatalf("failed to start transaction: %s", err)
	}
	scipWriter24, err := tx1.NewPreciseSCIPWriter(ctx, 24)
	if err != nil {
		t.Fatalf("failed to create SCIP writer: %s", err)
	}
	if err := scipWriter24.InsertDocument(
		ctx,
		"internal/util.go",
		&scip.Document{
			Symbols: []*scip.SymbolInformation{
				{Symbol: "lorem ipsum dolor sit amet"},
			},
		},
	); err != nil {
		t.Fatalf("failed to write SCIP document: %s", err)
	}
	if _, err := scipWriter24.Flush(ctx); err != nil {
		t.Fatalf("failed to flush SCIP data: %s", err)
	}
	if err := tx1.Done(nil); err != nil {
		t.Fatalf("failed to commit transaction: %s", err)
	}

	tx2, err := store.Transact(ctx)
	if err != nil {
		t.Fatalf("failed to start transaction: %s", err)
	}
	scipWriter25, err := tx2.NewPreciseSCIPWriter(ctx, 25)
	if err != nil {
		t.Fatalf("failed to create SCIP writer: %s", err)
	}
	if err := scipWriter25.InsertDocument(
		ctx,
		"internal/util.go",
		&scip.Document{
			Symbols: []*scip.SymbolInformation{
				{Symbol: "lorem ipsum dolor sit amet"},
			},
		},
	); err != nil {
		t.Fatalf("failed to write SCIP document: %s", err)
	}
	if err := scipWriter25.InsertDocument(
		ctx,
		"internal/util_test.go",
		&scip.Document{
			Symbols: []*scip.SymbolInformation{
				{Symbol: "consectetur adipiscing elit, sed do eiusmod"},
			},
		},
	); err != nil {
		t.Fatalf("failed to write SCIP document: %s", err)
	}
	if _, err := scipWriter25.Flush(ctx); err != nil {
		t.Fatalf("failed to flush SCIP data: %s", err)
	}
	if err := tx2.Done(nil); err != nil {
		t.Fatalf("failed to commit transaction: %s", err)
	}

	count, _, err := basestore.ScanFirstInt(codeIntelDB.Handle().QueryContext(ctx, `SELECT COUNT(*) FROM codeintel_scip_documents`))
	if err != nil {
		t.Fatalf("failed to query number of SCIP documents: %s", err)
	} else if expected := 2; count != expected {
		t.Fatalf("unexpected number of documents. want=%d have=%d", expected, count)
	}
}

func TestInsertDocumentWithSymbols(t *testing.T) {
	logger := logtest.Scoped(t)
	codeIntelDB := codeintelshared.NewCodeIntelDB(logger, dbtest.NewDB(t))
	store := New(&observation.TestContext, codeIntelDB)
	ctx := context.Background()

	var n uint32
	if err := store.WithTransaction(ctx, func(tx Store) error {
		scipWriter24, err := tx.NewPreciseSCIPWriter(ctx, 24)
		if err != nil {
			t.Fatalf("failed to write SCIP symbols: %s", err)
		}

		if err := scipWriter24.InsertDocument(
			ctx,
			"internal/util.go",
			&scip.Document{
				Symbols: []*scip.SymbolInformation{
					{Symbol: "foo.bar.ident"},
					{Symbol: "bar.baz.longerName"},
					{Symbol: "baz.bonk.quux"},
				},
				Occurrences: []*scip.Occurrence{
					{
						Range:       []int32{3, 25, 3, 30},
						Symbol:      "foo.bar.ident",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:       []int32{251, 24, 251, 30},
						Symbol:      "baz.bonk.quux",
						SymbolRoles: int32(scip.SymbolRole_Definition),
					},
					{
						Range:       []int32{4, 25, 4, 30},
						Symbol:      "foo.bar.ident",
						SymbolRoles: 0,
					},
					{
						Range:       []int32{100, 10, 100, 20},
						Symbol:      "bar.baz.longerName",
						SymbolRoles: 0,
					},
					{
						Range:       []int32{151, 14, 151, 20},
						Symbol:      "baz.bonk.quux",
						SymbolRoles: 0,
					},
				},
			},
		); err != nil {
			t.Fatalf("failed to write SCIP document: %s", err)
		}

		n, err = scipWriter24.Flush(ctx)
		if err != nil {
			t.Fatalf("failed to write SCIP symbols: %s", err)
		}

		return nil
	}); err != nil {
		t.Fatalf("failed to commit transaction: %s", err)
	}

	if expected := uint32(3); n != expected {
		t.Fatalf("unexpected number of symbols inserted. want=%d have=%d", expected, n)
	}
}
