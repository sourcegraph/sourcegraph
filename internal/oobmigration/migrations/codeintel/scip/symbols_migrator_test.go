package scip

import (
	"context"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"
	"github.com/sourcegraph/log/logtest"

	stores "github.com/sourcegraph/sourcegraph/internal/codeintel/shared"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbtest"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

func init() {
	symbolsMigratorConcurrencyLevel = 1
	symbolsMigratorSymbolRecordBatchSize = 100
}

func TestSymbolsMigratorUp(t *testing.T) {
	logger := logtest.Scoped(t)
	rawDB := dbtest.NewDB(logger, t)
	codeIntelDB := stores.NewCodeIntelDB(logger, rawDB)
	codeIntelStore := basestore.NewWithHandle(codeIntelDB.Handle())
	migrator := NewSCIPSymbolsMigrator(codeIntelStore)
	ctx := context.Background()

	contents, err := os.ReadFile("./testdata/trie.sql")
	if err != nil {
		t.Fatalf("unexpected error reading file: %s", err)
	}
	if _, err := codeIntelDB.ExecContext(ctx, string(contents)); err != nil {
		t.Fatalf("unexpected error executing test file: %s", err)
	}

	assertProgress := func(expectedProgress float64, applyReverse bool) {
		if progress, err := migrator.Progress(ctx, applyReverse); err != nil {
			t.Fatalf("unexpected error querying progress: %s", err)
		} else if progress != expectedProgress {
			t.Errorf("unexpected progress. want=%.2f have=%.2f", expectedProgress, progress)
		}
	}

	// Read all symbols accessible via trie traversal
	getTrieSymbols := func() map[int]string {
		m, err := scanIntStringMap(codeIntelStore.Query(ctx, sqlf.Sprintf(`
			WITH RECURSIVE
			symbols(id, upload_id, suffix, prefix_id) AS (
				(
					SELECT
						ssn.id,
						ssn.upload_id,
						ssn.name_segment AS suffix,
						ssn.prefix_id
					FROM codeintel_scip_symbol_names ssn
					WHERE ssn.id IN (SELECT symbol_id FROM codeintel_scip_symbols)
				) UNION (
					SELECT
						s.id,
						s.upload_id,
						ssn.name_segment || s.suffix AS suffix,
						ssn.prefix_id
					FROM symbols s
					JOIN codeintel_scip_symbol_names ssn ON
						ssn.upload_id = s.upload_id AND
						ssn.id = s.prefix_id
				)
			)
			SELECT ss.symbol_id, s.suffix AS symbol_name
			FROM codeintel_scip_symbols ss
			JOIN symbols s ON s.id = ss.symbol_id
			WHERE s.prefix_id IS NULL
		`)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		return m
	}

	// Read all symbols accessible via lookup table scan
	getLookupSymbols := func() map[int]string {
		m, err := scanIntStringMap(codeIntelStore.Query(ctx, sqlf.Sprintf(`
			SELECT
				s.symbol_id,
				-- ROUGHLY reconstruct symbol names from parts
				-- We don't want to do this on production paths as it ignores SCIP's escaping rules
				l1.name || ' ' || l2.name || ' ' || l3.name || ' ' || l4.name || ' ' || l5.name || l6.name AS symbol_name
			FROM codeintel_scip_symbols s
			JOIN codeintel_scip_symbols_lookup_leaves ll ON ll.upload_id = s.upload_id AND ll.symbol_id = s.symbol_id
			JOIN codeintel_scip_symbols_lookup l6 ON l6.upload_id = s.upload_id AND l6.id = ll.descriptor_suffix_id -- DESCRIPTOR_SUFFIX
			JOIN codeintel_scip_symbols_lookup l5 ON l5.upload_id = s.upload_id AND l5.id = l6.parent_id            -- DESCRIPTOR_NAMESPACE
			JOIN codeintel_scip_symbols_lookup l4 ON l4.upload_id = s.upload_id AND l4.id = l5.parent_id            -- PACKAGE_VERSION
			JOIN codeintel_scip_symbols_lookup l3 ON l3.upload_id = s.upload_id AND l3.id = l4.parent_id            -- PACKAGE_NAME
			JOIN codeintel_scip_symbols_lookup l2 ON l2.upload_id = s.upload_id AND l2.id = l3.parent_id            -- PACKAGE_MANAGER
			JOIN codeintel_scip_symbols_lookup l1 ON l1.upload_id = s.upload_id AND l1.id = l2.parent_id            -- SCHEME
		`)))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		return m
	}

	t.Run("up", func(t *testing.T) {
		// Initial state
		assertProgress(0, false)

		for i := 0; i < 128; i++ {
			if err := migrator.Up(ctx); err != nil {
				t.Fatalf("unexpected error performing up migration: %s", err)
			}
		}

		// End state (128 iterations should migrate everything)
		assertProgress(1, false)

		// Assert migrated state
		if diff := cmp.Diff(getTrieSymbols(), getLookupSymbols()); diff != "" {
			t.Errorf("unexpected symbol contents (-want +got):\n%s", diff)
		}
	})
}

var scanIntStringMap = basestore.NewMapScanner[int, string](func(s dbutil.Scanner) (k int, v string, _ error) {
	err := s.Scan(&k, &v)
	return k, v, err
})
