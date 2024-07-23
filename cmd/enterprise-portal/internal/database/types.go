package database

import "database/sql"

// NewNullString creates an *sql.NullString that indicates "invalid", i.e. null,
// if v is an empty string. It returns a pointer because many use cases require
// a pointer - it is safe to immediately deref the return value if you need to,
// since it always returns a non-nil value.
func NewNullString(v string) *sql.NullString {
	return &sql.NullString{
		String: v,
		Valid:  v != "",
	}
}

// NewNullString creates an *sql.NullString that indicates "invalid", i.e. null,
// if v is nil or an empty string. It returns a pointer because many use cases
// require a pointer - it is safe to immediately deref the return value if you
// need to, since it always returns a non-nil value.
func NewNullStringPtr(v *string) *sql.NullString {
	if v == nil {
		return &sql.NullString{}
	}
	return NewNullString(*v)
}

// NewNullInt32 is like NewNullString, but always produces a valid value.
func NewNullInt32[T int | int32 | int64 | uint64](v T) *sql.NullInt32 {
	return &sql.NullInt32{
		Int32: int32(v),
		Valid: true,
	}
}

// NewNullInt32 is like NewNullString, but always produces a valid value.
func NewNullInt64[T int | int32 | int64 | uint64](v T) *sql.NullInt64 {
	return &sql.NullInt64{
		Int64: int64(v),
		Valid: true,
	}
}
