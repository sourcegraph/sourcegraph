package utctime

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// Time is a wrapper around time.Time that implements the database/sql.Scanner
// and database/sql/driver.Valuer interfaces to serialize and deserialize time
// in UTC time zone.
//
// Time ensures that time.Time values are always:
//
//   - represented in UTC for consistency
//   - rounded to microsecond precision
//
// We round the time because PostgreSQL times are represented in microseconds:
// https://www.postgresql.org/docs/current/datatype-datetime.html
type Time time.Time

// Now returns the current time in UTC.
func Now() Time { return Time(time.Now()) }

// FromTime returns a utctime.Time from a time.Time.
func FromTime(t time.Time) Time { return Time(t.UTC().Round(time.Microsecond)) }

// Date is analagous to time.Date, but only represents UTC time.
func Date(year int, month time.Month, day, hour, min, sec, nsec int) Time {
	return FromTime(time.Date(year, month, day, hour, min, sec, nsec, time.UTC))
}

var _ sql.Scanner = (*Time)(nil)

func (t *Time) Scan(src any) error {
	if src == nil {
		return nil
	}
	if v, ok := src.(time.Time); ok {
		*t = FromTime(v)
		return nil
	}
	return errors.Newf("value %T is not time.Time", src)
}

var _ driver.Valuer = (*Time)(nil)

// Value must be called with a non-nil Time. driver.Valuer callers will first
// check that the value is non-nil, so this is safe.
func (t Time) Value() (driver.Value, error) {
	stdTime := t.GetTime()
	return *stdTime, nil
}

var _ json.Marshaler = (*Time)(nil)

func (t Time) MarshalJSON() ([]byte, error) { return json.Marshal(t.GetTime()) }

var _ json.Unmarshaler = (*Time)(nil)

func (t *Time) UnmarshalJSON(data []byte) error {
	var stdTime time.Time
	if err := json.Unmarshal(data, &stdTime); err != nil {
		return err
	}
	*t = FromTime(stdTime)
	return nil
}

// GetTime returns the underlying time.GetTime value, or nil if it is nil.
func (t *Time) GetTime() *time.Time {
	if t == nil {
		return nil
	}
	return pointers.Ptr(t.AsTime())
}

// Time casts the Time as a standard time.Time value.
func (t Time) AsTime() time.Time {
	return time.Time(t).UTC().Round(time.Microsecond)
}
