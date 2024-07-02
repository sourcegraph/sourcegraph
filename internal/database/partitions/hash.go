package partitions

import (
	"fmt"
)

type hashPartitionStrategy struct{}

func NewHashPartitionStrategy() PartitionStrategy[HashPartitionKey] {
	return &hashPartitionStrategy{}
}

func (m *hashPartitionStrategy) FormatValuesClause(partitionKey HashPartitionKey) string {
	return fmt.Sprintf(`WITH (MODULUS %d, REMAINDER %d)`, partitionKey.Modulus, partitionKey.Remainder)
}

type HashPartitionKey struct {
	Modulus   int
	Remainder int
}

func (k HashPartitionKey) Name() string {
	return fmt.Sprintf("%d_%d", k.Modulus, k.Remainder)
}
