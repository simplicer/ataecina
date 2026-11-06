// Package clock tells the time to the use cases.
package clock

import "time"

// System is the machine clock, in UTC.
type System struct{}

// Now is the current time in UTC.
func (System) Now() time.Time { return time.Now().UTC() }
