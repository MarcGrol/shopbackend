package mystore

import (
	"context"
	"os"
	"sync"
)

type inMemoryStore struct {
	sync.Mutex
	items       map[string]interface{}
	typeCreator func() (interface{}, interface{})
}

func init() {
	if os.Getenv("GOOGLE_CLOUD_PROJECT") == "" {
		New = newInMemoryStore
	}
}

func newInMemoryStore(c context.Context, typeCreator func() (interface{}, interface{})) (DataStorer, func(), error) {
	return &inMemoryStore{
		items:       map[string]interface{}{},
		typeCreator: typeCreator,
	}, func() {}, nil
}

func (ps *inMemoryStore) RunInTransaction(c context.Context, f func(c context.Context) error) error {
	// Start transaction
	ps.Lock()

	ctx := context.WithValue(c, "transaction", true)

	// Within this block everything is transactional
	err := f(ctx)
	if err != nil {

		// Rollback
		ps.Unlock()

		return err
	}

	// Commit
	ps.Unlock()

	return nil
}

func (ps *inMemoryStore) Put(c context.Context, kind, uid string, objectToStore interface{}) error {
	nonTransactional := c.Value("transaction") == nil

	if nonTransactional {
		ps.Lock()
	}

	ps.items[uid] = objectToStore

	if nonTransactional {
		ps.Unlock()
	}

	return nil
}

func (ps *inMemoryStore) Get(c context.Context, kind, uid string) (interface{}, bool, error) {
	nonTransactional := c.Value("transaction") == nil

	if nonTransactional {
		ps.Lock()
	}

	o, found := ps.items[uid]

	if nonTransactional {
		ps.Unlock()
	}

	objectToFetch, _ := ps.typeCreator()
	objectToFetch = o
	return objectToFetch, found, nil
}

func (ps *inMemoryStore) List(c context.Context, kind string) (interface{}, error) {
	nonTransactional := c.Value("transaction") == nil

	if nonTransactional {
		ps.Lock()
	}

	objectsToFetch := mapValuesToSlice(ps.items)

	if nonTransactional {
		ps.Unlock()
	}
	return objectsToFetch, nil
}

func mapValuesToSlice[K comparable, V any](m map[K]V) []V {
	values := make([]V, 0, len(m))
	for _, v := range m {
		values = append(values, v)
	}
	return values
}
