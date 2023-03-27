package mytime

import "time"

var (
	ExampleTime time.Time
)

func init() {
	ExampleTime, _ = time.Parse("2006-01-02T15:04:05Z", "2023-02-27T23:58:59Z")
}

//go:generate mockgen -source=api.go -package mytime -destination nower_mock.go Nower
type Nower interface {
	Now() time.Time
}
