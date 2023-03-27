package mytime

import "time"

type RealNower struct{}

func (n RealNower) Now() time.Time {
	return time.Now()
}
