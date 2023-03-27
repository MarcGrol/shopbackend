package myuuid

//go:generate mockgen -source=api.go -package myuuid -destination myuuid_mock.go UUIDer
type UUIDer interface {
	Create() string
}
