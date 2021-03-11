package testing

import "time"

type Clock interface {
	Now() time.Time
	Add(time.Duration) time.Time
}

type TestClock struct {
	Time time.Time
}

func (c *TestClock) Now() time.Time                { return c.Time }
func (c *TestClock) Add(d time.Duration) time.Time { c.Time = c.Time.Add(d); return c.Time }
