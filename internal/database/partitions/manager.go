package partitions

import (
	"context"
	"fmt"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

type PartitionManager[T PartitionKey] struct {
	db          dbutil.DB
	sourceTable string
	strategy    PartitionStrategy[T]
}

type PartitionStrategy[T PartitionKey] interface {
	FormatValuesClause(partitionKey T) string
}

type PartitionKey interface {
	Name() string
}

func NewPartitionManager[T PartitionKey](db dbutil.DB, sourceTable string, strategy PartitionStrategy[T]) *PartitionManager[T] {
	return &PartitionManager[T]{
		db:          db,
		sourceTable: sourceTable,
		strategy:    strategy,
	}
}

func (m *PartitionManager[T]) EnsurePartition(ctx context.Context, partitionKey T) error {
	return m.exec(ctx, sqlf.Sprintf(fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s PARTITION OF %s FOR VALUES %s`,
		m.PartitionTableNameFor(partitionKey),
		m.sourceTable,
		m.strategy.FormatValuesClause(partitionKey),
	)))
}

func (m *PartitionManager[T]) DeletePartition(ctx context.Context, partitionKey T) error {
	return m.exec(ctx, sqlf.Sprintf(fmt.Sprintf(
		`DROP TABLE IF EXISTS %s`,
		m.PartitionTableNameFor(partitionKey),
	)))
}

func (m *PartitionManager[T]) AttachPartition(ctx context.Context, partitionKey T) error {
	return m.exec(ctx, sqlf.Sprintf(fmt.Sprintf(
		`ALTER TABLE %s ATTACH PARTITION %s FOR VALUES %s`,
		m.sourceTable,
		m.PartitionTableNameFor(partitionKey),
		m.strategy.FormatValuesClause(partitionKey),
	)))
}

func (m *PartitionManager[T]) DetachPartition(ctx context.Context, partitionKey T) error {
	return m.exec(ctx, sqlf.Sprintf(fmt.Sprintf(
		`ALTER TABLE %s DETACH PARTITION %s`,
		m.sourceTable,
		m.PartitionTableNameFor(partitionKey),
	)))
}

func (m *PartitionManager[T]) PartitionTableNameFor(partitionKey T) string {
	return fmt.Sprintf("%s_%s", m.sourceTable, partitionKey.Name())
}

func (m *PartitionManager[T]) exec(ctx context.Context, query *sqlf.Query) error {
	_, err := m.db.ExecContext(ctx, query.Query(sqlf.PostgresBindVar), query.Args()...)
	return err
}
