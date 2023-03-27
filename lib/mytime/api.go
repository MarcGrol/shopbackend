package mytime

import "time"

//go:generate mockgen -source=api.go -package mytime -destination nower_mock.go Nower
type Nower interface {
	Now() time.Time
}
