package mytime

import "time"

type Nower interface {
	Now() time.Time
}
