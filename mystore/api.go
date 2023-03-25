package mystore

import "context"

var New func(c context.Context, typeCreator func() (interface{}, interface{})) (DataStorer, func(), error)

type DataStorer interface {
	RunInTransaction(c context.Context, f func(context.Context) error) error
	Put(c context.Context, kind, uid string, value interface{}) error
	Get(c context.Context, kind, uid string) (interface{}, bool, error)
	List(c context.Context, kind string) (interface{}, error)
}
