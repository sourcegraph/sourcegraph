package partitions

import (
	"fmt"
)

type listPartitionStrategy[T ListBound] struct{}

func NewListPartitionStrategy[T ListBound]() PartitionStrategy[ListPartitionKey[T]] {
	return &listPartitionStrategy[T]{}
}

func (m *listPartitionStrategy[T]) FormatValuesClause(partitionKey ListPartitionKey[T]) string {
	return fmt.Sprintf(`IN ('%s')`, partitionKey.Value.String()) // TODO - sanitize
}

type ListPartitionKey[T ListBound] struct {
	Value T
}

type ListBound interface {
	fmt.Stringer
}

func (k ListPartitionKey[T]) Name() string {
	// TODO - sanitize
	return fmt.Sprintf("%s", k.Value)
}

//
//

type String string

func (s String) String() string { return string(s) }
