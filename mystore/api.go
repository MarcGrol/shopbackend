package mystore

import "context"

type DataStorer interface {
	Put(c context.Context, kind, uid string, value interface{}) error
	Get(c context.Context, kind, uid string, result interface{}) (bool, error)
	List(c context.Context, kind string, resultList interface{}) error
}
