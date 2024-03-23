package codehosts

import (
	"database/sql/driver"

	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type nullInt struct{ N *int }

// newNullInt returns a NullInt treating zero value as null.
func newNullInt(i int) nullInt {
	if i == 0 {
		return nullInt{}
	}
	return nullInt{N: &i}
}

// Scan implements the Scanner interface.
func (n *nullInt) Scan(value any) error {
	switch value := value.(type) {
	case int64:
		*n.N = int(value)
	case int32:
		*n.N = int(value)
	case nil:
		return nil
	default:
		return errors.Errorf("value is not int: %T", value)
	}
	return nil
}

// Value implements the driver Valuer interface.
func (n nullInt) Value() (driver.Value, error) {
	if n.N == nil {
		return nil, nil
	}
	return *n.N, nil
}

type nullInt32 struct{ N **int32 }

// Scan implements the Scanner interface.
func (n *nullInt32) Scan(value any) error {
	switch value := value.(type) {
	case int64:
		*n.N = pointers.Ptr(int32(value))
	case int32:
		*n.N = pointers.Ptr(int32(value))
	case nil:
		return nil
	default:
		return errors.Errorf("value is not int: %T", value)
	}
	return nil
}

// Value implements the driver Valuer interface.
func (n nullInt32) Value() (driver.Value, error) {
	if n.N == nil {
		return nil, nil
	}
	return *n.N, nil
}
