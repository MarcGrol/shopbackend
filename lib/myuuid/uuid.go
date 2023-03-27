package myuuid

import "github.com/google/uuid"

type RealUUIDer struct{}

func (u RealUUIDer) Create() string {
	return uuid.New().String()
}
