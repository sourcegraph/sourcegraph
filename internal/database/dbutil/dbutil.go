package dbutil

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// A DB captures the methods shared between a *sql.DB and a *sql.Tx
type DB interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func IsPostgresError(err error, codename string) bool {
	var e *pgconn.PgError
	return errors.As(err, &e) && e.Code == codename
}

// NullTime represents a time.Time that may be null. nullTime implements the
// sql.Scanner interface, so it can be used as a scan destination, similar to
// sql.NullString. When the scanned value is null, Time is set to the zero value.
type NullTime struct{ *time.Time }

// Scan implements the Scanner interface.
func (nt *NullTime) Scan(value any) error {
	*nt.Time, _ = value.(time.Time)
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullTime) Value() (driver.Value, error) {
	if nt.Time == nil {
		return nil, nil
	}
	return *nt.Time, nil
}

// NullTimeColumn represents a timestamp that should be inserted/updated as NULL when t.IsZero() is true.
func NullTimeColumn(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// NullString represents a string that may be null. NullString implements the
// sql.Scanner interface, so it can be used as a scan destination, similar to
// sql.NullString. When the scanned value is null, String is set to the zero value.
type NullString struct{ S *string }

// NewNullString returns a NullString treating zero value as null.
func NewNullString(s string) NullString {
	if s == "" {
		return NullString{}
	}
	return NullString{S: &s}
}

// Scan implements the Scanner interface.
func (nt *NullString) Scan(value any) error {
	switch v := value.(type) {
	case []byte:
		*nt.S = string(v)
	case string:
		*nt.S = v
	}
	return nil
}

// Value implements the driver Valuer interface.
func (nt NullString) Value() (driver.Value, error) {
	if nt.S == nil {
		return nil, nil
	}
	return *nt.S, nil
}

// NullStringColumn represents a string that should be inserted/updated as NULL when blank.
func NullStringColumn(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// NullInt32 represents an int32 that may be null. NullInt32 implements the
// sql.Scanner interface, so it can be used as a scan destination, similar to
// sql.NullString. When the scanned value is null, int32 is set to the zero value.
type NullInt32 struct{ N *int32 }

// NewNullInt32 returns a NullInt64 treating zero value as null.
func NewNullInt32(i int32) NullInt32 {
	if i == 0 {
		return NullInt32{}
	}
	return NullInt32{N: &i}
}

// Scan implements the Scanner interface.
func (n *NullInt32) Scan(value any) error {
	switch value := value.(type) {
	case int64:
		*n.N = int32(value)
	case int32:
		*n.N = value
	case nil:
		return nil
	default:
		return errors.Errorf("value is not int64: %T", value)
	}
	return nil
}

// Value implements the driver Valuer interface.
func (n NullInt32) Value() (driver.Value, error) {
	if n.N == nil {
		return nil, nil
	}
	return *n.N, nil
}

// NullInt32Column represents an int32 that should be inserted/updated as NULL when the value is 0.
func NullInt32Column(n int32) *int32 {
	if n == 0 {
		return nil
	}
	return &n
}

// NullInt64 represents an int64 that may be null. NullInt64 implements the
// sql.Scanner interface, so it can be used as a scan destination, similar to
// sql.NullString. When the scanned value is null, int64 is set to the zero value.
type NullInt64 struct{ N *int64 }

// NewNullInt64 returns a NullInt64 treating zero value as null.
func NewNullInt64(i int64) NullInt64 {
	if i == 0 {
		return NullInt64{}
	}
	return NullInt64{N: &i}
}

// Scan implements the Scanner interface.
func (n *NullInt64) Scan(value any) error {
	switch value := value.(type) {
	case int64:
		*n.N = value
	case int32:
		*n.N = int64(value)
	case nil:
		return nil
	default:
		return errors.Errorf("value is not int64: %T", value)
	}
	return nil
}

// Value implements the driver Valuer interface.
func (n NullInt64) Value() (driver.Value, error) {
	if n.N == nil {
		return nil, nil
	}
	return *n.N, nil
}

// NullInt64Column represents an int64 that should be inserted/updated as NULL when the value is 0.
func NullInt64Column(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}

// NullInt represents an int that may be null. NullInt implements the
// sql.Scanner interface, so it can be used as a scan destination, similar to
// sql.NullString. When the scanned value is null, int is set to the zero value.
type NullInt struct{ N *int }

// NewNullInt returns a NullInt treating zero value as null.
func NewNullInt(i int) NullInt {
	if i == 0 {
		return NullInt{}
	}
	return NullInt{N: &i}
}

// Scan implements the Scanner interface.
func (n *NullInt) Scan(value any) error {
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
func (n NullInt) Value() (driver.Value, error) {
	if n.N == nil {
		return nil, nil
	}
	return *n.N, nil
}

// NullBool represents a bool that may be null. NullBool implements the
// sql.Scanner interface, so it can be used as a scan destination, similar to
// sql.NullString. When the scanned value is null, B is set to false.
type NullBool struct{ B *bool }

// Scan implements the Scanner interface.
func (n *NullBool) Scan(value any) error {
	switch v := value.(type) {
	case bool:
		*n.B = v
	case int:
		*n.B = v != 0
	case int32:
		*n.B = v != 0
	case int64:
		*n.B = v != 0
	case nil:
		break
	default:
		return errors.Errorf("value is not bool: %T", value)
	}
	return nil
}

// Value implements the driver Valuer interface.
func (n NullBool) Value() (driver.Value, error) {
	if n.B == nil {
		return nil, nil
	}
	return *n.B, nil
}

// JSONInt64Set represents an int64 set as a JSONB object where the keys are
// the ids and the values are null. It implements the sql.Scanner interface, so
// it can be used as a scan destination, similar to
// sql.NullString.
type JSONInt64Set struct{ Set *[]int64 }

// Scan implements the Scanner interface.
func (n *JSONInt64Set) Scan(value any) error {
	set := make(map[int64]*struct{})

	switch value := value.(type) {
	case nil:
	case []byte:
		if err := json.Unmarshal(value, &set); err != nil {
			return err
		}
	default:
		return errors.Errorf("value is not []byte: %T", value)
	}

	if *n.Set == nil {
		*n.Set = make([]int64, 0, len(set))
	} else {
		*n.Set = (*n.Set)[:0]
	}

	for id := range set {
		*n.Set = append(*n.Set, id)
	}

	return nil
}

// Value implements the driver Valuer interface.
func (n JSONInt64Set) Value() (driver.Value, error) {
	if n.Set == nil {
		return nil, nil
	}
	return *n.Set, nil
}

// NullJSONRawMessage represents a json.RawMessage that may be null. NullJSONRawMessage implements the
// sql.Scanner interface, so it can be used as a scan destination, similar to
// sql.NullString. When the scanned value is null, Raw is left as nil.
type NullJSONRawMessage struct {
	Raw json.RawMessage
}

// Scan implements the Scanner interface.
func (n *NullJSONRawMessage) Scan(value any) error {
	switch value := value.(type) {
	case nil:
	case []byte:
		// We make a copy here because the given value could be reused by
		// the SQL driver
		n.Raw = make([]byte, len(value))
		copy(n.Raw, value)
	default:
		return errors.Errorf("value is not []byte: %T", value)
	}

	return nil
}

// Value implements the driver Valuer interface.
func (n *NullJSONRawMessage) Value() (driver.Value, error) {
	return n.Raw, nil
}

// JSONMessage wraps a value that can be encoded/decoded as JSON so that
// it implements db.Scanner and db.Valuer.
func JSONMessage[T any](val *T) jsonMessage[T] {
	return jsonMessage[T]{val}
}

type jsonMessage[T any] struct {
	inner *T
}

func (m jsonMessage[T]) Scan(value any) error {
	switch value := value.(type) {
	case nil:
	case []byte:
		return json.Unmarshal(value, m.inner)
	case string:
		return json.Unmarshal([]byte(value), m.inner)
	default:
		return errors.Errorf("value is not []byte: %T", value)
	}

	return nil
}

// Value implements the driver Valuer interface.
func (m jsonMessage[T]) Value() (driver.Value, error) {
	b, err := json.Marshal(m.inner)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// CommitBytea represents a hex-encoded string that is efficiently encoded in Postgres and should
// be used in place of a text or varchar type when the table is large (e.g. a record per commit).
type CommitBytea string

// Scan implements the Scanner interface.
func (c *CommitBytea) Scan(value any) error {
	switch value := value.(type) {
	case nil:
	case []byte:
		*c = CommitBytea(hex.EncodeToString(value))
	default:
		return errors.Errorf("value is not []byte: %T", value)
	}

	return nil
}

// Value implements the driver Valuer interface.
func (c CommitBytea) Value() (driver.Value, error) {
	return hex.DecodeString(string(c))
}

// Scanner captures the Scan method of sql.Rows and sql.Row.
type Scanner interface {
	Scan(dst ...any) error
}

// A ScanFunc scans one or more rows from a scanner, returning
// the last id column scanned and the count of scanned rows.
type ScanFunc func(Scanner) (last, count int64, err error)
